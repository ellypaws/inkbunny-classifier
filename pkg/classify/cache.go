package classify

import (
	"context"
	"io"
	"os"
	"sync"

	"classifier/pkg/utils"
)

var DefaultCache = &cache{
	RWMutex:     new(sync.RWMutex),
	predictions: make(map[string]Prediction),
}

type cache struct {
	*sync.RWMutex
	predictions map[string]Prediction
}

func (c *cache) reset() {
	c.RWMutex = new(sync.RWMutex)
	c.predictions = make(map[string]Prediction)
}

func (c *cache) Save(name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	c.RLock()
	defer c.RUnlock()
	return utils.EncodeIndent(f, c.predictions, "  ")
}

func (c *cache) Load(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	predictions, err := utils.DecodeAndClose[map[string]Prediction](f)
	if err != nil {
		return err
	}
	c.Lock()
	c.predictions = predictions
	c.Unlock()
	return nil
}

func (c *cache) Predict(ctx context.Context, name string, file io.ReadSeeker) (Prediction, error) {
	c.RLock()
	if v, ok := c.predictions[name]; ok {
		c.RUnlock()
		return v, nil
	}
	c.RUnlock()

	c.Lock()
	defer c.Unlock()
	d, err := Predict(ctx, file)
	if err != nil {
		return nil, err
	}
	c.predictions[name] = d

	return d, nil
}

func (c *cache) PredictURL(ctx context.Context, path string) (Prediction, error) {
	c.RLock()
	if v, ok := c.predictions[path]; ok {
		c.RUnlock()
		return v, nil
	}
	c.RUnlock()

	c.Lock()
	defer c.Unlock()
	d, err := PredictURL(ctx, path)
	if err != nil {
		return nil, err
	}
	c.predictions[path] = d

	return d, nil
}
