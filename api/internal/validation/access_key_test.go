package validation

import "testing"

// A real authorized NF-e access key used across the worker test suite
// (worker/internal/service/distribution_test.go testAK) — numeric CNPJ, cUF
// 35 (SP), AAMM 2505, mod 55, tpEmis 1. Recomputed cDV below.
const validAccessKeyNumeric = "35250512345678000195550010000000011000000015"

func TestValidAccessKey_ValidNumericCNPJ(t *testing.T) {
	t.Parallel()
	if !ValidAccessKey(validAccessKeyNumeric) {
		t.Fatalf("ValidAccessKey(%q) = false, want true", validAccessKeyNumeric)
	}
}

func TestValidAccessKey_WrongLength(t *testing.T) {
	t.Parallel()
	if ValidAccessKey(validAccessKeyNumeric[:43]) {
		t.Fatal("expected false for a 43-char key")
	}
}

func TestValidAccessKey_InvalidUF(t *testing.T) {
	t.Parallel()
	key := "99" + validAccessKeyNumeric[2:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for cUF=99 (not an IBGE UF code)")
	}
}

func TestValidAccessKey_InvalidMonth(t *testing.T) {
	t.Parallel()
	key := validAccessKeyNumeric[:2] + "2513" + validAccessKeyNumeric[6:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for AAMM month=13")
	}
}

func TestValidAccessKey_WrongMod(t *testing.T) {
	t.Parallel()
	key := validAccessKeyNumeric[:20] + "65" + validAccessKeyNumeric[22:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for mod=65 (NFC-e, out of scope)")
	}
}

func TestValidAccessKey_InvalidTpEmisNFCeOnly(t *testing.T) {
	t.Parallel()
	key := validAccessKeyNumeric[:34] + "9" + validAccessKeyNumeric[35:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for tpEmis=9 (NFC-e contingency only)")
	}
}

func TestValidAccessKey_BadCheckDigit(t *testing.T) {
	t.Parallel()
	last := validAccessKeyNumeric[43]
	bad := byte('0')
	if last == '0' {
		bad = '1'
	}
	key := validAccessKeyNumeric[:43] + string(bad)
	if ValidAccessKey(key) {
		t.Fatal("expected false for a corrupted cDV")
	}
}

func TestValidAccessKey_CPFPrefixedDoc(t *testing.T) {
	t.Parallel()
	// CPF-in-CNPJ-slot convention: "000" + 11-digit CPF (529.982.247-25, valid).
	// cUF/AAMM/mod/serie/nNF/tpEmis/cNF kept from validAccessKeyNumeric; cDV
	// must be recomputed by the test using calcAccessKeyDV since the doc
	// segment changed the first 43 characters.
	base := validAccessKeyNumeric[:6] + "00052998224725" + validAccessKeyNumeric[20:43]
	dv := calcAccessKeyDV(base)
	key := base + string(rune('0'+dv))
	if !ValidAccessKey(key) {
		t.Fatalf("ValidAccessKey(%q) = false, want true (CPF-prefixed doc)", key)
	}
}

func TestValidAccessKey_BothCPFAndCNPJInvalid(t *testing.T) {
	t.Parallel()
	// "000" prefix with a CPF that fails its own check digit.
	base := validAccessKeyNumeric[:6] + "00052998224724" + validAccessKeyNumeric[20:43]
	dv := calcAccessKeyDV(base)
	key := base + string(rune('0'+dv))
	if ValidAccessKey(key) {
		t.Fatal("expected false for an invalid CPF check digit")
	}
}
