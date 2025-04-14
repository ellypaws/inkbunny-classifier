package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
		enabled:   shouldClassify,
		semaphore: make(chan struct{}, runtime.NumCPU()*2),
		crypto:    crypto,
		method:    os.Open, // we expect the files to already be encrypted after calling utils.DownloadEncrypt
	}

	if !distanceConfig.enabled && !classifyConfig.enabled {
		return
	}

	readSubs := make(map[string]*Result)
	var mu sync.RWMutex
	worker := utils.NewWorkerPool(50, func(submission api.SubmissionSearchList, yield func(*Result)) {
		if !utils.IsImage(submission.FileURLFull) {
			return
		}
		mu.RLock()
		if _, ok := readSubs[submission.SubmissionID]; ok {
			mu.RUnlock()
			return
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
			return
		}
		log.Debugf("Downloaded submission: %v", submission.FileURLFull)

		result, err := Handle(r.Context(), fileName, distanceConfig, classifyConfig)
		if err != nil {
			log.Errorf("Error processing submission %s: %v", submission.SubmissionID, err)
			return
		}
		if result == nil {
			return
		}
		if result.Prediction == nil && result.Color == nil {
			return
		}

		if encryptKey != "" {
			result.Path = fmt.Sprintf("%s?key=%s", submission.FileURLFull, encryptKey)
		}
		result.URL = fmt.Sprintf("https://inkbunny.net/s/%s", submission.SubmissionID)

		yield(result)
		go func() { mu.Lock(); readSubs[submission.SubmissionID] = result; mu.Unlock() }()
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
	log.Info("Starting watcher", "distance", distanceConfig.enabled, "classify", shouldClassify)
	Respond(w, r, worker.Iter())
	log.Info("Finished watching for new submissions")
}
