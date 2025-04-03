package main

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
)

//go:embed index.html
var index []byte

// homeHandler serves the main HTML page with a folder input and color wheel.
func homeHandler(w http.ResponseWriter, _ *http.Request) {
	// The HTML page includes inline JS to send a GET request to /walk
	// and then display streamed results.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(index)
}

func fileProxy(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Clean(r.PathValue("path")))
}

// walkHandler is the HTTP API endpoint that receives query parameters,
// starts the walkDir process, and streams results back using Flush.
func walkHandler(w http.ResponseWriter, r *http.Request) {
	// get query parameters: folder, color (as hex) and optional threshold
	folder := r.URL.Query().Get("folder")
	colorHex := r.URL.Query().Get("color")
	thresholdStr := r.URL.Query().Get("threshold")
	maxStr := r.URL.Query().Get("max")
	metricStr := r.URL.Query().Get("metric")

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

	// make sure streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")

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

	ctx := r.Context()
	results := make(chan Result)

	go func() {
		walkDir(ctx, folder, target, threshold, maxFiles, metric, results)
		close(results)
	}()

	enc := json.NewEncoder(w)
	for res := range results {
		select {
		case <-ctx.Done():
			return // interrupt detected
		default:
			_ = enc.Encode(res)
			flusher.Flush()
		}
	}
}

type Result struct {
	Path     string  `json:"path,omitempty"`
	Found    bool    `json:"found,omitempty"`
	Distance float64 `json:"distance,omitempty"`
}

// walkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU)
// that runs hasColor. The results (a string message per file) are sent
// into the results channel.
func walkDir(ctx context.Context, root string, target colorful.Color, threshold float64, max int, distanceFunc func(colorful.Color, colorful.Color) float64, results chan<- Result) {
	// convert colorful target to standard color.Color (RGBA)
	sem := make(chan struct{}, runtime.NumCPU())
	var (
		count int
		wg    sync.WaitGroup
	)

	if distanceFunc == nil {
		distanceFunc = colorful.Color.DistanceLab
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
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		count++
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer func() { <-sem; wg.Done() }()
			distance, found := hasColor(path, target, threshold, distanceFunc)
			if !found {
				log.Printf("%s not found, lowest: %.3f", path, distance)
				return
			}
			result := Result{Path: path, Found: found, Distance: distance}
			log.Printf("Found %#v", result)
			results <- result
		}(path)
		return nil
	})
	if err != nil {
		log.Printf("error walking the path %v: %v", root, err)
	}
	wg.Wait()
}
