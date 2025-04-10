package classify

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
)

type Config struct {
	Enabled bool
	Max     int
	Skipper func(path string) bool
}

type Result struct {
	Path       string      `json:"path"`
	Prediction *Prediction `json:"prediction,omitempty"`
}

// WalkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU)
func WalkDir(ctx context.Context, root string, results chan<- Result, config Config) {
	if !config.Enabled {
		return
	}

	var (
		count     int
		wg        sync.WaitGroup
		semaphore = make(chan struct{}, runtime.NumCPU())
	)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if config.Skipper != nil && config.Skipper(path) {
			return nil
		}
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".png") && !strings.HasSuffix(path, ".jpg") {
			return nil
		}
		if config.Max > 0 && count >= config.Max {
			return filepath.SkipDir
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		count++
		wg.Add(1)

		result := Result{Path: path}

		semaphore <- struct{}{}
		go func(path string) {
			defer func() { <-semaphore }()
			if !config.Enabled {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			prediction, err := DefaultCache.Predict(ctx, path, file)
			select {
			case <-ctx.Done():
				return
			default:
				if err != nil {
					log.Error("Error classifying", "path", path, "err", err)
					return
				}
				result.Prediction = &prediction
				log.Debugf("Found %s %#v", path, prediction)
			}

			if result.Prediction != nil {
				results <- result
			} else {
				select {
				case <-ctx.Done():
				default:
					log.Warn("No results found", "path", path)
				}
			}
			file.Close()
			wg.Done()
		}(path)

		return nil
	})
	if err != nil {
		log.Errorf("error walking the path %s: %v", root, err)
	}
	wg.Wait()
}
