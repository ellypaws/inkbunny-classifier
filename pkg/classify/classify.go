package classify

import (
	"bytes"
	"context"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"iter"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"

	_ "golang.org/x/image/webp"

	"classifier/pkg/utils"
)

type Prediction map[string]float64

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

// Sorted returns a sorted map of the predictions in descending order.
func (p Prediction) Sorted() iter.Seq2[string, float64] {
	s := utils.MapToSlice(p)
	utils.SortMapByValue(s)
	return s.Backward()
}

// Max returns the key and value of the prediction with the highest confidence.
func (p Prediction) Max() (string, float64) {
	if len(p) == 0 {
		return "", -1
	}
	for k, v := range p.Sorted() {
		return k, v
	}
	return "", -1
}

// Filter returns the modified prediction map with only the predictions that have a confidence greater than or equal to min.
func (p Prediction) Filter(filters ...func(string, float64) bool) Prediction {
	for _, filter := range filters {
		if filter == nil {
			continue
		}
		maps.DeleteFunc(p, filter)
	}
	return p
}

// Minimum returns the modified prediction map with only the predictions that have a confidence greater than or equal to min.
func (p Prediction) Minimum(min float64) Prediction {
	return p.Filter(Minimum[string](min))
}

// Whitelist returns the modified prediction map with only the predictions that are in the whitelist.
func (p Prediction) Whitelist(allow ...string) Prediction {
	return p.Filter(Whitelist[float64](allow...))
}

// Minimum returns a function that can be used to filter predictions based on a minimum confidence level.
func Minimum[K comparable](min float64) func(K, float64) bool {
	return func(_ K, confidence float64) bool { return confidence < min }
}

// Whitelist returns a function that can be used to filter predictions based on a whitelist of allowed keys.
func Whitelist[V any](allow ...string) func(string, V) bool {
	return func(key string, _ V) bool {
		return !slices.Contains(allow, key)
	}
}

// Predict expects file to already be encrypted if needed, such as [classifier/pkg/lib.Crypto.Encrypt].
// As such, it will not call these methods for you, and it is up to the caller to call them.
func Predict(ctx context.Context, name, key string, file io.Reader) (Prediction, error) {
	body := bodyPool.Get()
	body.Reset()
	defer bodyPool.Put(body)

	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", name)
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

	predictURL := predictURL
	if key != "" {
		predictURL = fmt.Sprintf("%s?key=%s", predictURL, key)
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
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(body)
		return nil, fmt.Errorf("error in predicting %s: %s", name, string(body))
	}

	return utils.DecodeAndClose[Prediction](resp.Body)
}

func PredictURL(ctx context.Context, path string) (Prediction, error) {
	params := url.Values{"url": {path}}
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
