package repositories

import (
	"testing"
	"time"
)

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

// A record that predates the identity cache must read as stale, not as fresh
// with empty names — otherwise the first read after the migration shows blanks
// and never refreshes.
func TestAnUnsyncedIdentityIsStale(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	if !(&LocalCompany{}).IdentityStale(now, time.Hour) {
		t.Error("a record with no sync timestamp read as fresh")
	}
	fresh := &LocalCompany{IdentitySyncedAt: now.Add(-time.Minute).UTC().Format(time.RFC3339)}
	if fresh.IdentityStale(now, time.Hour) {
		t.Error("a record synced a minute ago read as stale")
	}
	old := &LocalCompany{IdentitySyncedAt: now.Add(-2 * time.Hour).UTC().Format(time.RFC3339)}
	if !old.IdentityStale(now, time.Hour) {
		t.Error("a record synced two hours ago read as fresh")
	}
}

// An unparseable timestamp is stale, not fresh. Failing the other way means one
// corrupt value pins a wrong name in place forever, with nothing to notice it.
func TestAnUnparseableSyncTimestampIsStale(t *testing.T) {
	now := time.Now()
	if !(&LocalCompany{IdentitySyncedAt: "ontem"}).IdentityStale(now, time.Hour) {
		t.Error("an unparseable timestamp read as fresh")
	}
}

// A clock that ran backwards — a record stamped in the future — must not read as
// stale forever, nor drive a refresh loop.
func TestAFutureSyncTimestampIsFresh(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	ahead := &LocalCompany{IdentitySyncedAt: now.Add(time.Minute).UTC().Format(time.RFC3339)}
	if ahead.IdentityStale(now, time.Hour) {
		t.Error("a record stamped a minute ahead read as stale")
	}
}
