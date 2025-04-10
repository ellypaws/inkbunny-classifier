package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/lucasb-eyer/go-colorful"

	"classifier/pkg/classify"
	"classifier/pkg/distance"
	"classifier/pkg/utils"
)

// WalkHandler is the HTTP API endpoint that receives query parameters,
// starts the walkDir process, and streams results back using Flush.
func WalkHandler(w http.ResponseWriter, r *http.Request) {
	// get query parameters: folder, color (as hex) and optional threshold
	folder := r.URL.Query().Get("folder")
	colorHex := r.URL.Query().Get("color")
	thresholdStr := r.URL.Query().Get("threshold")
	maxStr := r.URL.Query().Get("max")
	metricStr := r.URL.Query().Get("metric")
	shouldGetDistance := r.URL.Query().Get("distance") == "true"
	shouldClassify := r.URL.Query().Get("classify") == "true"

	if folder == "" || colorHex == "" {
		http.Error(w, "folder and color parameters are required", http.StatusBadRequest)
		return
	}

	threshold := 0.1
	if thresholdStr != "" {
		if t, err := strconv.ParseFloat(thresholdStr, 64); err == nil {
			threshold = t
		}
	}

	maxFiles := -1
	if maxStr != "" {
		if m, err := strconv.Atoi(maxStr); err == nil && m > 0 {
			maxFiles = m
		}
	}

	// parse the hex color using go-colorful (expects "#RRGGBB")
	target, err := colorful.Hex(colorHex)
	if err != nil {
		http.Error(w, "invalid color format; use hex (e.g. #ff0000)", http.StatusBadRequest)
		return
	}

	metric := colorful.Color.DistanceLab
	switch metricStr {
	case "DistanceRgb":
		metric = colorful.Color.DistanceRgb
	case "DistanceLab":
		metric = colorful.Color.DistanceLab
	case "DistanceLuv":
		metric = colorful.Color.DistanceLuv
	case "DistanceCIE76":
		metric = colorful.Color.DistanceCIE76
	case "DistanceCIE94":
		metric = colorful.Color.DistanceCIE94
	case "DistanceCIEDE2000":
		metric = colorful.Color.DistanceCIEDE2000
	default:
		metric = colorful.Color.DistanceLab
	}

	results := make(chan Result)
	go walkDir(r.Context(), folder, maxFiles, results,
		distanceConfig{
			enabled:   shouldGetDistance,
			target:    target,
			metric:    metric,
			threshold: threshold,
			semaphore: make(chan struct{}, runtime.NumCPU()),
		},
		classifyConfig{
			enabled:   shouldClassify,
			semaphore: make(chan struct{}, runtime.NumCPU()*2),
		},
	)

	enc := json.NewEncoder(w)
	if flusher, ok := w.(http.Flusher); ok {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for res := range results {
			select {
			case <-r.Context().Done():
				break // interrupt detected
			default:
				if _, err := w.Write([]byte("data: ")); err != nil {
					log.Error("error writing data:", "err", err)
					return
				}
				if err := enc.Encode(res); err != nil {
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
	} else {
		var allResults []Result
		for res := range results {
			select {
			case <-r.Context().Done():
				break
			default:
				allResults = append(allResults, res)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := enc.Encode(allResults); err != nil {
			log.Error("error writing data:", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	log.Info("Finished processing results for", "folder", folder, "distance", shouldGetDistance, "classify", shouldClassify)
}

type Result struct {
	Path       string               `json:"path"`
	URL        string               `json:"url,omitempty"`
	Color      *distance.Distance   `json:"color,omitempty"`
	Prediction *classify.Prediction `json:"prediction,omitempty"`
}

type distanceConfig struct {
	enabled   bool
	target    colorful.Color
	metric    func(colorful.Color, colorful.Color) float64
	threshold float64
	semaphore chan struct{}
}
type classifyConfig struct {
	enabled   bool
	semaphore chan struct{}
}

// walkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU)
func walkDir(ctx context.Context, root string, max int, results chan<- Result, distanceConfig distanceConfig, classifyConfig classifyConfig) {
	defer close(results)
	if !distanceConfig.enabled && !classifyConfig.enabled {
		return
	}

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

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		count++
		wg.Add(1)

		var (
			group  sync.WaitGroup
			result = Result{Path: path}
		)

		distanceConfig.semaphore <- struct{}{}
		group.Add(1)
		go func(path string) {
			defer func() { <-distanceConfig.semaphore; group.Done() }()
			if !distanceConfig.enabled {
				return
			}
			file, err := os.Open(path)
			if err != nil {
				log.Errorf("Error opening file %s: %v", path, err)
				return
			}
			defer file.Close()
			select {
			case <-ctx.Done():
				return
			default:
			}
			pixelDistance := distance.PixelDistance(ctx, path, file, distanceConfig.target, distanceConfig.threshold, distanceConfig.metric)
			select {
			case <-ctx.Done():
			default:
				if !pixelDistance.Found {
					log.Warnf("%s not found, lowest: %.3f", path, pixelDistance.Distance)
					return
				}
				result.Color = &pixelDistance
				log.Debugf("Found %s %#v", path, pixelDistance)
			}
		}(path)

		classifyConfig.semaphore <- struct{}{}
		group.Add(1)
		go func(path string) {
			defer func() { <-classifyConfig.semaphore; group.Done() }()
			if !classifyConfig.enabled {
				return
			}
			file, err := os.Open(path)
			if err != nil {
				log.Errorf("Error opening file %s: %v", path, err)
				return
			}
			defer file.Close()
			select {
			case <-ctx.Done():
				return
			default:
			}
			prediction, err := classify.DefaultCache.Predict(ctx, path, file)
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
		}(path)

		go func() {
			group.Wait()
			if result.Prediction != nil || result.Color != nil {
				results <- result
			} else {
				select {
				case <-ctx.Done():
				default:
					log.Warn("No results found", "path", path)
				}
			}
			wg.Done()
		}()

		return nil
	})
	if err != nil {
		log.Errorf("error walking the path %s: %v", root, err)
	}
	wg.Wait()
}
