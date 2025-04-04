package classify

import (
	"bytes"
	"context"
	_ "embed"
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

func TestCache_CachePrediction(t *testing.T) {
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
