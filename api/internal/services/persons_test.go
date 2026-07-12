package services

import (
	"testing"
)

// ─── BuildPersonSK ────────────────────────────────────────────────────────────

func TestBuildPersonSK_CPF11Digits(t *testing.T) {
	got, err := BuildPersonSK("52998224725")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CPF_52998224725" {
		t.Errorf("got %q, want CPF_52998224725", got)
	}
}

func TestBuildPersonSK_CNPJ14Digits(t *testing.T) {
	got, err := BuildPersonSK("11222333000181")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CNPJ_11222333000181" {
		t.Errorf("got %q, want CNPJ_11222333000181", got)
	}
}

func TestBuildPersonSK_FormattedCPF(t *testing.T) {
	got, err := BuildPersonSK("529.982.247-25")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CPF_52998224725" {
		t.Errorf("got %q, want CPF_52998224725", got)
	}
}

func TestBuildPersonSK_FormattedCNPJ(t *testing.T) {
	got, err := BuildPersonSK("11.222.333/0001-81")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "CNPJ_11222333000181" {
		t.Errorf("got %q, want CNPJ_11222333000181", got)
	}
}

func TestBuildPersonSK_TooShortReturnsError(t *testing.T) {
	_, err := BuildPersonSK("123")
	if err == nil {
		t.Error("expected error for short input, got nil")
	}
}

func TestBuildPersonSK_12DigitsReturnsError(t *testing.T) {
	_, err := BuildPersonSK("123456789012")
	if err == nil {
		t.Error("expected error for 12-digit input (neither CPF nor CNPJ length)")
	}
}

func TestBuildPersonSK_13DigitsReturnsError(t *testing.T) {
	_, err := BuildPersonSK("1234567890123")
	if err == nil {
		t.Error("expected error for 13-digit input")
	}
}

func TestBuildPersonSK_EmptyReturnsError(t *testing.T) {
	_, err := BuildPersonSK("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// ─── RequirePJFields ──────────────────────────────────────────────────────────

func TestRequirePJFields_CNPJWithoutCRT_ReturnsError(t *testing.T) {
	if err := RequirePJFields("11222333000181", nil); err == nil {
		t.Fatal("expected error for CNPJ without CRT")
	}
}

func TestRequirePJFields_CNPJWithCRT_OK(t *testing.T) {
	if err := RequirePJFields("11222333000181", new(1)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequirePJFields_CPF_NoCRTRequired(t *testing.T) {
	if err := RequirePJFields("11122233344", nil); err != nil {
		t.Fatalf("CPF should not require CRT: %v", err)
	}
}

func TestRequirePJFields_FormattedCNPJWithoutCRT_ReturnsError(t *testing.T) {
	if err := RequirePJFields("11.222.333/0001-81", nil); err == nil {
		t.Fatal("expected error for formatted CNPJ without CRT")
	}
}
