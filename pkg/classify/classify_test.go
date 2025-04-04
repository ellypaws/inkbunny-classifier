package classify

import (
	"bytes"
	"context"
	_ "embed"
	"net/http"
	"sync"
	"testing"
	"time"
)

//go:embed jeffy.png
var file []byte
var image = bytes.NewReader(file)

func TestPredict(t *testing.T) {
	prediction, err := Predict(context.Background(), image)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Prediction: %v", prediction)
}

func TestCache_Predict(t *testing.T) {
	DefaultCache.reset()
	for range 5 {
		now := time.Now()
		prediction, err := DefaultCache.Predict(context.Background(), "image", image)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Prediction: %v, Time: %v", prediction, time.Since(now))
	}
}

const imagePath = "http://localhost:8000/image.png"

var proxy sync.Once

func start() {
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(file)
		if err != nil {
			panic(err)
		}
	})
	go http.ListenAndServe("localhost:8000", nil)
}

var warmupOnce sync.Once

func warmup() {
	proxy.Do(start)
	warmupOnce.Do(func() { PredictURL(context.Background(), imagePath) })
	DefaultCache.reset()
}

func TestPredictURL(t *testing.T) {
	proxy.Do(start)
	warmup()
	prediction, err := PredictURL(context.Background(), imagePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Prediction: %v", prediction)
}

func TestCache_PredictURL(t *testing.T) {
	proxy.Do(start)
	warmup()
	DefaultCache.reset()
	prediction, err := DefaultCache.PredictURL(context.Background(), imagePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Prediction: %v", prediction)
}

func BenchmarkPredict(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Predict(context.Background(), image)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPredictURL(b *testing.B) {
	proxy.Do(start)
	warmup()
	for i := 0; i < b.N; i++ {
		_, err := PredictURL(context.Background(), imagePath)
		if err != nil {
			b.Fatal(err)
		}
	}
}
