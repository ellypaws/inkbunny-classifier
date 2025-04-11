package utils

import (
	"io"
	"log"
	"os"
)

// NewLogWriters takes any number of io.Writer and returns a slice of wrapped writers.
// When you write to any of these writers, the data will also be appended to "log.txt".
// It also returns a cleanup function that you should call to close the log file.
func NewLogWriters(writers ...io.Writer) ([]io.Writer, func() error, error) {
	// Open (or create) the log file in append mode.
	f, err := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, nil, err
	}

	// Wrap each provided writer with our logWriter.
	wrapped := make([]io.Writer, len(writers))
	for i, w := range writers {
		wrapped[i] = io.MultiWriter(w, f)
	}

	return wrapped, f.Close, nil
}

func LogOutput(writer io.Writer) func() {
	f, _ := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	mw := io.MultiWriter(writer, f)
	r, w, _ := os.Pipe()

	os.Stdout = w
	os.Stderr = w

	log.SetOutput(mw)

	exit := make(chan bool)

	go func() {
		_, _ = io.Copy(mw, r)
		exit <- true
		close(exit)
	}()

	return func() {
		_ = w.Close()
		<-exit
		_ = f.Close()
	}
}
