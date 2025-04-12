package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ellypaws/inkbunny/api"

	"classifier/pkg/classify"
	"classifier/pkg/lib"
	"classifier/pkg/utils"
)

func Watcher(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("sid")
	encryptKey := r.URL.Query().Get("encrypt_key")
	shouldClassify := r.URL.Query().Get("classify") == "true"
	refreshRate := r.URL.Query().Get("refresh_rate_seconds")

	if !shouldClassify {
		return
	}

	crypto, err := lib.NewCrypto(encryptKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

			log.Infof("New submission found https://inkbunny.net/s/%s", submission.SubmissionID)

			file, err := utils.DownloadEncrypt(r.Context(), crypto, submission.FileURLFull, filepath.Join("inkbunny", submission.Username))
			if err != nil {
				log.Errorf("Error downloading submission %s: %v", submission.SubmissionID, err)
				continue
			}
			log.Debugf("Downloaded submission: %v", submission.FileURLFull)

			prediction, err := classify.DefaultCache.Predict(r.Context(), submission.FileURLFull, file)
			file.Close()
			if err != nil {
				log.Errorf("Error predicting submission: %v", err)
				continue
			}
			log.Infof("Classified submission https://inkbunny.net/%s: %+v", submission.SubmissionID, prediction)

			go func() { mu.Lock(); readSubs[submission.SubmissionID] = prediction; mu.Unlock() }()

			if encryptKey != "" {
				submission.FileURLFull = fmt.Sprintf("%s?key=%s", submission.FileURLFull, encryptKey)
			}
			yield(Result{
				Path:       submission.FileURLFull,
				URL:        fmt.Sprintf("https://inkbunny.net/s/%s", submission.SubmissionID),
				Prediction: &prediction,
			})
		}
	})

	go func() {
		timeout := 30 * time.Second
		if t, err := strconv.ParseInt(refreshRate, 10, 64); err != nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
		defer worker.Close()
		for r.Context().Err() == nil {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			user := api.Credentials{Sid: sid}
			request := api.SubmissionSearchRequest{
				SID:    sid,
				GetRID: true,
			}
			response, err := user.SearchSubmissions(request)
			if err != nil {
				log.Errorf("Error searching submissions: %v", err)
			}
			worker.Add(response.Submissions...)
			select {
			case <-r.Context().Done():
				return
			case <-time.After(timeout):
				continue
			}
		}
	}()

	worker.Work()
	Handle(w, r, worker.Iter())
	log.Info("Finished watching for new submissions")
}
