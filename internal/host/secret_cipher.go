package host

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const secretCipherKeySize = 32

type SecretCipher interface {
	Encrypt(plainText []byte) ([]byte, error)
	Decrypt(cipherText []byte) ([]byte, error)
}

type AESGCMSecretCipher struct {
	aead cipher.AEAD
}

func NewAESGCMSecretCipher(key []byte) (*AESGCMSecretCipher, error) {
	if len(key) != secretCipherKeySize {
		return nil, fmt.Errorf("invalid secret encryption key length: expected %d bytes", secretCipherKeySize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &AESGCMSecretCipher{aead: aead}, nil
}

func (c *AESGCMSecretCipher) Encrypt(plainText []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	encrypted := c.aead.Seal(nil, nonce, plainText, nil)
	result := make([]byte, 0, len(nonce)+len(encrypted))
	result = append(result, nonce...)
	result = append(result, encrypted...)
	return result, nil
}

func (c *AESGCMSecretCipher) Decrypt(cipherText []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext payload")
	}
	nonce := cipherText[:nonceSize]
	body := cipherText[nonceSize:]
	plainText, err := c.aead.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}
	return plainText, nil
}

var _ SecretCipher = (*AESGCMSecretCipher)(nil)
