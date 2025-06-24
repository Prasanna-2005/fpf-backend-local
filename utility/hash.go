package utility

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

var (
	ErrInvalidSecretKey = errors.New("secret key must be at least 32 characters")
	ErrInvalidHash      = errors.New("invalid hash format")
	ErrEmptyPassword    = errors.New("password cannot be empty")
)

// HashPassword encrypts password using AES-GCM
func HashPassword(password, secretKey string) (string, error) {
	Debug("Starting password hashing process")

	if password == "" {
		Error("Password hashing failed: empty password provided")
		return "", ErrEmptyPassword
	}

	if len(secretKey) < 32 {
		Errorf("Password hashing failed: secret key length is %d, minimum required is 32", len(secretKey))
		return "", ErrInvalidSecretKey
	}

	Debug("Creating SHA256 hash from secret key")
	keyHash := sha256.Sum256([]byte(secretKey))

	Debug("Creating AES cipher block")
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		Errorf("Failed to create AES cipher: %v", err)
		return "", err
	}

	Debug("Creating GCM mode cipher")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		Errorf("Failed to create GCM cipher: %v", err)
		return "", err
	}

	Debug("Generating random nonce")
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		Errorf("Failed to generate random nonce: %v", err)
		return "", err
	}

	Debug("Encrypting password with AES-GCM")
	ciphertext := gcm.Seal(nonce, nonce, []byte(password), nil)

	Debug("Encoding encrypted data to base64")
	result := base64.StdEncoding.EncodeToString(ciphertext)

	Info("Password successfully hashed and encrypted")
	return result, nil
}

// CheckPasswordHash verifies password against encrypted hash
