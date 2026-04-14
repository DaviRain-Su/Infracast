package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeriveKey validates PBKDF2 key derivation
func TestDeriveKey(t *testing.T) {
	password := "test-password"
	salt := []byte("test-salt-123456")

	key := DeriveKey(password, salt)

	// Should produce 32-byte (256-bit) key
	assert.Len(t, key, KeyLength)

	// Same password + salt should produce same key
	key2 := DeriveKey(password, salt)
	assert.Equal(t, key, key2)

	// Different password should produce different key
	key3 := DeriveKey("different-password", salt)
	assert.NotEqual(t, key, key3)

	// Different salt should produce different key
	differentSalt := []byte("different-salt!!")
	key4 := DeriveKey(password, differentSalt)
	assert.NotEqual(t, key, key4)
}

// TestEncryptDecrypt validates AES-256-GCM encryption and decryption
func TestEncryptDecrypt(t *testing.T) {
	plaintext := []byte("sensitive-credential-data")
	password := "strong-password-123"

	// Encrypt
	encrypted, err := Encrypt(plaintext, password)
	require.NoError(t, err)
	assert.NotNil(t, encrypted)
	assert.NotEmpty(t, encrypted.Ciphertext)
	assert.NotEmpty(t, encrypted.Salt)
	assert.NotEmpty(t, encrypted.Nonce)

	// Decrypt
	decrypted, err := Decrypt(encrypted, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestEncryptDecryptString validates string encryption
func TestEncryptDecryptString(t *testing.T) {
	plaintext := "my-secret-access-key"
	password := "encryption-password"

	// Encrypt
	encrypted, err := EncryptString(plaintext, password)
	require.NoError(t, err)
	assert.NotNil(t, encrypted)

	// Decrypt
	decrypted, err := DecryptString(encrypted, password)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestDecryptWrongPassword validates decryption fails with wrong password
func TestDecryptWrongPassword(t *testing.T) {
	plaintext := []byte("secret-data")
	password := "correct-password"
	wrongPassword := "wrong-password"

	encrypted, err := Encrypt(plaintext, password)
	require.NoError(t, err)

	// Decrypt with wrong password should fail
	_, err = Decrypt(encrypted, wrongPassword)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ECRED017")
}

// TestEncryptUniqueCiphertext validates each encryption produces unique ciphertext
func TestEncryptUniqueCiphertext(t *testing.T) {
	plaintext := []byte("same-data")
	password := "same-password"

	encrypted1, err := Encrypt(plaintext, password)
	require.NoError(t, err)

	encrypted2, err := Encrypt(plaintext, password)
	require.NoError(t, err)

	// Ciphertexts should be different due to random salt/nonce
	assert.NotEqual(t, encrypted1.Ciphertext, encrypted2.Ciphertext)
	assert.NotEqual(t, encrypted1.Salt, encrypted2.Salt)
	assert.NotEqual(t, encrypted1.Nonce, encrypted2.Nonce)

	// But both should decrypt to same plaintext
	decrypted1, err := Decrypt(encrypted1, password)
	require.NoError(t, err)

	decrypted2, err := Decrypt(encrypted2, password)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

// TestEncryptEmptyPlaintext validates empty plaintext handling
func TestEncryptEmptyPlaintext(t *testing.T) {
	plaintext := []byte("")
	password := "password"

	encrypted, err := Encrypt(plaintext, password)
	require.NoError(t, err)

	decrypted, err := Decrypt(encrypted, password)
	require.NoError(t, err)
	// Empty plaintext decrypts to nil or empty slice - both are valid
	assert.True(t, len(decrypted) == 0)
}

// TestPBKDF2Iterations validates OWASP 2024 recommended iteration count
func TestPBKDF2Iterations(t *testing.T) {
	// Verify we're using OWASP 2024 recommended value
	assert.Equal(t, 600000, PBKDF2Iterations)
}

// TestKeyLength validates AES-256 key size
func TestKeyLength(t *testing.T) {
	// AES-256 requires 32-byte key
	assert.Equal(t, 32, KeyLength)
}
