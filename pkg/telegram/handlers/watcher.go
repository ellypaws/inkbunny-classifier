package handlers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/telebot.v4"

	"github.com/ellypaws/inkbunny/api"

	"classifier/pkg/classify"
	"classifier/pkg/telegram/parser"
	"classifier/pkg/telegram/wrapper"
	"classifier/pkg/utils"
)

type Result struct {
	Path       string                   `json:"path"`
	Submission api.SubmissionSearchList `json:"submission,omitempty"`
	Prediction classify.Prediction      `json:"prediction,omitempty"`
}

func (b *Bot) Watcher() error {
	if !b.classify {
		return errors.New("classification not enabled")
	}

	var mu sync.RWMutex
	worker := utils.NewWorkerPool(50, func(submission api.SubmissionSearchList, yield func(Result)) {
		if !utils.IsImage(submission.FileURLFull) {
			return
		}
		mu.RLock()
		if _, ok := b.references[submission.SubmissionID]; ok {
			mu.RUnlock()
			return
		}
		mu.RUnlock()

		b.logger.Infof("New submission found https://inkbunny.net/s/%s", submission.SubmissionID)

		folder := filepath.Join("inkbunny", submission.Username)
		err := os.MkdirAll(filepath.Join("inkbunny", submission.Username), 0755)
		if err != nil {
			b.logger.Errorf("Error creating folder %s: %v", submission.SubmissionID, err)
		}

		fileName := filepath.Join(folder, filepath.Base(submission.FileURLFull))

		if !utils.FileExists(fileName) {
			_, err = utils.DownloadEncrypt(b.context, b.crypto, submission.FileURLFull, fileName)
			if err != nil {
				b.logger.Errorf("Error downloading submission %s: %v", submission.SubmissionID, err)
				return
			}
			b.logger.Debugf("Downloaded submission: %v", submission.FileURLFull)
		}

		file, err := os.Open(fileName)
		if err != nil {
			b.logger.Errorf("Error opening file %s: %v", submission.FileURLFull, err)
		}
		prediction, err := classify.DefaultCache.Predict(b.context, submission.FileURLFull, b.crypto.Key(), file)
		file.Close()
		if err != nil {
			b.logger.Errorf("Error predicting submission: %v", err)
			return
		}
		b.logger.Debugf("Classified submission https://inkbunny.net/%s: %+v", submission.SubmissionID, prediction)

		if b.key != "" {
			submission.FileURLFull = fmt.Sprintf("%s?key=%s", submission.FileURLFull, b.key)
		}
		yield(Result{
			Path:       submission.FileURLFull,
			Submission: submission,
			Prediction: prediction,
		})
	})

	go func() {
		defer worker.Close()
		for b.context.Err() == nil {
			select {
			case <-b.context.Done():
				return
			default:
			}
			user := api.Credentials{Sid: b.sid}
			request := api.SubmissionSearchRequest{
				SID:    b.sid,
				GetRID: true,
			}
			response, err := user.SearchSubmissions(request)
			if err != nil {
				b.logger.Errorf("Error searching submissions: %v", err)
			}
			worker.Add(response.Submissions...)
			select {
			case <-b.context.Done():
				return
			case <-time.After(b.refreshRate):
				continue
			}
		}
	}()

	allowed := []string{"cub"}
	if c := os.Getenv("CLASS"); c != "" {
		allowed = strings.Split(c, ",")
	}
	for res := range worker.Work() {
		if len(res.Prediction.Minimum(0.75).Whitelist(allowed...)) == 0 {
			b.mu.Lock()
			b.references[res.Submission.SubmissionID] = &MessageRef{Result: res}
			b.mu.Unlock()
			continue
		}

		b.mu.Lock()
		messages, err := b.Notify(res)
		b.references[res.Submission.SubmissionID] = &MessageRef{Messages: messages, Result: res}
		b.mu.Unlock()

		b.save()
		if err != nil {
			b.logger.Errorf("Error notifying users: %v", err)
		}
	}

	return nil
}

