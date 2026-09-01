package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var defaultSecretKey = []byte("hephaestus-super-secure-key-32b!") // 32 bytes fallback key

// SetSecretKey sets the active AES encryption key
func SetSecretKey(keyStr string) {
	if len(keyStr) > 0 {
		hash := sha256.Sum256([]byte(keyStr))
		defaultSecretKey = hash[:]
	}
}

// EncryptText encrypts plaintext using AES-256-GCM and returns "iv:authTag:ciphertext" in hex.
// Matches the format used by the previous Node.js system for seamless migration.
func EncryptText(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(defaultSecretKey)
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

	// GCM Seal appends the ciphertext + 16-byte auth tag
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	tagSize := 16
	if len(sealed) < tagSize {
		return "", errors.New("sealed ciphertext too short")
	}

	ciphertext := sealed[:len(sealed)-tagSize]
	authTag := sealed[len(sealed)-tagSize:]

	return fmt.Sprintf("%s:%s:%s",
		hex.EncodeToString(nonce),
		hex.EncodeToString(authTag),
		hex.EncodeToString(ciphertext),
	), nil
}

// DecryptText decrypts "iv:authTag:ciphertext" format hex string using AES-256-GCM.
func DecryptText(encryptedStr string) (string, error) {
	if encryptedStr == "" {
		return "", nil
	}

	parts := strings.Split(encryptedStr, ":")
	if len(parts) != 3 {
		// Fallback: if not in encrypted format, return as-is
		return encryptedStr, nil
	}

	nonce, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid nonce hex: %w", err)
	}

	authTag, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid authTag hex: %w", err)
	}

	ciphertext, err := hex.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext hex: %w", err)
	}

	block, err := aes.NewCipher(defaultSecretKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Reconstruct sealed payload for GCM Open (ciphertext + authTag)
	sealed := append(ciphertext, authTag...)
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decryption authentication failed: %w", err)
	}

	return string(plaintext), nil
}

// HashPassword hashes a plain password using bcrypt (12 rounds)
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPasswordHash verifies a plain password against a bcrypt hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HashToken generates a SHA-256 hex string from an input token
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
