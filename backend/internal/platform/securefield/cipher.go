package securefield

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const formatVersion byte = 1

type Cipher struct{ aead cipher.AEAD }

func New(encodedKey string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode phone data key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("phone data key must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create phone cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create phone AEAD: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	result := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	result[0] = formatVersion
	result = append(result, nonce...)
	result = c.aead.Seal(result, nonce, []byte(plaintext), nil)
	return result, nil
}

func (c *Cipher) Decrypt(ciphertext []byte) (string, error) {
	if len(ciphertext) < 1+c.aead.NonceSize()+c.aead.Overhead() || ciphertext[0] != formatVersion {
		return "", errors.New("invalid encrypted phone format")
	}
	nonceEnd := 1 + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, ciphertext[1:nonceEnd], ciphertext[nonceEnd:], nil)
	if err != nil {
		return "", errors.New("decrypt phone data")
	}
	return string(plaintext), nil
}
