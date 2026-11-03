package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

const (
	KeySize    = 32 // AES-256
	SaltSize   = 32
	NonceSize  = 12 // GCM standard nonce size
	BcryptCost = 12

	// PBKDF2Iter is the PBKDF2-SHA256 cost for new ciphertexts (OWASP 2023).
	PBKDF2Iter = 600000
	// LegacyPBKDF2Iter is the cost used before versioned ciphertexts.
	// Unprefixed ciphertexts were written with it and still decrypt.
	LegacyPBKDF2Iter = 100000

	// versionPrefix marks a ciphertext derived with PBKDF2Iter. The base64
	// alphabet has no ':', so it can never collide with a legacy value.
	versionPrefix = "v2:"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrDecryptionFailed  = errors.New("decryption failed")
)

// DeriveKey derives an encryption key from a password and salt using PBKDF2
// with the given iteration count
func DeriveKey(password string, salt []byte, iterations int) []byte {
	return pbkdf2.Key([]byte(password), salt, iterations, KeySize, sha256.New)
}

// IsLegacy reports whether encoded was written with LegacyPBKDF2Iter and
// should be re-encrypted with Encrypt once the password is known
func IsLegacy(encoded string) bool {
	return !strings.HasPrefix(encoded, versionPrefix)
}

// Encrypt encrypts plaintext data using AES-256-GCM with the given password
// Returns versionPrefix + base64(salt + nonce + ciphertext + tag)
func Encrypt(data []byte, password string) (string, error) {
	// Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key
	key := DeriveKey(password, salt, PBKDF2Iter)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Combine salt + nonce + ciphertext
	result := make([]byte, SaltSize+len(nonce)+len(ciphertext))
	copy(result[0:], salt)
	copy(result[SaltSize:], nonce)
	copy(result[SaltSize+len(nonce):], ciphertext)

	return versionPrefix + base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt decrypts a ciphertext from Encrypt using the given password.
// Legacy unprefixed ciphertexts are decrypted with LegacyPBKDF2Iter.
func Decrypt(encoded string, password string) ([]byte, error) {
	iterations := LegacyPBKDF2Iter
	if body, ok := strings.CutPrefix(encoded, versionPrefix); ok {
		iterations = PBKDF2Iter
		encoded = body
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Validate minimum length
	if len(data) < SaltSize+NonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract salt and nonce
	salt := data[0:SaltSize]
	nonce := data[SaltSize : SaltSize+NonceSize]
	ciphertext := data[SaltSize+NonceSize:]

	// Derive encryption key
	key := DeriveKey(password, salt, iterations)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// HashPassword hashes a password using bcrypt for master password storage
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a bcrypt hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
