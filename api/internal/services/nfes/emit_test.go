package nfes

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// ─── truncateNatOp ─────────────────────────────────────────────────────────────

func TestTruncateNatOp(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"short", "Venda e Remessa", "Venda e Remessa"},
		{"exactly60", strings.Repeat("a", 60), strings.Repeat("a", 60)},
		{"over60", strings.Repeat("a", 70), strings.Repeat("a", 57) + "..."},
		{"accents counted as runes", strings.Repeat("ç", 61), strings.Repeat("ç", 57) + "..."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateNatOp(c.in)
			if got != c.want {
				t.Errorf("truncateNatOp(%q) = %q, want %q", c.in, got, c.want)
			}
			if r := []rune(got); len(r) > natOpMaxLen {
				t.Errorf("result length %d exceeds natOpMaxLen %d", len(r), natOpMaxLen)
			}
		})
	}
}

// ─── calcDV ──────────────────────────────────────────────────────────────────

func TestCalcDV_KnownKey(t *testing.T) {
	key43 := "3526062716538090005500100000001011123456789"
	dv := calcDV(key43)
	if len(dv) != 1 {
		t.Fatalf("calcDV returned %q (len %d), want single digit", dv, len(dv))
	}
	for _, c := range dv {
		if c < '0' || c > '9' {
			t.Errorf("calcDV result %q is not a digit", dv)
		}
	}
}

func TestCalcDV_AllZeros(t *testing.T) {
	dv := calcDV("0000000000000000000000000000000000000000000")
	if dv != "0" {
		t.Errorf("all-zeros DV = %q, want 0 (remainder 0 → DV 0)", dv)
	}
}

func TestCalcDV_ReturnsSingleDigit(t *testing.T) {
	keys := []string{
		"1111111111111111111111111111111111111111111",
		"9999999999999999999999999999999999999999999",
		"3526062716538090005500100000001011123456789",
	}
	for _, k := range keys {
		dv := calcDV(k)
		if len(dv) != 1 || dv[0] < '0' || dv[0] > '9' {
			t.Errorf("calcDV(%s...) = %q: not a single digit", k[:10], dv)
		}
	}
}

// ─── fmtDhEmi ────────────────────────────────────────────────────────────────

var dhEmiRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$`)

func TestFmtDhEmi_Format(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, loc)
	got := fmtDhEmi(ts)
	if !dhEmiRe.MatchString(got) {
		t.Errorf("fmtDhEmi = %q: does not match RFC3339 local offset format", got)
	}
}

func TestFmtDhEmi_DatePart(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, loc)
	got := fmtDhEmi(ts)
	if len(got) < 10 || got[:10] != "2026-01-15" {
		t.Errorf("fmtDhEmi date part = %q, want 2026-01-15", got[:10])
	}
}

func TestFmtDhEmi_HasBrazilianOffset(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, loc)
	got := fmtDhEmi(ts)
	// Sao Paulo offset: -03:00 (BRST) or -02:00 (summer) — never UTC
	if len(got) < 6 {
		t.Fatalf("fmtDhEmi result too short: %q", got)
	}
	offset := got[len(got)-6:]
	if offset != "-03:00" && offset != "-02:00" {
		t.Errorf("fmtDhEmi offset = %q, expected -03:00 or -02:00", offset)
	}
}

func TestFmtDhEmi_TimePart(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, loc)
	got := fmtDhEmi(ts)
	// total length: 10 (date) + 1 (T) + 8 (time) + 6 (offset) = 25
	if len(got) != 25 {
		t.Errorf("fmtDhEmi len = %d, want 25: %q", len(got), got)
	}
}
