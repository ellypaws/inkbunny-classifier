package main

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"iter"
	"log"
	"os"

	"github.com/lucasb-eyer/go-colorful"
)

// hasColor inspects the image at filename and returns the Lab distance
// to target along with a boolean indicating if a pixel is found within maxDistance.
func hasColor(name string, target colorful.Color, maxDistance float64) (float64, bool) {
	file, err := os.Open(name)
	if err != nil {
		log.Printf("error opening %s: %v", name, err)
		return -1, false
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		log.Printf("error decoding %s: %v", name, err)
		return -1, false
	}

	for pixel := range pixels(img) {
		distance := pixel.DistanceLab(target)
		if distance <= maxDistance {
			return distance, true
		}
	}
	return -1, false
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
