package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

const key = `secret`

var aesKey = deriveKey(key)

func init() {
	if key := os.Getenv("AES_KEY"); key != "" {
		aesKey = deriveKey(key)
	}
}

// deriveKey creates a 32-byte key from the input string using SHA-256
func deriveKey(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
}

type Crypto struct {
	key   string
	block cipher.Block
	gcm   cipher.AEAD
}

func NewCrypto(key string) (*Crypto, error) {
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &Crypto{key, block, gcm}, nil
}

func (c *Crypto) Encrypt(b []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to read nonce: %w", err)
	}

	return c.gcm.Seal(nonce, nonce, b, nil), nil
}

func (c *Crypto) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("failed to decrypt cipher: ciphertext too short")
	}
	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := c.gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt cipher: %w", err)
	}

	return plaintext, nil
}
