package server

import (
	_ "embed"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"classifier/pkg/utils"
)

//go:embed index.html
var index []byte

// HomeHandler serves the main HTML page
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	const indexPath = "pkg/server/index.html"
	if fileExists(indexPath) {
		http.ServeFile(w, r, indexPath)
	} else {
		w.Write(index)
	}
}

func FileProxy(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.PathValue("path"), "http") {
		serveEncryptedFile(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Clean(r.PathValue("path")))
}

func serveEncryptedFile(w http.ResponseWriter, r *http.Request) {
	path, decryptKey := getImagePath(r.PathValue("path"))
	if path == "" {
		http.Error(w, "path not found", http.StatusNotFound)
		return
	}
	if decryptKey == "" {
		http.Error(w, "decrypt key not found", http.StatusNotFound)
		return
	}

	crypto, err := utils.NewCrypto(decryptKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	file, err := openFile(path, crypto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var inkbunnyRegexp = regexp.MustCompile(`(?:https?://)?((?:\w+\.)?i(?:nk)?b(?:unny)?(?:\.metapix)?.net)/(?:((?:private_)?thumbnails|usericons|files)/(medium|large|huge|full|preview))/((\d+)/(\d+)_([^_]+)_(.*?)(?:_noncustom)?\.[^\s?]+)\S*`)

func getArtist(url string) string {
	match := inkbunnyRegexp.FindStringSubmatch(url)
	if match == nil || len(match) < 7 {
		return ""
	}
	return inkbunnyRegexp.FindStringSubmatch(url)[7]
}

func getImagePath(path string) (string, string) {
	u, err := url.Parse(path)
	if err != nil {
		return "", ""
	}
	return filepath.Join("inkbunny", getArtist(u.String()), filepath.Base(u.Path)), u.Query().Get("decrypt_key")
}
