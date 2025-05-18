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
	Submission  *api.Submission `json:"submission,omitempty"`
	Predictions Predictions     `json:"predictions,omitempty"`
}

// Aggregate returns the prediction with the highest summed confidence after using [classify.Prediction.Filter], and their average.
// The returned Prediction is not guaranteed to meet the minimum confidence criteria.
func (p Predictions) Aggregate(allowed ...string) (highest *Prediction, confidence float64, average float64) {
	if len(p) == 0 {
		return
	}
	var sums float64
	for i, prediction := range p {
		sum := prediction.Prediction.Clone().Whitelist(allowed...).Sum()
		if i == 0 || sum > confidence {
			highest, confidence = prediction, sum
		}
		sums += sum
	}
	return highest, confidence, sums / float64(len(p))
}

// Max returns the class with the highest confidence in all Predictions and its index.
func (p Predictions) Max() (int, string, float64) {
	var (
		index      int
		class      string
		confidence float64
	)
	for i, pred := range p {
		if pred == nil || pred.Prediction == nil {
			continue
		}
		if _class, _confidence := pred.Prediction.Max(); _confidence > confidence {
			index, class, confidence = i, _class, _confidence
		}
	}
	return index, class, confidence
}

type Predictions []*Prediction

type Prediction struct {
	Path       string              `json:"path"`
	Prediction classify.Prediction `json:"prediction,omitempty"`
}

func (b *Bot) Watcher() error {
	if !b.classify {
		return errors.New("classification not enabled")
	}

	predictionWorker := utils.NewWorkerPool(5, b.predict)
	predictionWorker.Work()
	defer predictionWorker.Close()

	var batch sync.WaitGroup
	// Submission worker
	worker := utils.NewWorkerPool(30, func(submission api.Submission) *Result {
		defer batch.Done()
		b.mu.RLock()
		if _, ok := b.references[submission.SubmissionID]; ok {
			b.mu.RUnlock()
			return nil
		}
		b.mu.RUnlock()

		b.logger.Infof("New submission found https://inkbunny.net/s/%s with %d file%s", submission.SubmissionID, len(submission.Files), utils.Plural(len(submission.Files)))

		var (
			predictions = make([]*Prediction, 0, len(submission.Files))
			err         error
		)
		for i, file := range submission.Files {
			if err = b.context.Err(); err != nil {
				break
			}
			if !utils.IsImage(file.FileURLFull) {
				err = fmt.Errorf("file %s is not an image", file.FileURLFull)
				continue
			}
			prediction := <-predictionWorker.Promise(predictionRequest{
				Username:     submission.Username,
				FileURLFull:  file.FileURLFull,
				SubmissionID: file.SubmissionID,
			})
			if prediction == nil {
				continue
			}
			class, confidence := prediction.Prediction.Max()
			b.logger.Debug("Prediction found",
				"submission", submission.SubmissionID,
				b.classes, floatString(prediction.Prediction.Clone().Whitelist(b.classes...).Sum()),
				"file", fmt.Sprintf("%d/%d", i+1, len(submission.Files)),
				"class", class,
				"confidence", floatString(confidence),
			)
			predictions = append(predictions, prediction)
		}

		b.mu.Lock()
		b.references[submission.SubmissionID] = &MessageRef{Result: &Result{Submission: &submission}}
		b.mu.Unlock()
		if len(predictions) == 0 {
			b.logger.Warn("No prediction found", "submission", submission.SubmissionID, "error", err)
			return nil
		}
		return &Result{
			Submission:  &submission,
			Predictions: predictions,
		}
	})

	// Submission watcher, adds new submissions to worker
	go func() {
		defer worker.Close()
		ticker := time.NewTicker(time.Hour)
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
			var submissionIDs []string
			for _, submission := range response.Submissions {
				submissionIDs = append(submissionIDs, submission.SubmissionID)
			}
			details, err := api.Credentials{Sid: b.sid}.SubmissionDetails(api.SubmissionDetailsRequest{SID: b.sid, SubmissionIDs: strings.Join(submissionIDs, ",")})
			if err != nil {
				b.logger.Errorf("Error getting submission details: %v", err)
				continue
			}
			batch.Add(len(details.Submissions))
			worker.Add(details.Submissions...)
			batch.Wait()
			select {
			case <-b.context.Done():
				return
			case <-time.After(b.refreshRate):
				continue
			case <-ticker.C:
				b.prune()
			}
		}
	}()

	b.logger.Info("Starting watcher", "subscribers", len(b.Subscribers), "references", len(b.references))
	// Results collection
	for res := range worker.Work() {
		if res == nil {
			continue
		}
		if len(res.Predictions) == 0 {
			b.logger.Warn("Submission returned no predictions", "submission_id", res.Submission.SubmissionID)
			continue
		}

		prediction, aggregate, average := res.Predictions.Aggregate(b.classes...)
		if aggregate >= b.threshold {
			if prediction == nil {
				b.logger.Warn("Prediction returned nil", "submission_id", res.Submission.SubmissionID)
				continue
			}
			b.mu.Lock()
			messages, err := b.Notify(res.Submission, prediction)
			b.references[res.Submission.SubmissionID] = &MessageRef{Messages: messages, Result: res}
			b.mu.Unlock()

			b.save()
			if err != nil {
				b.logger.Errorf("Error notifying users: %v", err)
			}
			continue
		}

		index, class, confidence := res.Predictions.Max()
		b.logger.Debug("Submission not notifiable",
			"submission_id", res.Submission.SubmissionID,
			b.classes, floatString(average),
			"file", fmt.Sprintf("%d/%d", index+1, len(res.Predictions)),
			"class", class,
			"confidence", floatString(confidence),
		)

		b.mu.Lock()
		b.references[res.Submission.SubmissionID] = &MessageRef{Result: res}
		b.mu.Unlock()
		continue
	}

	return nil
}

