package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

type SecretCipher struct {
	key []byte
}

func NewSecretCipher() *SecretCipher {
	raw := os.Getenv("USER_SECRET_ENCRYPTION_KEY")
	if raw == "" {
		return &SecretCipher{}
	}
	sum := sha256.Sum256([]byte(raw))
	return &SecretCipher{key: sum[:]}
}

func (c *SecretCipher) Enabled() bool {
	return c != nil && len(c.key) == 32
}

func (c *SecretCipher) EncryptString(plain string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("user secret encryption key is not configured")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	out := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (c *SecretCipher) DecryptString(enc string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("user secret encryption key is not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("invalid ciphertext")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
