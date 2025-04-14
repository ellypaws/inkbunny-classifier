package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"

	"classifier/pkg/classify"
	"classifier/pkg/server"
	"classifier/pkg/tui"
	"classifier/pkg/tui/components/logger"
	"classifier/pkg/utils"
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

	loggers := logger.NewStack("classifier")
	classLogger := loggers.Get("classifier")

	writers, closer, err := utils.NewLogWriters(classLogger)
	if err != nil {
		log.Fatalf("error creating log writers: %v", err)
	}
	defer closer()
	log.SetOutput(writers[0])
	log.Default().SetLevel(log.DebugLevel)
	log.SetColorProfile(termenv.TrueColor)

	model := tui.NewModel(loggers, nil)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	go p.Run()
	done := make(chan os.Signal, 1)
	const port = 8080
	log.Infof("Server starting on http://localhost:%d", port)
	go func() { log.Print(http.ListenAndServe(fmt.Sprintf(":%d", port), nil)); close(done) }()

	model.Shutdown()
	wait(done)
}

func wait(done chan os.Signal) {
	signal.Notify(done, os.Interrupt)
	<-done
}
