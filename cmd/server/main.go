package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/charmbracelet/log"

	"classifier/pkg/classify"
	"classifier/pkg/server"
)

func main() {
	// Serve the home page and API endpoints.
	http.HandleFunc("GET /", server.HomeHandler)
	http.HandleFunc("GET /watch", server.Watcher)
	http.HandleFunc("GET /walk", server.WalkHandler)
	http.HandleFunc("GET /file/{path}", server.FileProxy)

	if os.Getenv("SKIP_LOAD") != "true" {
		classify.DefaultCache.Load("classifications.json")
	}
	defer classify.DefaultCache.Save("classifications.json")

	log.Default().SetLevel(log.DebugLevel)

	done := make(chan os.Signal, 1)
	const port = 8080
	log.Infof("Server starting on http://localhost:%d", port)
	go func() { log.Print(http.ListenAndServe(fmt.Sprintf(":%d", port), nil)); close(done) }()

	wait(done)
}

func wait(done chan os.Signal) {
	signal.Notify(done, os.Interrupt)
	<-done
}