func (b *Bot) Notify(result Result) ([]MessageWithButton, error) {
	class, confidence := result.Prediction.Max()
	b.logger.Infof("⚠️ Detected %q (%.2f%%) for https://inkbunny.net/s/%s by %q", class, confidence*100, result.Submission.SubmissionID, result.Submission.Username)

	if len(b.Subscribers) == 0 {
		b.logger.Warn("Cannot send message - no subscribers")
		return nil, nil
	}

	message := parser.Parsef("⚠️ Detected filtered (%.2f%%) for https://inkbunny.net/s/%s by %q", confidence*100, result.Submission.SubmissionID, result.Submission.Username)
	button := utils.Single(utils.CopyButton(falseButton, result.Submission.SubmissionID), utils.CopyButton(dangerButton, result.Submission.SubmissionID))
	references := make([]MessageWithButton, 0, len(b.Subscribers))
	for id, recipient := range b.Subscribers {
		if b.context.Err() != nil {
			b.logger.Warn("Bot is shutting down, stopping message sending")
			break
		}
		if recipient == nil {
			b.logger.Warnf("%d has no recipient", id)
			continue
		}
		b.logger.Debug("Sending message to Telegram", "user_id", id)
		reference, err := wrapper.Send(b.Bot, recipient, message, &telebot.SendOptions{ParseMode: telebot.ModeMarkdownV2, ReplyMarkup: button})
		if err != nil {
			b.logger.Error("Failed to send message", "error", err, "user_id", id)
			return nil, fmt.Errorf("error sending to telegram: %w", err)
		}

		b.logger.Info("Message sent successfully", "user_id", id)
		references = append(references, MessageWithButton{Message: reference, Button: button})
	}

	return references, nil
}

type state int

const (
	falsePositive state = iota
	undoFalsePositive
	danger
	undoDanger
)

const (
	falsePositiveState = "false_positive"
	dangerState        = "danger"
)

func (s state) String() string {
	switch s {
	case falsePositive:
		return falsePositiveState
	case danger:
		return dangerState
	default:
		return ""
	}
}

func previousState(states []string) state {
	for _, s := range states {
		switch s {
		case falsePositiveState:
			return falsePositive
		case dangerState:
			return danger
		}
	}
	return state(-1)
}

func buildText(base string, refs *MessageRef) string {
	var b strings.Builder
	falseReports := utils.CountEqual(refs.Reports, false)
	dangerReports := len(refs.Reports) - falseReports
	if falseReports > 0 {
		b.WriteString(fmt.Sprintf("✅ %d reported this as a false positive", falseReports))
	}
	if dangerReports > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("⚠️ %d reported this as dangerous", dangerReports))
	}
	if b.Len() > 0 {
		return fmt.Sprintf("%s\n\n%s", base, b.String())
	}
	return base
}

// set isFalseReport to add refs.count, and will edit an undoButton on who clicked the button
func (b *Bot) handleReport(action state) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		defer b.save()
		states := strings.SplitN(c.Data(), ",", 2)
		submissionID := states[0]
		if submissionID == "" {
			b.logger.Warn("No submission ID found")
			return nil
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		refs, ok := b.references[submissionID]
		if !ok {
			b.logger.Warn("No references found")
			return nil
		}

		user := c.Sender()
		if user == nil {
			b.logger.Warn("No user found")
			return nil
		}

		if err := c.Notify(utils.RandomActivity()); err != nil {
			b.logger.Error("Failed to notify users", "error", err, "users", len(b.Subscribers))
			return nil
		}

		switch action {
		case falsePositive, danger:
			if refs.Reports == nil {
				refs.Reports = make(map[int64]bool)
			}
			refs.Reports[user.ID] = action == danger
			submissionID = strings.Join([]string{submissionID, action.String()}, ",")
		case undoFalsePositive, undoDanger:
			delete(refs.Reports, user.ID)
		}

		if len(refs.Reports) == 0 {
			refs.Reports = nil
		}

		var button *telebot.ReplyMarkup
		undoBtn := utils.CopyButton(undoButton, submissionID)
		falseBtn := utils.CopyButton(falseButton, submissionID)
		dangerBtn := utils.CopyButton(dangerButton, submissionID)
		undoDangerBtn := utils.CopyButton(undoDangerButton, submissionID)
		switch action {
		case falsePositive:
			button = utils.Single(undoBtn, dangerBtn)
		case undoFalsePositive:
			button = utils.Single(falseBtn, dangerBtn)
		case danger:
			button = utils.Single(falseBtn, undoDangerBtn)
		case undoDanger:
			button = utils.Single(falseBtn, dangerBtn)
		}

		reporterMessage := c.Message()
		if reporterMessage == nil {
			b.logger.Error("Message cannot be nil")
			return nil
		}
		baseText := strings.SplitN(reporterMessage.Text, "\n", 2)[0]
		text := buildText(baseText, refs)
		for i, ref := range refs.Messages {
			if ref.Message.Chat.ID == reporterMessage.Chat.ID {
				edited, err := wrapper.Edit(b.Bot, ref.Message, text, button)
				if err != nil {
					b.logger.Warn("Failed to edit message", "error", err)
					continue
				}
				refs.Messages[i] = MessageWithButton{Message: edited, Button: button}
			} else {
				edited, err := wrapper.Edit(b.Bot, ref.Message, text, ref.Button)
				if err != nil {
					b.logger.Warn("Failed to edit message", "error", err)
					continue
				}
				refs.Messages[i].Message = edited
			}
		}

		return nil
	}
}
