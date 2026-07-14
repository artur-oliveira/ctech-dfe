package services

import (
	"fmt"
	"testing"
)

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
