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
	defer classify.DefaultCache.Save("classifications.json")

	done := make(chan os.Signal, 1)
	const port = 8080
	log.Printf("Server starting on http://localhost:%d", port)
	go func() { log.Println(http.ListenAndServe(fmt.Sprintf(":%d", port), nil)); close(done) }()

	wait(done)
}

func wait(done chan os.Signal) {
	signal.Notify(done, os.Interrupt)
	<-done
}
