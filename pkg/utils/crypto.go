package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"classifier/pkg/lib"
)

// DownloadEncrypt downloads a file from the given URL and saves it to the specified folder.
// After saving the file, it immediately opens it using [lib.Crypto.Open].
// If the file already exists, it calls [lib.Crypto.Open].
func DownloadEncrypt(ctx context.Context, crypto *lib.Crypto, link, fileName string) (*lib.CryptoFile, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}

	if FileExists(fileName) {
		return crypto.Open(fileName)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}
	defer resp.Body.Close()

	ext := filepath.Ext(fileName)
	base := filepath.Base(fileName)
	name := base[:len(base)-len(ext)]
	out, err := os.CreateTemp(filepath.Dir(fileName), fmt.Sprintf("%s_*%s", name, ext))
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	encoder, err := crypto.Encoder(out)
	if err != nil {
		out.Close()
		return nil, fmt.Errorf("error creating encoder: %w", err)
	}

	_, err = io.Copy(encoder, resp.Body)
	if err != nil {
		out.Close()
		return nil, fmt.Errorf("error writing to file: %w", err)
	}

	out.Close()
	if err := os.Rename(out.Name(), fileName); err != nil {
		return nil, fmt.Errorf("error renaming file: %w", err)
	}
	return crypto.Open(fileName)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}
