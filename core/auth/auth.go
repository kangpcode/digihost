// DigiPanel Authentication Engine (core/auth/auth.go)
// Standards: Argon2id (m=65536, t=3, p=4), AES-256-GCM Secrets Encryption,
// and RFC 6238 TOTP Two-Factor Authentication.
// Zero emojis, strictly production grade.

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	ArgonTime    = 3
	ArgonMemory  = 64 * 1024 // 64 MB
	ArgonThreads = 4
	ArgonKeyLen  = 32
	SaltLength   = 16
)

var (
	ErrInvalidHash         = errors.New("invalid Argon2id hash format")
	ErrIncompatibleVersion = errors.New("incompatible Argon2 version")
	ErrCiphertextTooShort  = errors.New("ciphertext too short for AES-GCM")
	ErrInvalidTOTP         = errors.New("invalid TOTP authentication code")
)

// HashPassword generates an Argon2id hash with a random salt
func HashPassword(password string) (string, error) {
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate random salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	format := "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s"
	return fmt.Sprintf(format, argon2.Version, ArgonMemory, ArgonTime, ArgonThreads, b64Salt, b64Hash), nil
}

// VerifyPassword verifies a plaintext password against an Argon2id hash in constant time
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, ErrInvalidHash
	}

	if parts[1] != "argon2id" {
		return false, errors.New("incompatible hash algorithm: must be argon2id")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	computedHash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(expectedHash)))

	if subtle.ConstantTimeCompare(expectedHash, computedHash) == 1 {
		return true, nil
	}
	return false, nil
}

// EncryptSecret encrypts plaintext using AES-256-GCM with a random nonce
func EncryptSecret(masterKey []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptSecret decrypts ciphertext using AES-256-GCM
func DecryptSecret(masterKey []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, encryptedData, nil)
}

// GenerateTOTPCode generates an RFC 6238 6-digit TOTP code for a given timestamp
func GenerateTOTPCode(secretBase32 string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secretBase32)))
	if err != nil {
		return "", fmt.Errorf("invalid base32 secret: %w", err)
	}

	interval := uint64(t.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, interval)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0f
	code := (binary.BigEndian.Uint32(h[offset:offset+4]) & 0x7fffffff) % 1000000

	return fmt.Sprintf("%06d", code), nil
}

// VerifyTOTP verifies a TOTP code within a +/- 1 step window (skew of 30 seconds)
func VerifyTOTP(secretBase32 string, code string, t time.Time) bool {
	cleanCode := strings.TrimSpace(code)
	if len(cleanCode) != 6 {
		return false
	}

	// Check intervals: t-30s, t, t+30s to handle clock drift
	for _, offset := range []int64{-30, 0, 30} {
		stepTime := t.Add(time.Duration(offset) * time.Second)
		expectedCode, err := GenerateTOTPCode(secretBase32, stepTime)
		if err == nil && subtle.ConstantTimeCompare([]byte(expectedCode), []byte(cleanCode)) == 1 {
			return true
		}
	}
	return false
}
