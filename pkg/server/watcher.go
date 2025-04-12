package server

import (
	"encoding/json"
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

	enc := json.NewEncoder(w)
	if flusher, ok := w.(http.Flusher); ok {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for res := range worker.Work() {
			select {
			case <-r.Context().Done():
				break // interrupt detected
			default:
				if _, err := w.Write([]byte("data: ")); err != nil {
					log.Error("error writing data:", "err", err)
					return
				}
				if err := enc.Encode(res.Response); err != nil {
					log.Error("error writing data:", "err", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if _, err := w.Write([]byte("\n")); err != nil {
					log.Error("error writing data:", "err", err)
					return
				}
				flusher.Flush()
			}
		}
		if _, err := w.Write([]byte("event: exit\ndata: exit\n\n")); err != nil {
			log.Error("error sending exit event", "err", err)
		}
	} else {
		var allResults []Result
		for res := range worker.Work() {
			select {
			case <-r.Context().Done():
				break
			default:
				allResults = append(allResults, res.Response)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := enc.Encode(allResults); err != nil {
			log.Error("error writing data:", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
