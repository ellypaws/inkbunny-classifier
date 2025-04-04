package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"jeffy/pkg/classify"
	"jeffy/pkg/server"
)

func main() {
	// Serve the home page and API endpoints.
	http.HandleFunc("GET /", server.HomeHandler)
	http.HandleFunc("GET /walk", server.WalkHandler)
	http.HandleFunc("GET /file/{path}", server.FileProxy)

	classify.DefaultCache.Load("classifications.json")

	const port = 8080
	log.Printf("Server starting on http://localhost:%d", port)
	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	wait()
	classify.DefaultCache.Save("classifications.json")
}

func wait() {
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	select {
	case <-done:
		return
	}
}
