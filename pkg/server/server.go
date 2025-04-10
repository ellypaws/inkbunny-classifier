package server

import (
	_ "embed"
	"net/http"
	"path/filepath"
)

//go:embed index.html
var index []byte

// HomeHandler serves the main HTML page
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, "pkg/server/index.html")
}

func FileProxy(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Clean(r.PathValue("path")))
}
