package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	readSubs := make(map[string][]*Result)
	distanceWorker := distanceConfig.worker(r.Context())
	classifyWorker := classifyConfig.worker(r.Context())
	var mu sync.RWMutex
	var batch sync.WaitGroup
	worker := utils.NewWorkerPool(30, func(submission api.Submission) []*Result {
		defer batch.Done()
		if !utils.IsImage(submission.FileURLFull) {
			return nil
		}
		mu.RLock()
		if _, ok := readSubs[submission.SubmissionID]; ok {
			mu.RUnlock()
			return nil
		}
		mu.RUnlock()

		log.Infof("New submission found https://inkbunny.net/s/%s with %d file%s", submission.SubmissionID, len(submission.Files), utils.Plural(len(submission.Files)))

		folder := filepath.Join("inkbunny", submission.Username)
		err := os.MkdirAll(filepath.Join("inkbunny", submission.Username), 0755)
		if err != nil {
			log.Errorf("Error creating folder %s: %v", submission.SubmissionID, err)
		}

		results := make([]*Result, 0, len(submission.Files))
		for i, file := range submission.Files {
			if err = r.Context().Err(); err != nil {
				break
			}
			if !utils.IsImage(file.FileURLFull) {
				err = fmt.Errorf("file %s is not an image", file.FileURLFull)
				continue
			}

			fileName := filepath.Join(folder, filepath.Base(file.FileURLFull))
			f, err := utils.DownloadEncrypt(r.Context(), classifyConfig.crypto, file.FileURLFull, fileName)
			if err != nil {
				log.Errorf("Error downloading file %d %s: %v", i+1, file.FileURLFull, err)
				continue
			}
			f.Close()
			log.Debugf("Downloaded submission: %v", file.FileURLFull)

			result, err := Collect(r.Context(), fileName, distanceWorker.Promise(fileName), classifyWorker.Promise(fileName))
			if err != nil {
				log.Errorf("Error processing submission %s: %v", file.SubmissionID, err)
				continue
			}
			if result == nil {
				continue
			}
			if result.Prediction == nil && result.Color == nil {
				continue
			}

			if encryptKey != "" {
				result.Path = fmt.Sprintf("%s?key=%s", file.FileURLFull, encryptKey)
			}
			result.URL = fmt.Sprintf("https://inkbunny.net/s/%s-p%d", file.SubmissionID, i+1)
			results = append(results, result)
		}

		mu.Lock()
		readSubs[submission.SubmissionID] = results
		mu.Unlock()
		if len(results) == 0 {
			log.Warn("No prediction found", "submission", submission.SubmissionID, "error", err)
			return nil
		}
		return results
	})

	// Submission watcher, adds new submissions to worker
	go func() {
		timeout := 30 * time.Second
		if t, err := strconv.ParseInt(refreshRate, 10, 64); err != nil && t > 0 {
			timeout = time.Duration(t) * time.Second
		}
		defer func() { worker.Close() }()
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
			var submissionIDs []string
			for _, submission := range response.Submissions {
				submissionIDs = append(submissionIDs, submission.SubmissionID)
			}
			details, err := api.Credentials{Sid: sid}.SubmissionDetails(api.SubmissionDetailsRequest{SID: sid, SubmissionIDs: strings.Join(submissionIDs, ",")})
			if err != nil {
				log.Errorf("Error getting submission details: %v", err)
				continue
			}
			batch.Add(len(details.Submissions))
			worker.Add(details.Submissions...)
			batch.Wait()
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
	Respond(w, r, utils.Unpack(worker.Iter()))
	classifyWorker.Close()
	distanceWorker.Close()
	log.Info("Finished watching for new submissions")
}
