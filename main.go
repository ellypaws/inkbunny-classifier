package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Serve the home page and API endpoints.
	http.HandleFunc("GET /", homeHandler)
	http.HandleFunc("GET /walk", walkHandler)
	http.HandleFunc("GET /file/{path}", fileProxy)

	const port = 8080
	log.Printf("Server starting on http://localhost:%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
