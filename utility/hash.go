/*
Package utility provides AES-GCM password encryption and verification.

This package implements secure password hashing using AES-GCM encryption
with SHA256-derived keys for authentication systems.
*/
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
)

func HashPassword(password, secretKey string) (string, error) {
	/*
		HashPassword encrypts a password using AES-GCM encryption.

		This function validates input parameters, creates a SHA256 hash from the secret key,
		and encrypts the password using AES-GCM with a random nonce.

		:param password: The plaintext password to encrypt
		:type password: string
		:param secretKey: Secret key for encryption (minimum 32 characters)
		:type secretKey: string

		:returns: Base64-encoded encrypted password and error if encryption fails
		:rtype: (string, error)
	*/

	if password == "" {
		logger.Log("ERROR", "Password hashing failed: empty password provided")
		return "", ErrEmptyPassword
	}

	if len(secretKey) < 32 {
		logger.Log("ERROR", fmt.Sprintf("Password hashing failed: secret key length is %d, minimum required is 32", len(secretKey)))
		return "", ErrInvalidSecretKey
	}

	keyHash := sha256.Sum256([]byte(secretKey))

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create AES cipher: %v", err))
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create GCM cipher: %v", err))
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to generate random nonce: %v", err))
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(password), nil)

	result := base64.StdEncoding.EncodeToString(ciphertext)

	logger.Log("INFO", "Password successfully hashed and encrypted")

	return result, nil
}

func CheckPasswordHash(hashedPassword, secretKey string) (string, error) {
	/*
		CheckPasswordHash verifies a password against its encrypted hash.

		This function decodes the base64 hash, extracts the nonce and ciphertext,
		decrypts the password using AES-GCM, and compares with the provided password.

		:param password: The plaintext password to verify
		:type password: string
		:param hashedPassword: The base64-encoded encrypted password hash
		:type hashedPassword: string
		:param secretKey: Secret key used for encryption (minimum 32 characters)
		:type secretKey: string

		:returns: Boolean indicating if password matches hash
		:rtype: bool
	*/
	logger.Log("INFO", "Starting password hash verification")

	if hashedPassword == "" || len(secretKey) < 32 {
		logger.Log("WARNING", "Password verification failed: invalid input parameters")
		return "", fmt.Errorf("invalide input params")
	}

	logger.Log("INFO", "Decoding base64 hashed password")
	data, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to decode base64 hash: %v", err.Error()))
		return "", err
	}

	logger.Log("INFO", "Creating SHA256 hash from secret key")
	keyHash := sha256.Sum256([]byte(secretKey))

	logger.Log("INFO", "Creating AES cipher block")
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create AES cipher during verification: %v", err))
		return "", err
	}

	logger.Log("INFO", "Creating GCM mode cipher")
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create GCM cipher during verification: %v", err))
		return "", err
	}

	logger.Log("INFO", "Extracting nonce and ciphertext from encrypted data")
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		logger.Log("WARNING", "Invalid hash format: data too short for nonce extraction")
		return "", err
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	logger.Log("INFO", "Decrypting password")
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to decrypt password: %v", err))
		return "", err
	}
	fmt.Println(plaintext)
	logger.Log("INFO", "Comparing decrypted password with provided password")

	return string(plaintext), nil
}

func DecryptToken(encryptedToken, secretKey string) (string, error) {
	if len(secretKey) < 32 {
		return "", errors.New("secret key must be at least 32 bytes")
	}

	cipherData, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		logger.Log("ERROR", "decode error"+err.Error())
		return "", err
	}

	keyHash := sha256.Sum256([]byte(secretKey))
	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		logger.Log("ERROR", "Cipher error"+err.Error())
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		logger.Log("ERROR", "GCM error"+err.Error())
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(cipherData) < nonceSize {
		logger.Log("ERROR", fmt.Sprintf("Nonce error: %v", err))
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := cipherData[:nonceSize], cipherData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		logger.Log("ERROR", "decryption error"+err.Error())
		return "", err
	}

	return string(plaintext), nil
}
