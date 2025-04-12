package server

import (
	"encoding/json"
	"iter"
	"net/http"

	"github.com/charmbracelet/log"
)

func Handle(w http.ResponseWriter, r *http.Request, worker iter.Seq[Result]) {
	enc := json.NewEncoder(w)
	if flusher, ok := w.(http.Flusher); ok {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for res := range worker {
			select {
			case <-r.Context().Done():
				break // interrupt detected
			default:
				if _, err := w.Write([]byte("data: ")); err != nil {
					log.Error("error writing data:", "err", err)
					return
				}
				if err := enc.Encode(res); err != nil {
					log.Error("error writing data:", "err", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if _, err := w.Write([]byte("\n")); err != nil {
					log.Error("error writing data:", "err", err)
					return
				}
				flusher.Flush()
			}
		}
		if _, err := w.Write([]byte("event: exit\ndata: exit\n\n")); err != nil {
			log.Error("error sending exit event", "err", err)
		}
	} else {
		var allResults []Result
		for res := range worker {
			select {
			case <-r.Context().Done():
				break
			default:
				allResults = append(allResults, res)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := enc.Encode(allResults); err != nil {
			log.Error("error writing data:", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
