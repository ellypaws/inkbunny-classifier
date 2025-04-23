package distance

import (
	"context"
	"fmt"
	"os"

	"github.com/lucasb-eyer/go-colorful"

	"classifier/pkg/utils"
	"classifier/pkg/walker"
)

type Result struct {
	Path  string   `json:"path"`
	Color *float64 `json:"color,omitempty"`
}

type Config struct {
	Enabled   bool
	Max       int
	Skipper   func(path string) bool
	Semaphore chan struct{}

	Args
}

type Args struct {
	Target    colorful.Color
	Threshold float64
	Metric    func(colorful.Color, colorful.Color) float64
}

// WalkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU by default)
func WalkDir(ctx context.Context, root string, results chan<- Result, config Config) error {
	return walker.WalkDir(ctx, root, results, walker.Config[Result, Args]{
		Enabled:   config.Enabled,
		Max:       config.Max,
		Semaphore: config.Semaphore,
		Skipper:   walker.Skippers(utils.NotImage, config.Skipper),
		Do:        Do,
		Args:      config.Args,
	})
}

func Do(args walker.Args[Args]) (Result, error) {
	if args.Args.Metric == nil {
		args.Args.Metric = colorful.Color.DistanceLab
	}

	file, err := os.Open(args.Path)
	if err != nil {
		return Result{Path: args.Path}, err
	}
	defer file.Close()
	distance := PixelDistance(args.Context, args.Path, file, args.Args.Target, args.Args.Metric)
	if distance < 0 || distance > args.Args.Threshold {
		return Result{Path: args.Path, Color: nil}, fmt.Errorf("lowest: %.3f", distance)
	}
	return Result{Path: args.Path, Color: &distance}, nil
}
