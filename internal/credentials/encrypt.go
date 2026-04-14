// Package credentials provides credential encryption using AES-256-GCM
package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptionConfig holds encryption parameters
const (
	// PBKDF2Iterations is the number of iterations for PBKDF2 (OWASP 2024 recommendation)
	PBKDF2Iterations = 600000
	// KeyLength is the length of the derived key in bytes (256 bits)
	KeyLength = 32
	// SaltLength is the length of the random salt in bytes
	SaltLength = 16
	// NonceLength is the length of the GCM nonce in bytes
	NonceLength = 12
)

// EncryptedData represents encrypted data with salt and nonce
type EncryptedData struct {
	Ciphertext string `json:"ciphertext"` // base64 encoded
	Salt       string `json:"salt"`       // base64 encoded
	Nonce      string `json:"nonce"`      // base64 encoded
}

// DeriveKey derives a 256-bit key from password using PBKDF2
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeyLength, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM with PBKDF2 key derivation
func Encrypt(plaintext []byte, password string) (*EncryptedData, error) {
	// Generate random salt
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("ECRED008: failed to generate salt: %w", err)
	}

	// Derive key from password
	key := DeriveKey(password, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("ECRED009: failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("ECRED010: failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("ECRED011: failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptedData{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext[NonceLength:]),
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
	}, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with PBKDF2 key derivation
func Decrypt(data *EncryptedData, password string) ([]byte, error) {
	// Decode base64
	salt, err := base64.StdEncoding.DecodeString(data.Salt)
	if err != nil {
		return nil, fmt.Errorf("ECRED012: failed to decode salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(data.Nonce)
	if err != nil {
		return nil, fmt.Errorf("ECRED013: failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(data.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("ECRED014: failed to decode ciphertext: %w", err)
	}

	// Derive key from password
	key := DeriveKey(password, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("ECRED015: failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("ECRED016: failed to create GCM: %w", err)
	}

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("ECRED017: decryption failed (wrong password or corrupted data): %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded result
func EncryptString(plaintext, password string) (*EncryptedData, error) {
	return Encrypt([]byte(plaintext), password)
}

// DecryptString decrypts to a string
func DecryptString(data *EncryptedData, password string) (string, error) {
	plaintext, err := Decrypt(data, password)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
