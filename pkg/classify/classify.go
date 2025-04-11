package classify

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"classifier/pkg/utils"
)

type Prediction = map[string]float64

var bodyPool = utils.NewPoolMake[*bytes.Buffer]()

var predictURL = "http://localhost:7860/predict"

func init() {
	predict := os.Getenv("PREDICT_URL")
	if predict == "" {
		return
	}
	if u, err := url.Parse(predict); err == nil {
		predictURL = u.String()
	}
}

func Predict(ctx context.Context, file io.Reader) (Prediction, error) {
	body := bodyPool.Get()
	body.Reset()
	defer bodyPool.Put(body)

	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "image")
	if err != nil {
		return nil, err
	}

	src, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, 640, 640))
	draw.NearestNeighbor.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)
	err = jpeg.Encode(part, dst, nil)
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

func PredictURL(ctx context.Context, path string) (Prediction, error) {
	params := url.Values{
		"url": {path},
	}
	requestURL := fmt.Sprintf("%s?%s", predictURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return utils.DecodeAndClose[Prediction](resp.Body)
}
