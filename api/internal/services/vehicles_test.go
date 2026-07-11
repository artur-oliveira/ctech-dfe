package services

import (
	"testing"
)

// ─── ValidatePlate ────────────────────────────────────────────────────────────

func TestValidatePlate_LegacyFormat(t *testing.T) {
	if err := ValidatePlate("ABC1234"); err != nil {
		t.Errorf("legacy plate ABC1234: unexpected error: %v", err)
	}
}

func TestValidatePlate_MercosulFormat(t *testing.T) {
	if err := ValidatePlate("ABC1D23"); err != nil {
		t.Errorf("Mercosul plate ABC1D23: unexpected error: %v", err)
	}
}

func TestValidatePlate_LowercaseNormalized(t *testing.T) {
	if err := ValidatePlate("abc1d23"); err != nil {
		t.Errorf("lowercase plate abc1d23: unexpected error: %v", err)
	}
}

func TestValidatePlate_TooShortReturnsError(t *testing.T) {
	if err := ValidatePlate("AB1234"); err == nil {
		t.Error("short plate: expected error, got nil")
	}
}

func TestValidatePlate_TooLongReturnsError(t *testing.T) {
	if err := ValidatePlate("ABCD1234"); err == nil {
		t.Error("long plate: expected error, got nil")
	}
}

func TestValidatePlate_AllDigitsReturnsError(t *testing.T) {
	if err := ValidatePlate("1234567"); err == nil {
		t.Error("all digits: expected error, got nil")
	}
}

func TestValidatePlate_EmptyReturnsError(t *testing.T) {
	if err := ValidatePlate(""); err == nil {
		t.Error("empty plate: expected error, got nil")
	}
}

func TestValidatePlate_SpecialCharsReturnsError(t *testing.T) {
	if err := ValidatePlate("ABC-1234"); err == nil {
		t.Error("plate with hyphen: expected error, got nil")
	}
}

// ─── validateRenavam ──────────────────────────────────────────────────────────

func TestValidateRenavam_9Digits(t *testing.T) {
	if err := validateRenavam("123456789"); err != nil {
		t.Errorf("9-digit renavam: unexpected error: %v", err)
	}
}

func TestValidateRenavam_11Digits(t *testing.T) {
	if err := validateRenavam("12345678901"); err != nil {
		t.Errorf("11-digit renavam: unexpected error: %v", err)
	}
}

func TestValidateRenavam_10Digits(t *testing.T) {
	if err := validateRenavam("1234567890"); err != nil {
		t.Errorf("10-digit renavam: unexpected error: %v", err)
	}
}

func TestValidateRenavam_TooShortReturnsError(t *testing.T) {
	if err := validateRenavam("12345678"); err == nil {
		t.Error("8-digit renavam: expected error, got nil")
	}
}

func TestValidateRenavam_TooLongReturnsError(t *testing.T) {
	if err := validateRenavam("123456789012"); err == nil {
		t.Error("12-digit renavam: expected error, got nil")
	}
}

func TestValidateRenavam_NonDigitsReturnsError(t *testing.T) {
	if err := validateRenavam("12345678A"); err == nil {
		t.Error("renavam with letter: expected error, got nil")
	}
}

func TestValidateRenavam_EmptyReturnsError(t *testing.T) {
	// validateRenavam rejects empty — the service skips the call when field is absent.
	if err := validateRenavam(""); err == nil {
		t.Error("empty string: expected error (caller must skip when absent)")
	}
}
