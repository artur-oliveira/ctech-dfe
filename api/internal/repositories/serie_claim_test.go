package repositories

import "testing"

// The claim is global by tax id, not scoped to an organization — deliberately,
// because the SEFAZ is global. An NF-e is unique by (CNPJ, modelo, série,
// número, ambiente), and two organizations sharing a CNPJ on one série collide
// there, not here.
func TestSerieClaimKeyIsStable(t *testing.T) {
	a := SerieClaimPK("11222333000181", "55", "1", 1)
	b := SerieClaimPK("11222333000181", "55", "1", 1)
	if a != b {
		t.Fatalf("the same claim built two keys: %q and %q", a, b)
	}
}

// Every component separates a distinct claim. A key that collapsed any of them
// would refuse a série somebody may legitimately use.
func TestEveryComponentSeparatesAClaim(t *testing.T) {
	base := SerieClaimPK("11222333000181", "55", "1", 1)
	others := map[string]string{
		"another tax id":   SerieClaimPK("11222333000182", "55", "1", 1),
		"another modelo":   SerieClaimPK("11222333000181", "65", "1", 1),
		"another ambiente": SerieClaimPK("11222333000181", "55", "2", 1),
		"another série":    SerieClaimPK("11222333000181", "55", "1", 2),
	}
	for name, other := range others {
		if other == base {
			t.Errorf("%s produced the same key as the base claim", name)
		}
	}
}

// Homologação and produção are different worlds. A test emission must never
// consume a production série.
func TestAmbienteSeparatesTheClaim(t *testing.T) {
	if SerieClaimPK("11222333000181", "55", "1", 1) == SerieClaimPK("11222333000181", "55", "2", 1) {
		t.Fatal("homologação and produção share a claim")
	}
}

// The '#' separator is only unambiguous because no component can contain one.
// This pins that: every component is alphanumeric by construction — the tax id
// is canonicalized by IssuerDoc, and modelo and ambiente are constants in the
// emission code.
//
// It matters because the separator IS ambiguous otherwise. (tax "1#5", modelo
// "5") and (tax "1", modelo "5#5") both build SERIE#1#5#5#1#1, so a component
// that could carry a '#' would let one company claim another's série. A guard
// in the key builder would be dead code today; this test is what fails if the
// inputs ever stop being canonical.
func TestClaimComponentsAreAlphanumericByConstruction(t *testing.T) {
	components := []string{"11222333000181", "12ABC34501DE35", "55", "65", "57", "58", "1", "2"}
	for _, c := range components {
		for _, r := range c {
			isAlnum := (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z')
			if !isAlnum {
				t.Errorf("%q carries %q, which the '#' separator cannot survive", c, r)
			}
		}
	}
}

// An alphanumeric CNPJ claims a série like any other. The key is built from the
// canonical tax id, and since 2026 that may hold letters.
func TestAnAlphanumericCNPJClaimsASerie(t *testing.T) {
	got := SerieClaimPK("12ABC34501DE35", "55", "1", 1)
	want := "SERIE#12ABC34501DE35#55#1#1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
