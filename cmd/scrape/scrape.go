package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"classifier/pkg/classify"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	results := make(chan classify.Result)
	go classify.WalkDir(ctx, os.Getenv("READ_DIR"), results, classify.Config{
		Enabled: true,
		Max:     50000,
		Skipper: targetExists,
	})

Polling:
	for {
		select {
		case r, ok := <-results:
			if !ok {
				cancel()
				break Polling
			}
			if sum := r.Prediction.Whitelist(strings.Split(os.Getenv("CLASSES"), ",")...).Sum(); sum >= 0.85 {
				filePath := target(r.Path)
				if fileExists(filePath) {
					log.Warnf("File %s already exists", filePath)
					continue
				}

				folder := filepath.Dir(filePath)
				err := os.MkdirAll(folder, 0755)
				if err != nil {
					log.Errorf("Error creating folder %s, %v", folder, err)
				}

				source, err := os.Open(r.Path)
				if err != nil {
					log.Errorf("Error opening file %s, %v", r.Path, err)
					continue
				}

				f, err := os.Create(filePath)
				if err != nil {
					log.Printf("Error creating file %s, %v", filePath, err)
					source.Close()
					continue
				}

				written, err := io.Copy(f, source)
				if err != nil {
					log.Errorf("Error writing file %s, %v", filePath, err)
					f.Close()
					source.Close()
					continue
				}
				f.Close()
				source.Close()
				log.Infof("File %s created [%d bytes]", filePath, written)
			}
		case <-ctx.Done():
			break Polling
		}
	}

	log.Info("Exiting")
}

func target(path string) string {
	return filepath.Join(os.Getenv("WRITE_DIR"), strings.TrimPrefix(path, os.Getenv("READ_DIR")))
}

func targetExists(path string) bool {
	return fileExists(target(path))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
