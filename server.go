package main

import (
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	results := make(chan Result)
	// start walking the directory in a separate goroutine
	go func() {
		walkDir(folder, target, threshold, results)
		close(results)
	}()

	// stream results as they come in
	enc := json.NewEncoder(w)
	for msg := range results {
		if err := enc.Encode(msg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		flusher.Flush()
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
func walkDir(root string, target colorful.Color, threshold float64, results chan<- Result) {
	// convert colorful target to standard color.Color (RGBA)
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("error accessing %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// only process common image file types (you can extend as needed)
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			return nil
		}

		wg.Add(1)
		sem <- struct{}{} // acquire semaphore
		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore
			distance, found := hasColor(path, target, threshold)
			if !found {
				return
			}
			results <- Result{
				Path:     path,
				Found:    found,
				Distance: distance,
			}
		}(path)
		return nil
	})
	if err != nil {
		log.Printf("error walking the path %v: %v", root, err)
	}
	wg.Wait()
}
