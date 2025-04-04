package distance

import (
	"sync"

	"github.com/lucasb-eyer/go-colorful"
)

var DefaultCache = &cache{
	RWMutex: new(sync.RWMutex),
	cache:   make(map[string]float64),
}

type cache struct {
	*sync.RWMutex
	cache map[string]float64
}

func (c *cache) Distance(distance func(colorful.Color, colorful.Color) float64, from, target colorful.Color) float64 {
	hex := from.Hex()
	c.RLock()
	if v, ok := c.cache[hex]; ok {
		c.RUnlock()
		return v
	}
	c.RUnlock()

	c.Lock()
	d := distance(from, target)
	c.cache[hex] = d
	c.Unlock()

	return d
}
