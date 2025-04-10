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
	go func() {
		classify.WalkDir(ctx, os.Getenv("READ_DIR"), results, classify.Config{
			Enabled: true,
			Max:     50000,
			Skipper: fileExists,
		})
		close(results)
	}()

Polling:
	for {
		select {
		case r, ok := <-results:
			if !ok {
				cancel()
				break Polling
			}
			if class := (*r.Prediction)[os.Getenv("CLASS")]; class >= 0.85 {
				filePath := filepath.Join(os.Getenv("WRITE_DIR"), strings.TrimPrefix(r.Path, os.Getenv("READ_DIR")))
				if fileExists(filePath) {
					log.Printf("File %s already exists", filePath)
					continue
				}

				folder := filepath.Dir(filePath)
				err := os.MkdirAll(folder, 0755)
				if err != nil {
					log.Printf("Error creating folder %s, %v", folder, err)
				}

				source, err := os.Open(r.Path)
				if err != nil {
					log.Printf("Error opening file %s, %v", r.Path, err)
					continue
				}
				defer source.Close()

				f, err := os.Create(filePath)
				if err != nil {
					log.Printf("Error creating file %s, %v", filePath, err)
					continue
				}
				defer f.Close()

				written, err := io.Copy(f, source)
				if err != nil {
					log.Printf("Error writing file %s, %v", filePath, err)
					continue
				}
				log.Printf("File %s created [%d bytes]", filePath, written)
			}
		case <-ctx.Done():
			break Polling
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
