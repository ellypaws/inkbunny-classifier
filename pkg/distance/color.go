package distance

import (
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"iter"

	"github.com/charmbracelet/log"
	"github.com/lucasb-eyer/go-colorful"
)

type Result struct {
	Found    bool    `json:"found,omitempty"`
	Distance float64 `json:"distance,omitempty"`
}

// PixelDistance inspects the image at filename and returns the Lab distance
// to target along with a boolean indicating if a pixel is found within maxDistance.
func PixelDistance(ctx context.Context, name string, file io.ReadSeeker, target colorful.Color, maxDistance float64, distanceFunc func(colorful.Color, colorful.Color) float64) Result {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		log.Errorf("Error seeking to beginning of file: %v", err)
		return Result{Found: false, Distance: -1}
	}
	img, _, err := image.Decode(file)
	if err != nil {
		log.Errorf("error decoding %s: %v", name, err)
		return Result{Found: false, Distance: -1}
	}

	lowest := -1.0
	for pixel := range pixels(img) {
		select {
		case <-ctx.Done():
			break
		default:
			distance := DefaultCache.Distance(distanceFunc, pixel, target)
			if distance <= maxDistance {
				if lowest < 0 {
					lowest = distance
				} else {
					lowest = min(lowest, distance)
				}
			}
		}
	}
	return Result{Found: lowest >= 0, Distance: lowest}
}

// pixels is an iterator over all the pixels in an image.
// It uses your iter.Seq2 type from the iter package.
func pixels(m image.Image) iter.Seq2[colorful.Color, bool] {
	return func(yield func(colorful.Color, bool) bool) {
		b := m.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if !yield(colorful.MakeColor(m.At(x, y))) {
					return
				}
			}
		}
	}
}
