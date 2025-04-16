package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/ellypaws/inkbunny/api"

	"classifier/pkg/lib"
	"classifier/pkg/utils"
)

func Watcher(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("sid")
	encryptKey := r.URL.Query().Get("encrypt_key")
	shouldClassify := r.URL.Query().Get("classify") == "true"
	refreshRate := r.URL.Query().Get("refresh_rate_seconds")

	distanceConfig, err := newDistanceConfig(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	crypto, err := lib.NewCrypto(encryptKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	classifyConfig := classifyConfig[*os.File]{
		enabled: shouldClassify,
		crypto:  crypto,
		method:  os.Open, // we expect the files to already be encrypted after calling utils.DownloadEncrypt
	}

	if !distanceConfig.enabled && !classifyConfig.enabled {
		return
	}

	readSubs := make(map[string]*Result)
	distanceWorker := distanceConfig.worker(r.Context())
	classifyWorker := classifyConfig.worker(r.Context())
	var mu sync.RWMutex
	worker := utils.NewWorkerPool(30, func(submission api.SubmissionSearchList) *Result {
		if !utils.IsImage(submission.FileURLFull) {
			return nil
		}
		mu.RLock()
		if _, ok := readSubs[submission.SubmissionID]; ok {
			mu.RUnlock()
			return nil
		}
		mu.RUnlock()

		log.Infof("New submission found https://inkbunny.net/s/%s", submission.SubmissionID)

		folder := filepath.Join("inkbunny", submission.Username)
		err := os.MkdirAll(filepath.Join("inkbunny", submission.Username), 0755)
		if err != nil {
			log.Errorf("Error creating folder %s: %v", submission.SubmissionID, err)
		}

		fileName := filepath.Join(folder, filepath.Base(submission.FileURLFull))
		_, err = utils.DownloadEncrypt(r.Context(), classifyConfig.crypto, submission.FileURLFull, fileName)
		if err != nil {
			log.Errorf("Error downloading submission %s: %v", submission.SubmissionID, err)
			return nil
		}
		log.Debugf("Downloaded submission: %v", submission.FileURLFull)

		result, err := Collect(r.Context(), fileName, distanceWorker.Promise(fileName), classifyWorker.Promise(fileName))
		if err != nil {
			log.Errorf("Error processing submission %s: %v", submission.SubmissionID, err)
			return nil
		}
		if result == nil {
			return nil
		}
		if result.Prediction == nil && result.Color == nil {
			return nil
		}

		if encryptKey != "" {
			result.Path = fmt.Sprintf("%s?key=%s", submission.FileURLFull, encryptKey)
		}
		result.URL = fmt.Sprintf("https://inkbunny.net/s/%s", submission.SubmissionID)

		go func() { mu.Lock(); readSubs[submission.SubmissionID] = result; mu.Unlock() }()
		return result
	})

	go func() {
		timeout := 30 * time.Second
		if t, err := strconv.ParseInt(refreshRate, 10, 64); err != nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
		defer func() { worker.Close(); classifyWorker.Close(); distanceWorker.Close() }()
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
	classifyWorker.Work()
	distanceWorker.Work()
	log.Info("Starting watcher", "distance", distanceConfig.enabled, "classify", classifyConfig.enabled)
	Respond(w, r, worker.Iter())
	log.Info("Finished watching for new submissions")
}
