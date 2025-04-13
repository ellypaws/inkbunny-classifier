package lib

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
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

func (c *Crypto) Key() string {
	return c.key
}

// cipherWriter wraps an io.Writer and a cipher.Stream.
type cipherWriter struct {
	cipher.StreamWriter
	sync.Once
	iv []byte
}

// Write encrypts data using the stream cipher and writes the resulting ciphertext
// to the underlying writer. The first write will also write the IV of length aes.BlockSize.
func (cw *cipherWriter) Write(p []byte) (n int, err error) {
	cw.Once.Do(func() {
		n, err = cw.StreamWriter.W.Write(cw.iv)
	})
	if err != nil {
		return
	}
	return cw.StreamWriter.Write(p)
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
	// Create a CTR stream cipher.
	stream := cipher.NewCTR(c.block, iv)
	return &cipherWriter{StreamWriter: cipher.StreamWriter{S: stream, W: w}, iv: iv}, nil
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
	return cipher.StreamReader{S: stream, R: r}, nil
}

// Encrypt returns an io.Reader that produces the encrypted version of data read
// from the provided plaintext reader. The IV is generated and served as the first
// aes.BlockSize bytes of the output.
func (c *Crypto) Encrypt(r io.Reader) (io.Reader, error) {
	if c.key == "" {
		return r, nil
	}

	iv := make([]byte, aes.BlockSize)
	// Generate a random IV.
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	// Create a CTR stream cipher with the IV.
	stream := cipher.NewCTR(c.block, iv)

	// Return the encrypter that prepends the IV header and then encrypts the rest
	// of the data read from the underlying plaintext reader.
	return io.MultiReader(bytes.NewReader(iv), &cipher.StreamReader{S: stream, R: r}), nil
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// Open opens a file and returns a CryptoFile, which implements io.ReadSeekCloser
func (c *Crypto) Open(path string) (*CryptoFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	decoder, err := c.Decoder(file)
	if err != nil {
		return nil, fmt.Errorf("error making decoder: %w", err)
	}

	return &CryptoFile{decoder, file}, nil
}

type CryptoFile struct {
	decoder io.Reader
	file    *os.File
}

func (c *CryptoFile) Read(p []byte) (n int, err error) { return c.decoder.Read(p) }

func (c *CryptoFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		offset += aes.BlockSize
	case io.SeekCurrent:
		break
	case io.SeekEnd:
		offset -= aes.BlockSize
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	pos, err := c.file.Seek(offset, whence)
	if err != nil {
		return 0, err
	}

	if pos < aes.BlockSize {
		return 0, fmt.Errorf("invalid seek: resulting decrypted position (%d) is negative", pos)
	}

	return pos - aes.BlockSize, nil
}

func (c *CryptoFile) Close() error { return c.file.Close() }
