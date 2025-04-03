package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"iter"
	"log"
	"os"

	"github.com/lucasb-eyer/go-colorful"
)

//TIP Hi Jeffy

func main() {
	target := color.RGBA{R: 55, G: 59, B: 63, A: 255}
	fileName := "image.png"
	distance, has := hasColor(fileName, target, 0.1)
	fmt.Printf("%s has %v: %.2f %t\n", fileName, target, distance, has)
}

func hasColor(name string, target color.Color, maxDistance float64) (float64, bool) {
	file, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	targetColor, _ := colorful.MakeColor(target)
	for pixel := range pixels(img) {
		distance := pixel.DistanceLab(targetColor)
		if distance <= maxDistance {
			return distance, true
		}
	}
	return -1, false
}

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
