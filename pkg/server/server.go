package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/lucasb-eyer/go-colorful"

	"jeffy/pkg/classify"
	"jeffy/pkg/distance"
)

//go:embed index.html
var index []byte

// HomeHandler serves the main HTML page with a folder input and color wheel.
func HomeHandler(w http.ResponseWriter, _ *http.Request) {
	// The HTML page includes inline JS to send a GET request to /walk
	// and then display streamed results.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(index)
}

func FileProxy(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Clean(r.PathValue("path")))
}

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
	go func() {
		walkDir(r.Context(), folder, maxFiles, results,
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
			})
		close(results)
	}()

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
					log.Println("error writing data:", err)
					return
				}
				if err := enc.Encode(res); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if _, err := w.Write([]byte("\n")); err != nil {
					log.Println("error writing data:", err)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Finished processing results for %q: distance=%t classify=%t", folder, shouldGetDistance, shouldClassify)
}

type Result struct {
	Path       string               `json:"path"`
	Color      *distance.Result     `json:"color,omitempty"`
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
	if !distanceConfig.enabled && !classifyConfig.enabled {
		return
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
		if !strings.HasSuffix(strings.ToLower(path), ".png") && !strings.HasSuffix(path, ".jpg") {
			return nil
		}
		if max > 0 && count >= max {
			return filepath.SkipDir
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

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
			pixelDistance := distance.PixelDistance(ctx, path, file, distanceConfig.target, distanceConfig.threshold, distanceConfig.metric)
			select {
			case <-ctx.Done():
			default:
				if !pixelDistance.Found {
					log.Printf("%s not found, lowest: %.3f", path, pixelDistance.Distance)
					return
				}
				result.Color = &pixelDistance
				log.Printf("Found %s %#v", path, pixelDistance)
			}
		}(path)

		classifyConfig.semaphore <- struct{}{}
		group.Add(1)
		go func(path string) {
			defer func() { <-classifyConfig.semaphore; group.Done() }()
			if !classifyConfig.enabled {
				return
			}
			prediction, err := classify.DefaultCache.Predict(ctx, path, file)
			select {
			case <-ctx.Done():
			default:
				if err != nil {
					log.Printf("Error classifying %s: %v", path, err)
					return
				}
				result.Prediction = &prediction
				log.Printf("Found %s %#v", path, prediction)
			}
		}(path)

		go func() {
			group.Wait()
			if result.Prediction != nil || result.Color != nil {
				results <- result
			}
			wg.Done()
		}()

		return nil
	})
	if err != nil {
		log.Printf("error walking the path %s: %v", root, err)
	}
	wg.Wait()
}
