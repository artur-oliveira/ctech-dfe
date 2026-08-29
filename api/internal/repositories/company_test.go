package repositories

import "testing"

// The raiz comes off the record's tax id, never off the partition key. Under a
// company id there is no CNPJ in the key to slice.
func TestCNPJRootComesFromTheRecord(t *testing.T) {
	c := &LocalCompany{TaxID: "11222333000181", TaxIDKind: TaxKindCNPJ}
	if got := c.CNPJRoot(); got != "11222333" {
		t.Errorf("CNPJRoot = %q, want 11222333", got)
	}
}

// A CNPJ is alphanumeric in its first twelve positions since 2026, so the raiz
// can hold letters. Slicing is right; assuming digits is not.
func TestCNPJRootHandlesAnAlphanumericCNPJ(t *testing.T) {
	c := &LocalCompany{TaxID: "12ABC34501DE35", TaxIDKind: TaxKindCNPJ}
	if got := c.CNPJRoot(); got != "12ABC345" {
		t.Errorf("CNPJRoot = %q, want 12ABC345", got)
	}
}

// A CPF has no branch concept, so it has no root — and returning a prefix of one
// would make two unrelated people look like matriz and filial.
func TestCNPJRootIsEmptyForACPF(t *testing.T) {
	c := &LocalCompany{TaxID: "52998224725", TaxIDKind: TaxKindCPF}
	if got := c.CNPJRoot(); got != "" {
		t.Errorf("CNPJRoot = %q, want empty", got)
	}
}

// A record with no tax id at all — one that predates the migration — has no
// root either. It must not slice whatever happens to be in the field.
func TestCNPJRootIsEmptyWithoutATaxID(t *testing.T) {
	if got := (&LocalCompany{TaxIDKind: TaxKindCNPJ}).CNPJRoot(); got != "" {
		t.Errorf("CNPJRoot = %q, want empty", got)
	}
	if got := (&LocalCompany{}).CNPJRoot(); got != "" {
		t.Errorf("CNPJRoot = %q, want empty", got)
	}
}
