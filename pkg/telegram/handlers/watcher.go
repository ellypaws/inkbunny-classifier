package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ellypaws/inkbunny/api"

	"classifier/pkg/classify"
	"classifier/pkg/lib"
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
	worker := utils.NewWorkerPool(50, func(jobs <-chan api.SubmissionSearchList, yield func(Result)) {
		for submission := range jobs {
			if !utils.IsImage(submission.FileURLFull) {
				continue
			}
			mu.RLock()
			if _, ok := readSubs[submission.SubmissionID]; ok {
				mu.RUnlock()
				continue
			}
			mu.RUnlock()

			b.logger.Infof("New submission found https://inkbunny.net/s/%s", submission.SubmissionID)

			file, err := lib.DownloadFile(ctx, submission.FileURLFull, filepath.Join("inkbunny", submission.Username), b.crypto)
			if err != nil {
				b.logger.Errorf("Error downloading submission %s: %v", submission.SubmissionID, err)
				continue
			}
			b.logger.Debugf("Downloaded submission: %v", submission.FileURLFull)

			prediction, err := classify.DefaultCache.Predict(ctx, submission.FileURLFull, file)
			file.Close()
			if err != nil {
				b.logger.Printf("Error predicting submission: %v", err)
				continue
			}
			b.logger.Infof("Classified submission https://inkbunny.net/%s: %+v", submission.SubmissionID, prediction)

			mu.Lock()
			readSubs[submission.SubmissionID] = prediction
			mu.Unlock()

			if b.key != "" {
				submission.FileURLFull = fmt.Sprintf("%s?decrypt_key=%s", submission.FileURLFull, b.key)
			}
			yield(Result{
				Path:       submission.FileURLFull,
				Submission: submission,
				Prediction: prediction,
			})
		}
	})

	go func() {
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
		worker.Close()
	}()

	classes := []string{"cub"}
	if c := os.Getenv("CLASS"); c != "" {
		classes = strings.Split(c, ",")
	}
	for res := range worker.Work() {
		var highest *string
		for _, class := range classes {
			if prediction := res.Response.Prediction[class]; prediction > 0.5 {
				if highest == nil {
					highest = &class
				} else if prediction > res.Response.Prediction[*highest] {
					highest = &class
				}
			}
		}
		if highest != nil {
			_, err := b.Notify(fmt.Sprintf("⚠️ Detected class %q (%d%%) for https://inkbunny.net/s/%s by %q", *highest, int(res.Response.Prediction[*highest]*100), res.Response.Submission.SubmissionID, res.Response.Submission.Username))
			if err != nil {
				b.logger.Errorf("Error sending message to telegram: %v", err)
			}
		}
	}

	return nil
}
