package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func RetrieveFile(url string) (io.Reader, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	file := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, err
	}

	return file, nil
}

var DefaultCache = Cache{store: make(map[string][]byte)}

type Cache struct {
	sync.RWMutex
	store map[string][]byte
}

func (c *Cache) RetrieveFile(url string) (io.Reader, error) {
	c.RLock()
	if bin, ok := c.store[url]; ok {
		c.RUnlock()
		return bytes.NewReader(bin), nil
	}
	c.RUnlock()

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	file := bytes.NewBuffer(make([]byte, 0, resp.ContentLength))
	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, err
	}

	c.Lock()
	c.store[url] = file.Bytes()
	c.Unlock()

	return file, nil
}
