package services

import (
	"fmt"
	"testing"
)

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

// ─── AuthorizedViewer ─────────────────────────────────────────────────────────

func TestAppendAuthorizedViewer_RejectsDuplicateCpfCnpj(t *testing.T) {
	current := []AuthorizedViewerEntry{{CpfOrCnpj: "11122233344", Name: "Existing"}}
	_, err := appendAuthorizedViewer(current, AuthorizedViewerEntry{CpfOrCnpj: "111.222.333-44", Name: "Dup"})
	if err == nil {
		t.Fatal("expected error for duplicate cpf_cnpj (formatting-insensitive)")
	}
}

func TestAppendAuthorizedViewer_RejectsAt11thEntry(t *testing.T) {
	current := make([]AuthorizedViewerEntry, maxAuthorizedViewers)
	for i := range current {
		current[i] = AuthorizedViewerEntry{CpfOrCnpj: fmt.Sprintf("%011d", i), Name: "V"}
	}
	_, err := appendAuthorizedViewer(current, AuthorizedViewerEntry{CpfOrCnpj: "99999999999", Name: "Overflow"})
	if err == nil {
		t.Fatal("expected error at 11th entry")
	}
}

func TestAppendAuthorizedViewer_AppendsValidNormalized(t *testing.T) {
	out, err := appendAuthorizedViewer(nil, AuthorizedViewerEntry{CpfOrCnpj: "111.222.333-44", Name: "New"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].CpfOrCnpj != "11122233344" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestRemoveAuthorizedViewerEntry_FiltersMatching(t *testing.T) {
	viewers := []AuthorizedViewerEntry{
		{CpfOrCnpj: "11122233344", Name: "A"},
		{CpfOrCnpj: "22233344455", Name: "B"},
	}
	out := removeAuthorizedViewerEntry(viewers, "111.222.333-44")
	if len(out) != 1 || out[0].CpfOrCnpj != "22233344455" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestRemoveAuthorizedViewerEntry_NoMatchIsNoop(t *testing.T) {
	viewers := []AuthorizedViewerEntry{{CpfOrCnpj: "11122233344", Name: "A"}}
	out := removeAuthorizedViewerEntry(viewers, "99988877766")
	if len(out) != 1 {
		t.Fatalf("expected list unchanged, got %+v", out)
	}
}

func TestExtractAuthorizedViewers_ReadsFromPlainMap(t *testing.T) {
	item := map[string]any{
		"authorized_xml_viewers": []any{
			map[string]any{"cpf_cnpj": "11122233344", "name": "Contador"},
		},
	}
	out := extractAuthorizedViewers(item)
	if len(out) != 1 || out[0] != (AuthorizedViewerEntry{CpfOrCnpj: "11122233344", Name: "Contador"}) {
		t.Fatalf("unexpected result: %+v", out)
	}
}
