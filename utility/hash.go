package utility

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidSecretKey = errors.New("secret key must be at least 32 characters")
	ErrInvalidHash      = errors.New("invalid hash format")
	ErrEmptyPassword    = errors.New("password cannot be empty")

	// Global logger instance
	logger = LoggerFunc()
)

// HashPassword encrypts password using AES-GCM
func HashPassword(password, secretKey string) (string, error) {
	logger.Log("DEBUG", "Starting password hashing process")

	if password == "" {
		logger.Log("ERROR", "Password hashing failed: empty password provided")
		return "", ErrEmptyPassword
	}

	if len(secretKey) < 32 {
		logger.Log("ERROR", fmt.Sprintf("Password hashing failed: secret key length is %d, minimum required is 32", len(secretKey)))
		return "", ErrInvalidSecretKey
	}

	logger.Log("DEBUG", "Creating SHA256 hash from secret key")
	keyHash := sha256.Sum256([]byte(secretKey))

	logger.Log("DEBUG", "Creating AES cipher block")
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create AES cipher: %v", err))
		return "", err
	}

	logger.Log("DEBUG", "Creating GCM mode cipher")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create GCM cipher: %v", err))
		return "", err
	}

	logger.Log("DEBUG", "Generating random nonce")
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to generate random nonce: %v", err))
		return "", err
	}

	logger.Log("DEBUG", "Encrypting password with AES-GCM")
	ciphertext := gcm.Seal(nonce, nonce, []byte(password), nil)

	logger.Log("DEBUG", "Encoding encrypted data to base64")
	result := base64.StdEncoding.EncodeToString(ciphertext)

	logger.Log("INFO", "Password successfully hashed and encrypted")
	return result, nil
}

// CheckPasswordHash verifies password against encrypted hash
func CheckPasswordHash(password, hashedPassword, secretKey string) bool {
	logger.Log("DEBUG", "Starting password hash verification")

	if password == "" || hashedPassword == "" || len(secretKey) < 32 {
		logger.Log("WARNING", "Password verification failed: invalid input parameters")
		return false
	}

	logger.Log("DEBUG", "Decoding base64 hashed password")
	data, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to decode base64 hash: %v", err))
		return false
	}

	logger.Log("DEBUG", "Creating SHA256 hash from secret key")
	keyHash := sha256.Sum256([]byte(secretKey))

	logger.Log("DEBUG", "Creating AES cipher block")
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create AES cipher during verification: %v", err))
		return false
	}

	logger.Log("DEBUG", "Creating GCM mode cipher")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create GCM cipher during verification: %v", err))
		return false
	}

	logger.Log("DEBUG", "Extracting nonce and ciphertext from encrypted data")
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		logger.Log("WARNING", "Invalid hash format: data too short for nonce extraction")
		return false
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	logger.Log("DEBUG", "Decrypting password")
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to decrypt password: %v", err))
		return false
	}

	logger.Log("DEBUG", "Comparing decrypted password with provided password")
	isValid := string(plaintext) == password

	if isValid {
		logger.Log("INFO", "Password verification successful")
	} else {
		logger.Log("WARNING", "Password verification failed: passwords do not match")
	}

	return isValid
}
