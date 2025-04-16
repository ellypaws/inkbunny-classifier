package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/lucasb-eyer/go-colorful"

	"classifier/pkg/lib"
	"classifier/pkg/utils"
)

// WalkHandler is the HTTP API endpoint that receives query parameters,
// starts the walkDir process, and streams results back using Flush.
func WalkHandler(w http.ResponseWriter, r *http.Request) {
	// get query parameters: folder, color (as hex) and optional threshold
	folder := r.URL.Query().Get("folder")
	maxStr := r.URL.Query().Get("max")
	shouldClassify := r.URL.Query().Get("classify") == "true"
	encryptKey := r.URL.Query().Get("encrypt_key")

	if folder == "" {
		http.Error(w, "folder parameter is required", http.StatusBadRequest)
		return
	}

	maxFiles := -1
	if maxStr != "" {
		if m, err := strconv.Atoi(maxStr); err == nil && m > 0 {
			maxFiles = m
		}
	}

	if maxFiles < 1 {
		return
	}

	distanceConfig, err := newDistanceConfig(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	crypto, err := lib.NewCrypto(encryptKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	classifyConfig := classifyConfig[*lib.CryptoFile]{
		enabled: shouldClassify,
		crypto:  crypto,
		method:  crypto.OpenWithMethod(crypto.Encrypt), // because we expect local files to be unencrypted, we encrypt before calling classify.Predict
	}

	if !distanceConfig.enabled && !classifyConfig.enabled {
		return
	}

	results := make(chan *Result)
	go walkDir(r.Context(), folder, maxFiles, results,
		distanceConfig,
		classifyConfig,
	)

	Respond(w, r, utils.Iter(results))
	log.Info("Finished processing results for", "folder", folder, "distance", distanceConfig.enabled, "classify", shouldClassify)
}

// walkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU)
func walkDir(ctx context.Context, root string, max int, results chan<- *Result, distanceConfig distanceConfig[*os.File], classifyConfig classifyConfig[*lib.CryptoFile]) {
	defer close(results)
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		count int
		wg    sync.WaitGroup
	)

	if distanceConfig.metric == nil {
		distanceConfig.metric = colorful.Color.DistanceLab
	}

	distanceWorker := distanceConfig.worker(ctx)
	classifyWorker := classifyConfig.worker(ctx)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if utils.NotImage(path) {
			return nil
		}
		if max > 0 && count >= max {
			return filepath.SkipDir
		}

		count++
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Collect(ctx, path, distanceWorker.Promise(path), classifyWorker.Promise(path))
			if err != nil {
				return
			}
			results <- result
		}()

		return nil
	})
	if err != nil {
		log.Errorf("error walking the path %s: %v", root, err)
	}
	distanceWorker.Work()
	classifyWorker.Work()
	wg.Wait()
	distanceWorker.Close()
	classifyWorker.Close()
	distanceWorker.Wait()
	classifyWorker.Wait()
}
