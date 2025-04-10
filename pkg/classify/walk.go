package classify

import (
	"context"
	"os"

	"github.com/charmbracelet/log"

	"classifier/pkg/utils"
	"classifier/pkg/walker"
)

type Result struct {
	Path       string      `json:"path"`
	Prediction *Prediction `json:"prediction,omitempty"`
}

type Config struct {
	Enabled   bool
	Max       int
	Skipper   func(path string) bool
	Semaphore chan struct{}
}

// WalkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU by default)
func WalkDir(ctx context.Context, root string, results chan<- Result, config Config) error {
	return walker.WalkDir(ctx, root, results, walker.Config[Result, Config]{
		Enabled:   config.Enabled,
		Max:       config.Max,
		Semaphore: config.Semaphore,
		Skipper:   walker.Skippers(utils.NotImage, config.Skipper),
		Do:        Do,
		Args:      config,
	})
}

func Do(args walker.Args[Config]) (Result, error) {
	file, err := os.Open(args.Path)
	if err != nil {
		return Result{Path: args.Path}, err
	}
	defer file.Close()
	prediction, err := DefaultCache.Predict(args.Context, args.Path, file)
	if err != nil {
		log.Error("Error classifying", "path", args.Path, "err", err)
		return Result{Path: args.Path, Prediction: nil}, err
	}
	log.Debugf("Found %s %#v", args.Path, prediction)
	return Result{Path: args.Path, Prediction: &prediction}, nil
}
