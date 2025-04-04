package main

import (
	"fmt"
	"io"
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
		Username: "JeffyCottonbun",
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
			semaphore <- struct{}{}
			if err := downloadFile(sub.ThumbnailURLHugeNonCustom); err != nil {
				log.Printf("Error downloading submission: %v", err)
			}
			log.Printf("Downloaded submission: %v", sub.ThumbnailURLHugeNonCustom)
			<-semaphore
		}
	}
}

var client = http.Client{Timeout: 30 * time.Second}

func downloadFile(path string) error {
	u, err := url.Parse(path)
	if err != nil {
		return err
	}

	resp, err := client.Get(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fileName := filepath.Join("dataset/raw", filepath.Base(u.Path))
	err = os.MkdirAll("dataset/raw", 0755)
	if err != nil {
		return err
	}
	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
