package repositories

import "testing"

// The new key. A company id is a UUIDv7 from ctech-account and passes through
// untouched — it is already canonical.
func TestParseOrgPKAcceptsACompanyID(t *testing.T) {
	const id = "0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70"
	got, err := ParseOrgPK(id)
	if err != nil || got != id {
		t.Fatalf("got %q, %v; want the id unchanged", got, err)
	}
	if !IsCompanyKey(id) {
		t.Error("IsCompanyKey said no to a company id")
	}
}

// The legacy shapes stay valid. The old partitions are the rollback, and a
// build that cannot read them cannot roll back.
func TestParseOrgPKStillAcceptsTheLegacyShapes(t *testing.T) {
	for _, in := range []string{"CNPJ_11222333000181", "CPF_52998224725"} {
		got, err := ParseOrgPK(in)
		if err != nil || got != in {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
		if IsCompanyKey(in) {
			t.Errorf("%q: IsCompanyKey said yes to a legacy key", in)
		}
	}
}

func TestParseOrgPKRefusesEverythingElse(t *testing.T) {
	for _, in := range []string{
		"",
		"nope",
		"0199f3a1-8c42-7c31-9d5e",               // short
		"0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f7",   // one short
		"0199f3a1_8c42_7c31_9d5e_6a2b4c8e1f70",  // wrong separators
		"ORG#0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70",
		"0199F3A1-8C42-7C31-9D5E-6A2B4C8E1F70",  // uppercase: not what uuid.String emits
	} {
		if _, err := ParseOrgPK(in); err == nil {
			t.Errorf("%q: accepted, want refused", in)
		}
	}
}

// The legacy path normalizes a typed document. It is what created every key in
// production and must keep working while those partitions exist.
func TestParseOrgPKStillNormalizesATypedDocument(t *testing.T) {
	got, err := ParseOrgPK("11.222.333/0001-81")
	if err != nil || got != "CNPJ_11222333000181" {
		t.Fatalf("got %q, %v", got, err)
	}
}
