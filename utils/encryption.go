package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	encryptionKeyEnv = "ENCRYPTION_KEY"
)

var (
	encryptionKey     []byte
	encryptionKeyOnce sync.Once
	loadKeyErr        error
)

func loadEncryptionKey() {
	encryptionKeyOnce.Do(func() {
		key := os.Getenv(encryptionKeyEnv)
		if key == "" {
			loadKeyErr = fmt.Errorf("ENCRYPTION_KEY environment variable is not set")
			return
		}
		if len(key) != 32 {
			loadKeyErr = fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes (256 bits)")
			return
		}
		encryptionKey = []byte(key)
	})
}

func Encrypt(plaintext string) (string, error) {
	loadEncryptionKey()
	if loadKeyErr != nil {
		return "", loadKeyErr
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func Decrypt(encryptedText string) (string, error) {
	loadEncryptionKey()
	if loadKeyErr != nil {
		return "", loadKeyErr
	}
	ciphertext, err := hex.DecodeString(encryptedText)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %w", err)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

func SetEncryptionKey(key string) error {
	if len(key) != 32 {
		return fmt.Errorf("encryption key must be exactly 32 bytes (256 bits)")
	}
	encryptionKey = []byte(key)
	return nil
}

func GetEncryptionKey() []byte {
	return encryptionKey
}
