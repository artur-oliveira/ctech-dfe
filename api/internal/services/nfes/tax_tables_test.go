package nfes

import (
	"regexp"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

var decimalRe = regexp.MustCompile(`^\d+\.\d{2}$`)
var xsdInterEnum = map[string]bool{"4.00": true, "7.00": true, "12.00": true}

var allUFs = []string{
	"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA",
	"MG", "MS", "MT", "PA", "PB", "PE", "PI", "PR", "RJ", "RN",
	"RO", "RR", "RS", "SC", "SE", "SP", "TO",
}

// ─── aliqICMSTable ────────────────────────────────────────────────────────────

func TestAliqICMSTable_All27UFsPresent(t *testing.T) {
	if len(aliqICMSTable) != 27 {
		t.Fatalf("expected 27 rows, got %d", len(aliqICMSTable))
	}
	for uf, row := range aliqICMSTable {
		if len(row) != 27 {
			t.Errorf("UF %s: expected 27 columns, got %d", uf, len(row))
		}
	}
}

func TestAliqICMSTable_KnownIntrastate(t *testing.T) {
	cases := []struct{ uf, want string }{
		{"SP", "18.00"},
		{"RJ", "22.00"},
		{"MG", "18.00"},
		{"RS", "17.00"},
		{"SC", "17.00"},
		{"MA", "23.00"},
		{"AL", "21.50"},
	}
	for _, tc := range cases {
		got := aliqICMSTable[tc.uf][tc.uf]
		if got != tc.want {
			t.Errorf("aliqICMSTable[%s][%s] = %q, want %q", tc.uf, tc.uf, got, tc.want)
		}
	}
}

func TestAliqICMSTable_AllValuesDecimalFormat(t *testing.T) {
	for uf, row := range aliqICMSTable {
		for dest, aliq := range row {
			if !decimalRe.MatchString(aliq) {
				t.Errorf("aliqICMSTable[%s][%s] = %q: not ##.## format", uf, dest, aliq)
			}
		}
	}
}

func TestAliqICMSTable_IntraAlwaysGt12(t *testing.T) {
	for _, uf := range allUFs {
		intra := resolveICMSIntraAliq(uf)
		n := parseDecimal(intra)
		if n < 17.0 {
			t.Errorf("UF %s intra rate %v unexpectedly low (< 17)", uf, n)
		}
	}
}

func parseDecimal(s string) float64 {
	dot := -1
	for i, c := range s {
		if c == '.' {
			dot = i
		}
	}
	if dot < 0 {
		return 0
	}
	var ip, fp int64
	for _, c := range s[:dot] {
		ip = ip*10 + int64(c-'0')
	}
	div := int64(1)
	for _, c := range s[dot+1:] {
		fp = fp*10 + int64(c-'0')
		div *= 10
	}
	return float64(ip) + float64(fp)/float64(div)
}

// ─── fcpAliq ─────────────────────────────────────────────────────────────────

func TestFcpAliq_RJHasFCP(t *testing.T) {
	if fcpAliq["RJ"] != "2.00" {
		t.Errorf("fcpAliq[RJ] = %q, want 2.00", fcpAliq["RJ"])
	}
}

func TestFcpAliq_PEHasFCP(t *testing.T) {
	if fcpAliq["PE"] != "2.00" {
		t.Errorf("fcpAliq[PE] = %q, want 2.00", fcpAliq["PE"])
	}
}

func TestFcpAliq_MGHasFCP(t *testing.T) {
	if fcpAliq["MG"] != "2.00" {
		t.Errorf("fcpAliq[MG] = %q, want 2.00 (bug: was 2.0)", fcpAliq["MG"])
	}
}

func TestFcpAliq_AllValuesDecimalFormat(t *testing.T) {
	for uf, aliq := range fcpAliq {
		if !decimalRe.MatchString(aliq) {
			t.Errorf("fcpAliq[%s] = %q: not ##.## format", uf, aliq)
		}
	}
}

// ─── services.UFCode ───────────────────────────────────────────────────────────────────

func TestUFCode_All27UFsPresent(t *testing.T) {
	if len(services.UFCode) != 27 {
		t.Fatalf("services.UFCode: expected 27 entries, got %d", len(services.UFCode))
	}
}

func TestUFCode_KnownCodes(t *testing.T) {
	cases := []struct{ uf, code string }{
		{"SP", "35"},
		{"RJ", "33"},
		{"MG", "31"},
		{"BA", "29"},
		{"RS", "43"},
		{"SC", "42"},
		{"PR", "41"},
		{"DF", "53"},
	}
	for _, tc := range cases {
		got := services.UFCode[tc.uf]
		if got != tc.code {
			t.Errorf("services.UFCode[%s] = %q, want %q", tc.uf, got, tc.code)
		}
	}
}

// ─── resolveICMSAliq ─────────────────────────────────────────────────────────

func TestResolveICMSAliq_IntrastateSP(t *testing.T) {
	got := resolveICMSAliq("SP", "SP", nil)
	if got != "18.00" {
		t.Errorf("SP→SP = %q, want 18.00", got)
	}
}

func TestResolveICMSAliq_UnknownUFFallback(t *testing.T) {
	got := resolveICMSAliq("ZZ", "ZZ", nil)
	if got != "17.00" {
		t.Errorf("ZZ→ZZ fallback = %q, want 17.00", got)
	}
}

func TestResolveICMSAliq_OverrideTakesPrecedence(t *testing.T) {
	got := resolveICMSAliq("SP", "SP", new("25.00"))
	if got != "25.00" {
		t.Errorf("override = %q, want 25.00", got)
	}
}

func TestResolveICMSAliq_InterstateSulToNorte(t *testing.T) {
	got := resolveICMSAliq("SP", "AM", nil)
	if got != "7.00" {
		t.Errorf("SP→AM = %q, want 7.00", got)
	}
}

func TestResolveICMSAliq_InterstateSulToSul(t *testing.T) {
	got := resolveICMSAliq("SP", "PR", nil)
	if got != "12.00" {
		t.Errorf("SP→PR = %q, want 12.00", got)
	}
}

func TestResolveICMSAliq_InterestateNorteToNorte(t *testing.T) {
	got := resolveICMSAliq("AM", "PA", nil)
	if got != "12.00" {
		t.Errorf("AM→PA = %q, want 12.00", got)
	}
}

func TestResolveICMSAliq_EmptyOverrideUsesTable(t *testing.T) {
	got := resolveICMSAliq("SP", "SP", new(""))
	if got != "18.00" {
		t.Errorf("empty override SP→SP = %q, want 18.00", got)
	}
}

// ─── resolveFCPAliq ──────────────────────────────────────────────────────────

func TestResolveFCPAliq_RJHasFCP(t *testing.T) {
	got := resolveFCPAliq("RJ", nil)
	if got != "2.00" {
		t.Errorf("RJ FCP = %q, want 2.00", got)
	}
}

func TestResolveFCPAliq_SPHasFCP(t *testing.T) {
	got := resolveFCPAliq("SP", nil)
	if got != "2.00" {
		t.Errorf("SP FCP = %q, want 2.00", got)
	}
}

func TestResolveFCPAliq_UnknownUFReturnsZero(t *testing.T) {
	got := resolveFCPAliq("ZZ", nil)
	if got != "0.00" {
		t.Errorf("ZZ FCP = %q, want 0.00", got)
	}
}

func TestResolveFCPAliq_OverrideTakesPrecedence(t *testing.T) {
	got := resolveFCPAliq("SP", new("1.00"))
	if got != "1.00" {
		t.Errorf("SP FCP override = %q, want 1.00", got)
	}
}

func TestResolveFCPAliq_OverrideZeroForRJ(t *testing.T) {
	got := resolveFCPAliq("RJ", new("0.00"))
	if got != "0.00" {
		t.Errorf("RJ FCP override 0.00 = %q, want 0.00", got)
	}
}

func TestResolveFCPAliq_SCHasZero(t *testing.T) {
	got := resolveFCPAliq("SC", nil)
	if got != "0.00" {
		t.Errorf("SC FCP = %q, want 0.00", got)
	}
}

// ─── resolveICMSIntraAliq ─────────────────────────────────────────────────────

func TestResolveICMSIntraAliq_KnownUFs(t *testing.T) {
	cases := []struct{ uf, want string }{
		{"SP", "18.00"},
		{"RJ", "22.00"},
		{"MG", "18.00"},
		{"RS", "17.00"},
		{"SC", "17.00"},
		{"MA", "23.00"},
		{"AL", "21.50"},
	}
	for _, tc := range cases {
		got := resolveICMSIntraAliq(tc.uf)
		if got != tc.want {
			t.Errorf("resolveICMSIntraAliq(%s) = %q, want %q", tc.uf, got, tc.want)
		}
	}
}

func TestResolveICMSIntraAliq_UnknownUFFallback(t *testing.T) {
	got := resolveICMSIntraAliq("ZZ")
	if got != "17.00" {
		t.Errorf("unknown UF fallback = %q, want 17.00", got)
	}
}

func TestResolveICMSIntraAliq_All27UFsDecimalFormat(t *testing.T) {
	for _, uf := range allUFs {
		got := resolveICMSIntraAliq(uf)
		if !decimalRe.MatchString(got) {
			t.Errorf("resolveICMSIntraAliq(%s) = %q: not ##.## format", uf, got)
		}
	}
}

func TestResolveICMSInterAliq_Origin1Returns4(t *testing.T) {
	got := resolveICMSInterAliq("SP", "AM", new("1"))
	if got != "4.00" {
		t.Errorf("origin=1 = %q, want 4.00", got)
	}
}

func TestResolveICMSInterAliq_Origin2Returns4(t *testing.T) {
	got := resolveICMSInterAliq("RJ", "BA", new("2"))
	if got != "4.00" {
		t.Errorf("origin=2 = %q, want 4.00", got)
	}
}

func TestResolveICMSInterAliq_Origin6Returns4(t *testing.T) {
	got := resolveICMSInterAliq("SP", "GO", new("6"))
	if got != "4.00" {
		t.Errorf("origin=6 = %q, want 4.00", got)
	}
}

func TestResolveICMSInterAliq_Origin7Returns4(t *testing.T) {
	got := resolveICMSInterAliq("MG", "PA", new("7"))
	if got != "4.00" {
		t.Errorf("origin=7 = %q, want 4.00", got)
	}
}

func TestResolveICMSInterAliq_SPToAMReturns7(t *testing.T) {
	got := resolveICMSInterAliq("SP", "AM", new("0"))
	if got != "7.00" {
		t.Errorf("SP→AM origin=0 = %q, want 7.00", got)
	}
}

func TestResolveICMSInterAliq_RSToBAReturns7(t *testing.T) {
	got := resolveICMSInterAliq("RS", "BA", new("0"))
	if got != "7.00" {
		t.Errorf("RS→BA origin=0 = %q, want 7.00", got)
	}
}

func TestResolveICMSInterAliq_SPToRJReturns12(t *testing.T) {
	got := resolveICMSInterAliq("SP", "RJ", new("0"))
	if got != "12.00" {
		t.Errorf("SP→RJ origin=0 = %q, want 12.00", got)
	}
}

func TestResolveICMSInterAliq_MGToPRReturns12(t *testing.T) {
	got := resolveICMSInterAliq("MG", "PR", new("0"))
	if got != "12.00" {
		t.Errorf("MG→PR origin=0 = %q, want 12.00", got)
	}
}

func TestResolveICMSInterAliq_Origin0NeverReturns4(t *testing.T) {
	got := resolveICMSInterAliq("SP", "BA", new("0"))
	if got == "4.00" {
		t.Errorf("origin=0 must not return 4.00")
	}
}

func TestResolveICMSInterAliq_AlwaysValidEnum(t *testing.T) {
	pairs := [][2]string{
		{"SP", "AM"}, {"SP", "RJ"}, {"MG", "BA"}, {"RS", "SC"}, {"AC", "PA"},
	}
	for _, ov := range []string{"0", "1", "2", "3", "4", "5", "6", "7"} {
		for _, p := range pairs {
			got := resolveICMSInterAliq(p[0], p[1], new(ov))
			if !xsdInterEnum[got] {
				t.Errorf("%s→%s origin=%s: %q not in {4.00,7.00,12.00}", p[0], p[1], ov, got)
			}
		}
	}
}

func TestResolveICMSInterAliq_NilOriginUsesTable(t *testing.T) {
	got := resolveICMSInterAliq("SP", "AM", nil)
	if !xsdInterEnum[got] {
		t.Errorf("nil origin SP→AM = %q: not valid", got)
	}
}
