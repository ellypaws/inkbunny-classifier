package utils

import (
	"io"
	"net/http"
	"testing"
	"time"
)

const testString = "Hello, world!"

func init() {
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(testString))
		if err != nil {
			panic(err)
		}
	})
	_ = http.ListenAndServe("localhost:8000", nil)
}

func TestCache_RetrieveFile(t *testing.T) {
	for range 5 {
		now := time.Now()
		reader, err := DefaultCache.RetrieveFile("http://localhost:8000")
		if err != nil {
			t.Fatalf("failed to retrieve file: %v", err)
		}
		written, err := io.Copy(io.Discard, reader)
		if err != nil {
			t.Error(err)
		}
		if written != int64(len(testString)) {
			t.Errorf("expected %d bytes written, got %d", len(testString), written)
		}
		t.Logf("Time taken: %s, %d bytes", time.Since(now), written)
	}
}
