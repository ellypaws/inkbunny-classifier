package lib

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
)

// deriveKey creates a 32-byte key from the input string using SHA-256.
func deriveKey(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
}

// Crypto holds the key and the AES block used for encryption/decryption.
// An empty Crypto instance means no encryption is applied.
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
	if c == nil {
		return ""
	}
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
	if c == nil || c.key == "" {
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

// cipherReader wraps an io.Reader and a cipher.Stream.
type cipherReader struct {
	cipher.Block
	cipher.StreamReader
	sync.Once
}

// Read decrypts data using the stream cipher and reads the resulting plaintext
// from the underlying reader. The first read will also read the IV of length aes.BlockSize.
func (cr *cipherReader) Read(p []byte) (n int, err error) {
	cr.Once.Do(func() {
		iv := make([]byte, aes.BlockSize)
		// Read the IV from the beginning of the stream.
		n, err = io.ReadFull(cr.StreamReader.R, iv)
		if err == nil {
			cr.StreamReader.S = cipher.NewCTR(cr.Block, iv)
		}
	})
	if err != nil {
		return
	}
	return cr.StreamReader.Read(p)
}

// Decoder wraps an io.Reader so that the data is decrypted on the fly.
// It expects the first aes.BlockSize bytes from the reader to be the IV.
func (c *Crypto) Decoder(r io.Reader) (io.Reader, error) {
	if c == nil || c.key == "" {
		return r, nil
	}
	return &cipherReader{Block: c.block, StreamReader: cipher.StreamReader{R: r}}, nil
}

// Encrypt returns an io.Reader that produces the encrypted version of data read
// from the provided plaintext reader. The IV is generated and served as the first
// aes.BlockSize bytes of the output.
func (c *Crypto) Encrypt(r io.Reader) (io.Reader, error) {
	if c == nil || c.key == "" {
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

	return &CryptoFile{decoder: decoder, file: file, encrypted: c != nil && c.key != ""}, nil
}

type CryptoFile struct {
	decoder   io.Reader
	file      *os.File
	once      sync.Once
	encrypted bool
}

func (c *CryptoFile) Read(p []byte) (n int, err error) {
	c.once.Do(func() {
		if !c.encrypted {
			return
		}
		var off int64
		off, err = c.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return
		}
		_, err = c.file.Seek(0, io.SeekStart)
		if err != nil {
			return
		}
		n, err = c.decoder.Read(nil)
		if err != nil {
			return
		}
		if off > aes.BlockSize {
			_, err = c.file.Seek(off, io.SeekStart)
		}
	})
	if err != nil {
		return
	}
	return c.decoder.Read(p)
}

func (c *CryptoFile) Seek(offset int64, whence int) (int64, error) {
	if !c.encrypted {
		return c.file.Seek(offset, whence)
	}
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

// OpenWithMethod returns an open function using a method such as Decoder or Encrypt, which implements io.ReadSeekCloser
func (c *Crypto) OpenWithMethod(method func(io.Reader) (io.Reader, error)) func(string) (*CryptoFile, error) {
	return func(path string) (*CryptoFile, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("error opening file: %w", err)
		}

		decoder, err := method(file)
		if err != nil {
			return nil, fmt.Errorf("error making decoder: %w", err)
		}

		return &CryptoFile{decoder: decoder, file: file, encrypted: c != nil && c.key != ""}, nil
	}
}
