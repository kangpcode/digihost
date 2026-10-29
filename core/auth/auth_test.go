// DigiPanel Auth Engine Unit Tests (core/auth/auth_test.go)
// Verifies Argon2id, AES-256-GCM, and TOTP RFC 6238 verification.

package auth

import (
	"bytes"
	"testing"
	"time"
)

func TestArgon2idHashingAndVerification(t *testing.T) {
	password := "CorrectHorseBatteryStaple2026!"

	encodedHash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password with Argon2id: %v", err)
	}

	// Verify correct password
	match, err := VerifyPassword(password, encodedHash)
	if err != nil {
		t.Fatalf("Verification returned error: %v", err)
	}
	if !match {
		t.Errorf("Expected password to verify successfully")
	}

	// Verify incorrect password
	wrongMatch, err := VerifyPassword("WrongPassword123!", encodedHash)
	if err != nil {
		t.Fatalf("Verification of wrong password returned error: %v", err)
	}
	if wrongMatch {
		t.Errorf("Security failure: wrong password matched hash")
	}
}

func TestAesGcmSecretsEncryption(t *testing.T) {
	masterKey := []byte("01234567890123456789012345678901") // 32-byte AES-256 key
	secretData := []byte("JBSWY3DPEHPK3PXP")                // Sample TOTP Base32 Secret

	ciphertext, err := EncryptSecret(masterKey, secretData)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if bytes.Equal(ciphertext, secretData) {
		t.Fatalf("Ciphertext equals plaintext")
	}

	decrypted, err := DecryptSecret(masterKey, ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(decrypted, secretData) {
		t.Errorf("Decrypted data '%s' does not match original '%s'", decrypted, secretData)
	}
}

func TestTotpGenerationAndVerification(t *testing.T) {
	secretBase32 := "JBSWY3DPEHPK3PXP"
	now := time.Now()

	code, err := GenerateTOTPCode(secretBase32, now)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}

	if len(code) != 6 {
		t.Fatalf("Expected 6-digit TOTP code, got '%s'", code)
	}

	// Verify code at current time
	if !VerifyTOTP(secretBase32, code, now) {
		t.Errorf("Expected current TOTP code '%s' to verify", code)
	}

	// Verify code at +/- 25 seconds (within 30s window)
	if !VerifyTOTP(secretBase32, code, now.Add(25*time.Second)) {
		t.Errorf("Expected TOTP code to verify within skew window")
	}

	// Verify rejection of incorrect code
	if VerifyTOTP(secretBase32, "000000", now) && code != "000000" {
		t.Errorf("Security failure: arbitrary code verified")
	}
}
