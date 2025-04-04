package utils

import (
	"encoding/json"
	"io"
)

func Decode[T any](reader io.Reader) (T, error) {
	decoder := json.NewDecoder(reader)
	var t T
	return t, decoder.Decode(&t)
}

func DecodeAndClose[T any](reader io.ReadCloser) (T, error) {
	defer reader.Close()
	return Decode[T](reader)
}

func Encode[T any](w io.Writer, t T) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(t)
}

func EncodeIndent[T any](w io.Writer, t T, indent string) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", indent)
	return encoder.Encode(t)
}