type predictionRequest struct {
	Username     string
	FileURLFull  string
	SubmissionID string
}

// predict downloads the file and predicts the class
func (b *Bot) predict(req predictionRequest) *Prediction {
	folder := filepath.Join("inkbunny", req.Username)
	err := os.MkdirAll(folder, 0755)
	if err != nil {
		b.logger.Errorf("Error creating folder %s for https://inkbunny.net/s/%s: %v", folder, req.SubmissionID, err)
		return nil
	}

	fileName := filepath.Join(folder, filepath.Base(req.FileURLFull))
	if !utils.FileExists(fileName) {
		file, err := utils.DownloadEncrypt(b.context, b.crypto, req.FileURLFull, fileName)
		if err != nil {
			b.logger.Errorf("Error downloading file %s: %v", req.FileURLFull, err)
			return nil
		}
		file.Close()
		b.logger.Debugf("Downloaded submission: %v", req.FileURLFull)
	}

	file, err := os.Open(fileName)
	if err != nil {
		b.logger.Errorf("Error opening file %s: %v", req.FileURLFull, err)
		return nil
	}
	prediction, err := classify.DefaultCache.Predict(b.context, req.FileURLFull, b.crypto.Key(), file)
	file.Close()
	if err != nil {
		b.logger.Errorf("Error predicting submission: %v", err)
		return nil
	}
	if b.crypto.Key() != "" {
		req.FileURLFull = fmt.Sprintf("%s?key=%s", req.FileURLFull, b.crypto.Key())
	}
	return &Prediction{
		Path:       req.FileURLFull,
		Prediction: prediction,
	}
}

func defaultSendOption(button *telebot.ReplyMarkup) *telebot.SendOptions {
	return &telebot.SendOptions{
		DisableWebPagePreview: true,
		ParseMode:             telebot.ModeMarkdownV2,
		Protected:             true,
		ReplyMarkup:           button,
	}
}

func floatString(f float64) string {
	return fmt.Sprintf("%.2f%%", f*100)
}

