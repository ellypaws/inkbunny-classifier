package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ellypaws/inkbunny/api"
)

var sid = os.Getenv("SID")

func main() {
	user := api.Credentials{Sid: sid}
	request := api.SubmissionSearchRequest{
		SID:      sid,
		GetRID:   true,
		Text:     os.Getenv("TEXT"),
		Keywords: true,
		OrderBy:  "favs",
	}
	response, err := user.SearchSubmissions(request)
	if err != nil {
		fmt.Println(err)
	}

	semaphore := make(chan struct{}, 50)
	for subs, err := range response.AllSubmissions() {
		if err != nil {
			log.Printf("Error getting submissions: %v", err)
			continue
		}
		for _, sub := range subs {
			if sub.RatingID < 1 {
				log.Printf("Skipping submission: %v, rating: %d", sub, sub.RatingID)
				continue
			}

			semaphore <- struct{}{}
			err = downloadFile(sub.FileURLFull, "cub")
			<-semaphore

			if err != nil {
				log.Printf("Error downloading submission: %v", err)
				continue
			}
			log.Printf("Downloaded submission: %v", sub.FileURLFull)
		}
	}
}

var client = http.Client{Timeout: 30 * time.Second}

func downloadFile(path string, folder string) error {
	u, err := url.Parse(path)
	if err != nil {
		return err
	}

	resp, err := client.Get(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	folderName := filepath.Join("dataset", folder)
	fileName := filepath.Join(folderName, filepath.Base(u.Path))
	err = os.MkdirAll(folderName, 0755)
	if err != nil {
		return err
	}

	if fileExists(fileName) {
		return errors.New("file already exists")
	}

	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
