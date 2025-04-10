package walker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/charmbracelet/log"
)

type Config[R any, A any] struct {
	Enabled     bool
	Max         int
	Semaphore   chan struct{}
	Skipper     func(path string) bool
	Do          func(Args[A]) (R, error)
	Args        A
	ConfigCheck func(*A)
}

type Args[A any] struct {
	Context context.Context
	Path    string
	Args    A
}

type Result[R any] struct {
	Path   string
	Result R
}

// WalkDir traverses the folder rooted at "root" and, for each image file,
// spawns a goroutine (limited by a semaphore of size runtime.NumCPU by default)
func WalkDir[R any, A any](ctx context.Context, root string, results chan<- R, config Config[R, A]) error {
	if results == nil {
		return errors.New("results must not be nil")
	}

	defer close(results)
	if !config.Enabled {
		return nil
	}

	if config.Do == nil {
		return errors.New("do function must not be nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if config.Semaphore == nil {
		config.Semaphore = make(chan struct{}, runtime.NumCPU())
	}

	if config.ConfigCheck != nil {
		config.ConfigCheck(&config.Args)
	}

	var (
		count int
		wg    sync.WaitGroup
	)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if config.Skipper != nil && config.Skipper(path) {
			return nil
		}
		if err != nil || info.IsDir() {
			return nil
		}
		if config.Max > 0 && count >= config.Max {
			return filepath.SkipDir
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		count++
		wg.Add(1)

		config.Semaphore <- struct{}{}
		go func(path string) {
			defer func() { <-config.Semaphore; wg.Done() }()
			if !config.Enabled {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			result, err := config.Do(Args[A]{
				Context: ctx,
				Path:    path,
				Args:    config.Args,
			})
			select {
			case <-ctx.Done():
				return
			default:
				if err != nil {
					log.Warnf("%s not found, %v", path, err)
					return
				}
				results <- result
				log.Debugf("Found %s %#v", path, result)
			}
		}(path)

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking the path %s: %w", root, err)
	}
	wg.Wait()
	return nil
}

func Skippers(skippers ...func(path string) bool) func(path string) bool {
	return func(path string) bool {
		for _, skipper := range skippers {
			if skipper == nil {
				continue
			}
			if skipper(path) {
				return true
			}
		}
		return false
	}
}
