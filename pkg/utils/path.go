package utils

import (
	"path/filepath"
	"strings"
)

func IsImage(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".png":
		return true
	case ".jpg", ".jpeg":
		return true
	case ".gif":
		return true
	case ".webp":
		return true
	default:
		return false
	}
}

func NotImage(path string) bool {
	return !IsImage(path)
}
