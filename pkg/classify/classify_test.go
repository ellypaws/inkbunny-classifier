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

func TestPredict(t *testing.T) {
	image := bytes.NewReader(file)
	prediction, err := Predict(context.Background(), image)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Prediction: %v", prediction)
}

func TestCache_CachePrediction(t *testing.T) {
	for range 5 {
		image := bytes.NewReader(file)
		now := time.Now()
		prediction, err := DefaultCache.Predict(context.Background(), "image", image)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Prediction: %v, Time: %v", prediction, time.Since(now))
	}
}
