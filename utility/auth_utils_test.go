package utility

import (
	"encoding/base32"
	"testing"
	"time"

	"form_pf_backend/config"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestGenTOTP_Success(t *testing.T) {
	cfg := &config.OTPconfig{
		SecretKey: "TESTSECRET123",
		OTPExpiry: 30 * time.Second,
		OTPLength: otp.DigitsSix,
	}

	code, err := GenTOTP(cfg, "user123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6 digits, got %d", len(code))
	}
	if !isNumeric(code) {
		t.Errorf("code should be numeric, got %s", code)
	}
}

func TestGenTOTP_EmptySecret(t *testing.T) {
	cfg := &config.OTPconfig{
		SecretKey: "",
		OTPExpiry: 30 * time.Second,
		OTPLength: otp.DigitsSix,
	}

	code, err := GenTOTP(cfg, "user123")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if code != "" {
		t.Errorf("expected empty code, got %s", code)
	}
}

func TestGenTOTP_DifferentUsers(t *testing.T) {
	cfg := &config.OTPconfig{
		SecretKey: "SHARED123",
		OTPExpiry: 30 * time.Second,
		OTPLength: otp.DigitsSix,
	}

	code1, _ := GenTOTP(cfg, "user1")
	code2, _ := GenTOTP(cfg, "user2")

	if code1 == code2 {
		t.Errorf("different users should get different codes")
	}
}

func TestGenTOTP_SameUserConsistency(t *testing.T) {
	cfg := &config.OTPconfig{
		SecretKey: "CONSISTENT123",
		OTPExpiry: 30 * time.Second,
		OTPLength: otp.DigitsSix,
	}

	code1, _ := GenTOTP(cfg, "user1")
	code2, _ := GenTOTP(cfg, "user1")

	if code1 != code2 {
		t.Errorf("same user should get same code within time window")
	}
}

func TestGenTOTP_ValidateGenerated(t *testing.T) {
	cfg := &config.OTPconfig{
		SecretKey: "VALIDATE123",
		OTPExpiry: 30 * time.Second,
		OTPLength: otp.DigitsSix,
	}
	userId := "testuser"

	code, err := GenTOTP(cfg, userId)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Verify with same secret
	secret := cfg.SecretKey + userId
	encodedSecret := base32.StdEncoding.EncodeToString([]byte(secret))

	if !totp.Validate(code, encodedSecret) {
		t.Errorf("generated code should be valid")
	}
}

func TestGenTOTP_InvalidLength(t *testing.T) {
	t.Run("zero length", func(t *testing.T) {
		cfg := &config.OTPconfig{
			SecretKey: "TEST123",
			OTPExpiry: 30 * time.Second,
			OTPLength: otp.Digits(0),
		}

		code, err := GenTOTP(cfg, "user")
		// Zero length actually generates a code (library uses default)
		// Just verify it's not empty and is numeric
		if code == "" {
			t.Errorf("expected non-empty code, got empty")
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !isNumeric(code) {
			t.Errorf("code should be numeric, got %s", code)
		}
	})

	t.Run("negative length", func(t *testing.T) {
		cfg := &config.OTPconfig{
			SecretKey: "TEST123",
			OTPExpiry: 30 * time.Second,
			OTPLength: otp.Digits(-1),
		}

		// This should panic due to divide by zero
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic with negative length")
			}
		}()

		GenTOTP(cfg, "user")
	})
}

func TestGenTOTP_NilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("should panic with nil config")
		}
	}()
	GenTOTP(nil, "user")
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
