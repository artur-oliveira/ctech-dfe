package services

import "testing"

// ─── RequireOrgIE ─────────────────────────────────────────────────────────────

func TestRequireOrgIE_CNPJWithoutStateRegistrations_ReturnsError(t *testing.T) {
	if err := RequireOrgIE("11222333000181", nil); err == nil {
		t.Fatal("expected error for CNPJ org without state_registrations")
	}
}

func TestRequireOrgIE_CNPJWithStateRegistration_OK(t *testing.T) {
	regs := []StateRegistrationEntry{{UF: "SP", StateRegistration: "123456"}}
	if err := RequireOrgIE("11222333000181", regs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireOrgIE_CPF_NoIERequired(t *testing.T) {
	if err := RequireOrgIE("11122233344", nil); err != nil {
		t.Fatalf("CPF org should not require IE: %v", err)
	}
}

func TestRequireOrgIE_FormattedCNPJWithoutStateRegistrations_ReturnsError(t *testing.T) {
	if err := RequireOrgIE("11.222.333/0001-81", []StateRegistrationEntry{}); err == nil {
		t.Fatal("expected error for formatted CNPJ org without state_registrations")
	}
}
