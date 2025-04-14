package handlers

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ellypaws/inkbunny/api"
	"gopkg.in/telebot.v4"

	"classifier/pkg/classify"
	"classifier/pkg/telegram/parser"
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

	ctx := context.Background()

	readSubs := make(map[string]classify.Prediction)
	var mu sync.RWMutex
	worker := utils.NewWorkerPool(50, func(submission api.SubmissionSearchList, yield func(Result)) {
		if !utils.IsImage(submission.FileURLFull) {
			return
		}
		mu.RLock()
		if _, ok := readSubs[submission.SubmissionID]; ok {
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
		file, err := utils.DownloadEncrypt(ctx, b.crypto, submission.FileURLFull, fileName)
		if err != nil {
			b.logger.Errorf("Error downloading submission %s: %v", submission.SubmissionID, err)
			return
		}
		b.logger.Debugf("Downloaded submission: %v", submission.FileURLFull)

		prediction, err := classify.DefaultCache.Predict(ctx, submission.FileURLFull, file)
		file.Close()
		if err != nil {
			b.logger.Errorf("Error predicting submission: %v", err)
			return
		}
		b.logger.Debugf("Classified submission https://inkbunny.net/%s: %+v", submission.SubmissionID, prediction)

		go func() { mu.Lock(); readSubs[submission.SubmissionID] = prediction; mu.Unlock() }()

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
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
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
			case <-ctx.Done():
				return
			case <-time.After(b.refreshRate):
				continue
			}
		}
	}()

	classes := []string{"cub"}
	if c := os.Getenv("CLASS"); c != "" {
		classes = strings.Split(c, ",")
	}
	for res := range worker.Work() {
		var highestClass *string
		for _, class := range classes {
			if prediction := res.Prediction[class]; prediction >= 0.5 {
				if highestClass == nil {
					highestClass = &class
				} else if prediction > res.Prediction[*highestClass] {
					highestClass = &class
				}
			}
		}
		if highestClass != nil {
			text := fmt.Sprintf("⚠️ Detected class %q (%.2f%%) for https://inkbunny.net/s/%s by %q", *highestClass, res.Prediction[*highestClass]*100, res.Submission.SubmissionID, res.Submission.Username)
			b.logger.Info(text)

			b.mu.Lock()
			messages, err := b.Notify(text, res)
			b.references[res.Submission.SubmissionID] = &MessageRef{Messages: messages}
			b.mu.Unlock()

			if err != nil {
				b.logger.Errorf("Error sending message to telegram: %v", err)
			}
		}
	}

	return nil
}

func addButton(button *telebot.ReplyMarkup, messages ...*telebot.Message) []MessageWithButton {
	withButtons := make([]MessageWithButton, len(messages))
	for i, m := range messages {
		withButtons[i] = MessageWithButton{Message: m, Button: button}
	}
	return withButtons
}

func (b *Bot) Notify(content string, result Result) ([]MessageWithButton, error) {
	if len(b.Subscribers) == 0 {
		b.logger.Warn("Cannot send message - no subscribers")
		return nil, nil
	}

	message := parser.Parse(content)
	button := Single(CopyButton(falseButton, result.Submission.SubmissionID))

	references := make([]MessageWithButton, 0, len(b.Subscribers))
	for id, recipient := range b.Subscribers {
		if recipient == nil {
			b.logger.Warnf("%d has no recipient", id)
			continue
		}
		b.logger.Debug("Sending message to Telegram", "user_id", id)
		message, err := b.Bot.Send(recipient, message, &telebot.SendOptions{ParseMode: telebot.ModeMarkdownV2}, button)
		if err != nil {
			b.logger.Error("Failed to send message", "error", err, "user_id", id)
			return nil, fmt.Errorf("error sending to telegram: %w", err)
		}

		b.logger.Info("Message sent successfully", "user_id", id)
		references = append(references, MessageWithButton{Message: message, Button: button})
	}

	return references, nil
}

func CopyButton(button telebot.Btn, data string) telebot.Btn {
	button.Data = data
	return button
}

func Single[button interface{ Inline() *telebot.InlineButton }](buttons ...button) *telebot.ReplyMarkup {
	return NewButtons(buttons)
}

func NewButtons[button interface{ Inline() *telebot.InlineButton }](rows ...[]button) *telebot.ReplyMarkup {
	buttonRows := make([][]telebot.InlineButton, len(rows))
	for i, row := range rows {
		buttonRows[i] = NewRow(row...)
	}

	return &telebot.ReplyMarkup{InlineKeyboard: buttonRows}
}

func NewRow[button interface{ Inline() *telebot.InlineButton }](buttons ...button) []telebot.InlineButton {
	column := make([]telebot.InlineButton, len(buttons))
	for i, b := range buttons {
		column[i] = *b.Inline()
	}

	return column
}

func random[T any](v ...T) T {
	if len(v) == 0 {
		var def T
		return def
	}
	return v[rand.IntN(len(v))]
}

func randomActivity() telebot.ChatAction {
	return random(
		telebot.Typing,
		telebot.UploadingPhoto,
		telebot.UploadingVideo,
		telebot.UploadingAudio,
		telebot.UploadingDocument,
		telebot.UploadingVNote,
		telebot.RecordingVideo,
		telebot.RecordingAudio,
		telebot.RecordingVNote,
		telebot.FindingLocation,
		telebot.ChoosingSticker,
	)
}

// set isFalseReport to add refs.count, and will edit an undoButton on who clicked the button
func (b *Bot) handleReport(isFalseReport bool) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		if err := c.Notify(randomActivity()); err != nil {
			return err
		}

		submissionID := c.Data()
		if submissionID == "" {
			b.logger.Warn("No submission ID found")
			return errors.New("no submission ID found")
		}

		b.mu.Lock()
		defer b.mu.Unlock()
		refs, ok := b.references[submissionID]
		if !ok {
			b.logger.Warn("No references found")
			return errors.New("no references found")
		}

		var button *telebot.ReplyMarkup
		if isFalseReport {
			button = Single(CopyButton(undoButton, submissionID))
			refs.Count++
		} else {
			button = Single(CopyButton(falseButton, submissionID))
			refs.Count--
		}

		reporterMessage := c.Message()
		if reporterMessage == nil {
			b.logger.Error("Message cannot be nil")
			return errors.New("message cannot be nil")
		}

		text := strings.SplitN(reporterMessage.Text, "\n", 2)[0]
		if refs.Count > 0 {
			text = fmt.Sprintf("%s\n\n%d reported this as a false positive", text, refs.Count)
		}
		for i, ref := range refs.Messages {
			if ref.Message.Chat.ID == reporterMessage.Chat.ID {
				edited, err := b.Bot.Edit(ref.Message, text, button)
				if err != nil {
					b.logger.Warn("Failed to edit message", "error", err)
					continue
				}
				refs.Messages[i] = MessageWithButton{Message: edited, Button: button}
			} else {
				edited, err := b.Bot.Edit(ref.Message, text, ref.Button)
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
