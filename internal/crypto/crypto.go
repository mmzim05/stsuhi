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

// Sizes and costs for the connection cipher and the master password hash.
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

// Errors returned by Decrypt. ErrDecryptionFailed also covers a wrong
// password, since GCM can't tell the two apart.
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
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	key := DeriveKey(password, salt, PBKDF2Iter)

	block, err := aes.NewCipher(key)
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

	ciphertext := gcm.Seal(nil, nonce, data, nil)

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

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(data) < SaltSize+NonceSize {
		return nil, ErrInvalidCiphertext
	}

	salt := data[0:SaltSize]
	nonce := data[SaltSize : SaltSize+NonceSize]
	ciphertext := data[SaltSize+NonceSize:]

	key := DeriveKey(password, salt, iterations)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// NewRandomKey returns a random master password for machines with a
// keyring, where the user never types one. 32 random bytes match the AES
// key size, and the base64 text stays under bcrypt's 72-byte limit.
func NewRandomKey() (string, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("generate master key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(key), nil
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
