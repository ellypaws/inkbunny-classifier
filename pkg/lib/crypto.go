package lib

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// deriveKey creates a 32-byte key from the input string using SHA-256.
func deriveKey(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
}

// Crypto holds the key and the AES block used for encryption/decryption.
type Crypto struct {
	key   string
	block cipher.Block
}

// NewCrypto initializes a new Crypto instance using the provided key.
func NewCrypto(key string) (*Crypto, error) {
	if key == "" {
		return &Crypto{key: key}, nil
	}
	derivedKey := deriveKey(key)
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, err
	}
	return &Crypto{
		key:   key,
		block: block,
	}, nil
}

// cipherWriter wraps an io.Writer and a cipher.Stream.
type cipherWriter struct {
	writer io.Writer
	stream cipher.Stream
}

// Write encrypts data using the stream cipher and writes the resulting ciphertext
// to the underlying writer.
func (cw *cipherWriter) Write(p []byte) (int, error) {
	// Copy p into a temporary slice so that the original data isn't modified.
	tmp := make([]byte, len(p))
	copy(tmp, p)
	// Encrypt in-place in the temporary slice.
	cw.stream.XORKeyStream(tmp, tmp)
	return cw.writer.Write(tmp)
}

// Encoder wraps an io.Writer into an encrypting writer. It writes a random IV
// (initialization vector) as a prefix to the output so that the Decoder can decrypt.
func (c *Crypto) Encoder(w io.Writer) (io.Writer, error) {
	if c.key == "" {
		return w, nil
	}
	iv := make([]byte, aes.BlockSize)
	// Generate a random IV.
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	// Write the IV to the underlying writer.
	if _, err := w.Write(iv); err != nil {
		return nil, err
	}
	// Create a CTR stream cipher.
	stream := cipher.NewCTR(c.block, iv)
	return &cipherWriter{
		writer: w,
		stream: stream,
	}, nil
}

// cipherReader wraps an io.Reader and a cipher.Stream.
type cipherReader struct {
	reader io.Reader
	stream cipher.Stream
}

// Read reads from the underlying reader, decrypts the data in-place, and
// returns the decrypted data.
func (cr *cipherReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	if n > 0 {
		cr.stream.XORKeyStream(p[:n], p[:n])
	}
	return n, err
}

// Decoder wraps an io.Reader so that the data is decrypted on the fly.
// It expects the first aes.BlockSize bytes from the reader to be the IV.
func (c *Crypto) Decoder(r io.Reader) (io.Reader, error) {
	if c.key == "" {
		return r, nil
	}
	iv := make([]byte, aes.BlockSize)
	// Read the IV from the beginning of the stream.
	if _, err := io.ReadFull(r, iv); err != nil {
		return nil, err
	}
	// Create a CTR stream cipher for decryption using the same IV.
	stream := cipher.NewCTR(c.block, iv)
	return &cipherReader{
		reader: r,
		stream: stream,
	}, nil
}

var client = http.Client{Timeout: 30 * time.Second}

func DownloadFile(ctx context.Context, path, folder string, crypto *Crypto) (io.ReadCloser, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	fileName := filepath.Join(folder, filepath.Base(u.Path))
	if FileExists(fileName) {
		return OpenFile(fileName, crypto)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}
	defer resp.Body.Close()

	err = os.MkdirAll(folder, 0755)
	if err != nil {
		return nil, fmt.Errorf("error creating folder: %w", err)
	}

	out, err := os.Create(fileName)
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

	return OpenFile(fileName, crypto)
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func OpenFile(path string, crypto *Crypto) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	decoder, err := crypto.Decoder(file)
	if err != nil {
		return nil, fmt.Errorf("error making decoder: %w", err)
	}

	return &closer{decoder, file}, nil
}

type closer struct {
	decoder io.Reader
	closer  io.Closer
}

func (c *closer) Read(p []byte) (n int, err error) { return c.decoder.Read(p) }

func (c *closer) Close() error { return c.closer.Close() }