var filteredMessage = parser.Patternf("⚠️ Detected filtered (%.2f%%) for ||https://inkbunny.net/s/%s|| by %q", 1.0, "<UNKNOWN>", "Username")

func (b *Bot) Notify(submission *api.Submission, prediction *Prediction) ([]MessageWithButton, error) {
	class, confidence := prediction.Prediction.Max()
	b.logger.Infof("⚠️ Detected %q (%.2f%%) for https://inkbunny.net/s/%s by %q", class, confidence*100, submission.SubmissionID, submission.Username)

	if len(b.Subscribers) == 0 {
		b.logger.Warn("Cannot send message - no subscribers")
		return nil, nil
	}
	message := filteredMessage(prediction.Prediction.Clone().Whitelist(b.classes...).Sum()*100, submission.SubmissionID, submission.Username)
	button := utils.Single(utils.CopyButton(falseButton, submission.SubmissionID), utils.CopyButton(dangerButton, submission.SubmissionID))
	b.mu.RLock()
	defer b.mu.RUnlock()
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
		if err := b.Bot.Notify(recipient, utils.RandomActivity()); err != nil {
			b.logger.Error("Failed to notify user", "error", err, "user", recipient.ID, "username", recipient.Username)
			if errors.Is(err, telebot.ErrBlockedByUser) {
				continue
			} else {
				return nil, err
			}
		}
		reference, err := wrapper.Send(b.Bot, recipient, message, defaultSendOption(button))
		if err != nil {
			b.logger.Error("Failed to send message", "error", err, "user_id", id)
			if errors.Is(err, telebot.ErrBlockedByUser) {
				continue
			} else {
				return nil, err
			}
		}

		b.logger.Info("Notified successfully", "user_id", id, "username", recipient.Username)
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
	falsePositiveState     = "false_positive"
	undoFalsePositiveState = "undo"
	dangerState            = "danger"
	undoDangerState        = "undo_danger"
)

func (s state) String() string {
	switch s {
	case falsePositive:
		return falsePositiveState
	case undoFalsePositive:
		return undoFalsePositiveState
	case danger:
		return dangerState
	case undoDanger:
		return undoDangerState
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

func (b *Bot) buildText(refs *MessageRef) string {
	var builder strings.Builder
	falseReports := utils.CountEqual(refs.Reports, false)
	dangerReports := len(refs.Reports) - falseReports
	if falseReports > 0 {
		builder.WriteString(fmt.Sprintf("✅ %d reported this as a false positive", falseReports))
	}
	if dangerReports > 0 {
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(fmt.Sprintf("⚠️ %d reported this as dangerous", dangerReports))
	}

	_, confidence, _ := refs.Result.Predictions.Aggregate(b.classes...)
	base := filteredMessage(confidence*100, refs.Result.Submission.SubmissionID, refs.Result.Submission.Username)
	if builder.Len() > 0 {
		return fmt.Sprintf("%s\n\n%s", base, parser.Parse(builder.String()))
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
			b.logger.Error("Failed to notify user", "error", err, "user", user.ID, "username", user.Username)
			return nil
		}

		b.logger.Info("Reported this", "submission", submissionID, "action", action, "user", user.ID, "username", user.Username)
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

		reporterChat := c.Chat()
		if reporterChat == nil {
			b.logger.Error("Reporter chat cannot be nil")
			return nil
		}
		text := b.buildText(refs)
		for i, ref := range refs.Messages {
			if ref.Message.Chat.ID == reporterChat.ID {
				edited, err := wrapper.Edit(b.Bot, ref.Message, text, defaultSendOption(button))
				if err != nil {
					b.logger.Warn("Failed to edit message", "error", err)
					continue
				}
				refs.Messages[i] = MessageWithButton{Message: edited, Button: button}
			} else {
				edited, err := wrapper.Edit(b.Bot, ref.Message, text, defaultSendOption(ref.Button))
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
