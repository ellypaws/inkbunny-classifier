package lib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
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
