package distance

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"iter"

	_ "golang.org/x/image/webp"

	"github.com/charmbracelet/log"
	"github.com/lucasb-eyer/go-colorful"
)

// PixelDistance inspects the image at reader and returns the distance according to distanceFunc.
// It returns the lowest distance found.
func PixelDistance(ctx context.Context, name string, file io.Reader, target colorful.Color, distanceFunc func(colorful.Color, colorful.Color) float64) float64 {
	img, _, err := image.Decode(file)
	if err != nil {
		log.Errorf("error decoding %s: %v", name, err)
		return -1
	}

	lowest := -1.0
	for pixel := range pixels(img) {
		select {
		case <-ctx.Done():
			break
		default:
			distance := DefaultCache.Distance(distanceFunc, pixel, target)
			if lowest < 0 {
				lowest = distance
			} else {
				lowest = min(lowest, distance)
			}
		}
	}
	return lowest
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
