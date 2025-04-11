package lib

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"testing"
)

var crypto, _ = NewCrypto("secret")

//go:embed image.png
var file []byte

func TestCrypto_Encoder(t *testing.T) {
	encrypted, err := os.Create("encrypted.png")
	if err != nil {
		t.Fatalf("error creating encrypted file: %v", err)
	}
	encoder, err := crypto.Encoder(encrypted)
	if err != nil {
		t.Fatalf("encoder failed: %v", err)
	}

	source := bytes.NewReader(file)
	_, err = io.Copy(encoder, source)
	if err != nil {
		t.Fatalf("encoder failed: %v", err)
	}
}

func TestCrypto_Decoder(t *testing.T) {
	t.Run("encrypted file", TestCrypto_Encoder)
	encrypted, err := os.Open("encrypted.png")
	if err != nil {
		t.Fatalf("error opening encrypted file: %v", err)
	}
	defer encrypted.Close()

	decoder, err := crypto.Decoder(encrypted)
	if err != nil {
		t.Fatalf("decoder failed: %v", err)
	}

	decrypted, err := os.Create("decrypted.png")
	if err != nil {
		t.Fatalf("error creating encrypted file: %v", err)
	}
	defer decrypted.Close()
	_, err = io.Copy(decrypted, decoder)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
}
