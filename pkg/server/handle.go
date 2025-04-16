package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/lucasb-eyer/go-colorful"

	"classifier/pkg/classify"
	"classifier/pkg/distance"
	"classifier/pkg/lib"
	"classifier/pkg/utils"
)

// Respond sends any results from the worker to the client.
func Respond[P ~*T, T any](w http.ResponseWriter, r *http.Request, worker iter.Seq[P]) {
	enc := json.NewEncoder(w)
	if flusher, ok := w.(http.Flusher); ok {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for res := range worker {
			if res == nil {
				continue
			}
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
		if _, err := w.Write([]byte("event: exit\ndata: exit\n\n")); err != nil {
			log.Error("error sending exit event", "err", err)
		}
	} else {
		var allResults []P
		for res := range worker {
			if res == nil {
				continue
			}
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
}

type Result struct {
	Path       string               `json:"path"`
	URL        string               `json:"url,omitempty"`
	Color      *distance.Distance   `json:"color,omitempty"`
	Prediction *classify.Prediction `json:"prediction,omitempty"`
}

type distanceConfig[R io.ReadSeekCloser] struct {
	enabled   bool
	target    colorful.Color
	metric    func(colorful.Color, colorful.Color) float64
	threshold float64
	method    func(string) (R, error)
}

func newDistanceConfig(r *http.Request) (distanceConfig[*os.File], error) {
	colorHex := r.URL.Query().Get("color")
	thresholdStr := r.URL.Query().Get("threshold")
	metricStr := r.URL.Query().Get("metric")
	shouldGetDistance := r.URL.Query().Get("distance") == "true"

	if !shouldGetDistance {
		return distanceConfig[*os.File]{enabled: false}, nil
	}

	if colorHex == "" {
		return distanceConfig[*os.File]{enabled: false}, errors.New("folder and color parameters are required")
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
		return distanceConfig[*os.File]{enabled: false}, errors.New("invalid color format; use hex (e.g. #ff0000)")
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

	return distanceConfig[*os.File]{
		enabled:   shouldGetDistance,
		target:    target,
		metric:    metric,
		threshold: threshold,
		method:    os.Open,
	}, nil
}

func (d *distanceConfig[_]) worker(ctx context.Context) utils.WorkerPool[string, *distance.Distance] {
	return utils.NewWorkerPool(runtime.NumCPU(), func(path string) *distance.Distance {
		if !d.enabled {
			return nil
		}
		log.Info("Starting distance worker", "path", path)
		file, err := d.method(path)
		if err != nil {
			log.Errorf("Error opening file %s: %v", path, err)
			return nil
		}
		defer file.Close()
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		pixelDistance := distance.PixelDistance(ctx, path, file, d.target, d.threshold, d.metric)
		select {
		case <-ctx.Done():
			return nil
		default:
			if !pixelDistance.Found {
				log.Warnf("%s not found, lowest: %.3f", path, pixelDistance.Distance)
				return nil
			}
			log.Debugf("Found %s %#v", path, pixelDistance)
			return &pixelDistance
		}
	})
}

type classifyConfig[R io.ReadSeekCloser] struct {
	enabled bool
	crypto  *lib.Crypto
	method  func(string) (R, error)
}

func (d *classifyConfig[_]) worker(ctx context.Context) utils.WorkerPool[string, *classify.Prediction] {
	return utils.NewWorkerPool(runtime.NumCPU(), func(path string) *classify.Prediction {
		if !d.enabled {
			return nil
		}
		log.Info("Starting classify worker", "path", path)
		file, err := d.method(path)
		if err != nil {
			log.Errorf("Error opening file %s: %v", path, err)
			return nil
		}
		defer file.Close()
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		prediction, err := classify.DefaultCache.Predict(ctx, path, d.crypto.Key(), file)
		select {
		case <-ctx.Done():
			return nil
		default:
			if err != nil {
				log.Error("Error classifying", "path", path, "err", err)
				return nil
			}
			log.Debugf("Found %s %#v", path, prediction)
			return &prediction
		}
	})
}

// Collect processes a file and returns a Result.
func Collect(ctx context.Context, path string, distancePromise <-chan *distance.Distance, predictionPromise <-chan *classify.Prediction) (*Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := Result{
		Path:       path,
		Color:      <-distancePromise,
		Prediction: <-predictionPromise,
	}
	if result.Prediction != nil || result.Color != nil {
		return &result, nil
	} else {
		select {
		case <-ctx.Done():
		default:
			log.Warn("No results found", "path", path)
		}
	}

	return nil, nil
}
