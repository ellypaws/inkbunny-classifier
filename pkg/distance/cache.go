package distance

import (
	"sync"

	"github.com/lucasb-eyer/go-colorful"
)

var DefaultCache = &cache{
	RWMutex: new(sync.RWMutex),
	cache:   make(map[colorful.Color]float64),
}

type cache struct {
	*sync.RWMutex
	cache map[colorful.Color]float64
}

func (c *cache) Distance(distance func(colorful.Color, colorful.Color) float64, from, target colorful.Color) float64 {
	c.RLock()
	if v, ok := c.cache[from]; ok {
		c.RUnlock()
		return v
	}
	c.RUnlock()

	d := distance(from, target)

	c.Lock()
	c.cache[from] = d
	c.Unlock()

	return d
}
