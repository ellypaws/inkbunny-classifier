package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/log"

	"classifier/pkg/telegram/handlers"
	"github.com/ellypaws/inkbunny/api"

	"classifier/pkg/classify"
	"classifier/pkg/lib"
	"classifier/pkg/utils"
)

func main() {
	crypto, err := lib.NewCrypto("")
	if err != nil {
		panic(err)
	}
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()
	b := base{
		logger:  log.New(os.Stdout),
		crypto:  crypto,
		context: ctx,
		sid:     os.Getenv("SID"),
		classes: strings.Split(os.Getenv("CLASSES"), ","),
	}
	b.logger.SetLevel(log.DebugLevel)
	predictionWorker := utils.NewWorkerPool(5, b.predict)
	predictionWorker.Work()
	defer predictionWorker.Close()

	worker := utils.NewWorkerPool(30, func(submission api.Submission) *handlers.Result {
		b.logger.Infof("New submission found https://inkbunny.net/s/%s with %d file%s", submission.SubmissionID, len(submission.Files), utils.Plural(len(submission.Files)))

		var (
			predictions = make([]*handlers.Prediction, 0, len(submission.Files))
			err         error
		)
		for _, file := range submission.Files {
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
			if prediction != nil {
				predictions = append(predictions, prediction)
			}
		}
		if len(predictions) == 0 {
			b.logger.Warn("No prediction found", "submission", submission.SubmissionID, "error", err)
			return nil
		}
		return &handlers.Result{
			Submission:  &submission,
			Predictions: predictions,
		}
	})

	// Submission watcher, thumbs through pages and adds submissions to the worker.
	go func() {
		defer worker.Close()
		user := api.Credentials{Sid: b.sid}
		request := api.SubmissionSearchRequest{
			SID:    b.sid,
			GetRID: true,
		}
		response, err := user.SearchSubmissions(request)
		if err != nil {
			b.logger.Errorf("Error searching submissions: %v", err)
		}
		for submissions, err := range response.AllSubmissions() {
			select {
			case <-b.context.Done():
				return
			default:
			}
			if err != nil {
				b.logger.Errorf("Error searching submissions: %v", err)
				continue
			}
			if len(submissions) == 0 {
				b.logger.Warn("No submissions were found at this page")
				continue
			}
			if len(submissions) > 100 {
				b.logger.Warn("More than 100 submissions were found at this page, might fail")
			}
			submissionIDs := make([]string, len(submissions))
			for i := range len(submissions) {
				submissionIDs[i] = submissions[i].SubmissionID
			}
			details, err := api.Credentials{Sid: b.sid}.SubmissionDetails(api.SubmissionDetailsRequest{SID: b.sid, SubmissionIDs: strings.Join(submissionIDs, ",")})
			if err != nil {
				b.logger.Errorf("Error getting submission details: %v", err)
				continue
			}
			for i := range details.Submissions {
				select {
				case <-b.context.Done():
					return
				default:
					worker.Add(details.Submissions[i])
				}
			}
		}
	}()

	b.logger.Info("Starting scraping")
	// Results collection
NextResult:
	for res := range worker.Work() {
		if res == nil {
			continue
		}
		if len(res.Predictions) == 0 {
			b.logger.Warn("Submission returned no predictions", "submission_id", res.Submission.SubmissionID)
			continue
		}
		for _, prediction := range res.Predictions {
			if prediction == nil {
				b.logger.Warn("Prediction is nil", "submission_id", res.Submission.SubmissionID)
				continue
			}
			if prediction.Prediction.Clone().Whitelist(b.classes...).Sum() >= 0.75 {
				err := b.Save(prediction)
				if err != nil {
					b.logger.Warn("Error saving prediction", "submission_id", res.Submission.SubmissionID, "error", err)
				} else {
					b.logger.Info("Saved prediction", "submission_id", res.Submission.SubmissionID)
				}
				continue NextResult
			}
		}

		_, _, average := res.Predictions.Aggregate(b.classes...)
		index, class, confidence := res.Predictions.Max()
		b.logger.Debug("Submission not a false positive",
			"submission_id", res.Submission.SubmissionID,
			b.classes, floatString(average),
			"file", fmt.Sprintf("%d/%d", index+1, len(res.Predictions)),
			"class", class,
			"confidence", floatString(confidence),
		)
		continue
	}
}

func floatString(f float64) string {
	return fmt.Sprintf("%.2f%%", f*100)
}

type base struct {
	logger  *log.Logger
	crypto  *lib.Crypto
	context context.Context
	sid     string
	classes []string

	mu sync.RWMutex
}

type predictionRequest struct {
	Username     string
	FileURLFull  string
	SubmissionID string
}

// Save moves the false positive's file to the dataset folder
func (b *base) Save(prediction *handlers.Prediction) error {
	err := os.MkdirAll("dataset", 0755)
	if err != nil {
		return err
	}
	return os.Rename(prediction.Path, filepath.Join("dataset", filepath.Base(prediction.Path)))
}

// predict downloads the file and predicts the class
func (b *base) predict(req predictionRequest) *handlers.Prediction {
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
	return &handlers.Prediction{
		Path:       fileName,
		Prediction: prediction,
	}
}
