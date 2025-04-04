package classify

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"jeffy/pkg/utils"
)

type Prediction map[string]float64

var bodyPool = utils.NewPoolMake[bytes.Buffer]()

const predictURL = "http://localhost:7860/predict"

func Predict(ctx context.Context, file io.ReadSeeker) (Prediction, error) {
	body := bodyPool.Get()
	body.Reset()

	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "image")
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, predictURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return utils.DecodeAndClose[Prediction](resp.Body)
}
