package nfes

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func orgWithUF(uf string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"person": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"addresses": &types.AttributeValueMemberL{Value: []types.AttributeValue{
				&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"state_federation": &types.AttributeValueMemberS{Value: uf},
				}},
			}},
		}},
	}
}

func TestGenerateAccessKey_Model65(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	key, err := generateAccessKey("CNPJ_11222333000181", orgWithUF("SP"), 1, 1, now, nfModel65, tpEmisNormal)
	if err != nil {
		t.Fatalf("generateAccessKey: %v", err)
	}
	if len(key) != 44 {
		t.Fatalf("key length = %d, want 44", len(key))
	}
	if got := key[20:22]; got != "65" {
		t.Errorf("model digits = %q, want 65", got)
	}
	if dv := calcDV(key[:43]); string(key[43]) != dv {
		t.Errorf("check digit = %q, want %q", string(key[43]), dv)
	}
}

func TestBuildQRCode_OnlineV2(t *testing.T) {
	chave := strings.Repeat("3", 44)
	cscID := "1"
	csc := "ABCDEF0123456789ABCDEF0123456789"
	qr := buildQRCode("https://www.nfce.fazenda.sp.gov.br/qrcode", chave, cscID, csc, 2)

	if !strings.HasPrefix(qr, "https://www.nfce.fazenda.sp.gov.br/qrcode?p=") {
		t.Fatalf("unexpected prefix: %s", qr)
	}
	parts := strings.Split(strings.SplitN(qr, "?p=", 2)[1], "|")
	if len(parts) != 5 {
		t.Fatalf("p has %d fields, want 5: %v", len(parts), parts)
	}
	if parts[0] != chave || parts[1] != "2" || parts[2] != "2" || parts[3] != cscID {
		t.Errorf("p fields = %v", parts[:4])
	}
	// Hash must match SHA1(chave|2|2|cscID + CSC) upper-hex.
	sum := sha1.Sum([]byte(strings.Join([]string{chave, "2", "2", cscID}, "|") + csc))
	want := strings.ToUpper(hex.EncodeToString(sum[:]))
	if parts[4] != want {
		t.Errorf("hash = %s, want %s", parts[4], want)
	}
}

func TestAppendQueryParam(t *testing.T) {
	cases := map[string]string{
		"https://x/qrcode?": "https://x/qrcode?p=1",
		"https://x/qrcode":  "https://x/qrcode?p=1",
		"https://x/q?a=b":   "https://x/q?a=b&p=1",
	}
	for base, want := range cases {
		if got := appendQueryParam(base, "p=1"); got != want {
			t.Errorf("appendQueryParam(%q) = %q, want %q", base, got, want)
		}
	}
}

func TestBuildNFCeSupl_Errors(t *testing.T) {
	if _, err := buildNFCeSupl("XX", 2, strings.Repeat("3", 44), "1", "csc", 2); err == nil {
		t.Error("expected error for unknown UF")
	}
	if _, err := buildNFCeSupl("SP", 2, strings.Repeat("3", 44), "", "", 2); err == nil {
		t.Error("expected error for missing CSC")
	}
	supl, err := buildNFCeSupl("SP", 2, strings.Repeat("3", 44), "1", "csc", 2)
	if err != nil {
		t.Fatalf("buildNFCeSupl: %v", err)
	}
	if supl["qrCode"] == "" || supl["urlChave"] == "" {
		t.Errorf("supl missing fields: %v", supl)
	}
}

func TestBuildSubstituteBody_Structure(t *testing.T) {
	chave := strings.Repeat("3", 44)
	body := buildSubstituteBody(chave, "CNPJ", "11222333000181", 2, "PROT123", strings.Repeat("4", 44), "just", 1, "ver")
	env := body["envEvento"].(map[string]any)
	evt := env["evento"].(map[string]any)
	inf := evt["infEvento"].(map[string]any)
	if inf["tpEvento"] != TpEventoCancelamentoSubst {
		t.Errorf("tpEvento = %v, want %s", inf["tpEvento"], TpEventoCancelamentoSubst)
	}
	det := inf["detEvento"].(map[string]any)
	if det["chNFeRef"] != strings.Repeat("4", 44) {
		t.Errorf("chNFeRef = %v", det["chNFeRef"])
	}
	if det["nProt"] != "PROT123" {
		t.Errorf("nProt = %v", det["nProt"])
	}
}

// Manual DANFE NFC-e 4.4.1: online v3 is chave|3|tpAmb, with no CSC, hash or signature.
func TestBuildNFCeSupl_V3NoCSC(t *testing.T) {
	chave := strings.Repeat("3", 44)
	supl, err := buildNFCeSupl("SP", 1, chave, "", "", 3)
	if err != nil {
		t.Fatalf("v3 must not need the CSC: %v", err)
	}
	want := "https://www.nfce.fazenda.sp.gov.br/qrcode?p=" + chave + "|3|1"
	if supl["qrCode"] != want {
		t.Errorf("qrCode = %v, want %s", supl["qrCode"], want)
	}
	if _, err := buildNFCeSupl("SP", 2, chave, "", "", 2); err == nil {
		t.Error("v2 without CSC must still fail")
	}
}

// ZX02-222 / rejeição 444.
func TestCheckQRVersionForIssuer(t *testing.T) {
	cpf, cnpj := "12345678909", "11222333000181"
	if checkQRVersionForIssuer(2, cpf, "SP") == nil {
		t.Error("v2 with CPF issuer must be rejected")
	}
	for _, c := range []struct {
		v       int
		doc, uf string
	}{{3, cpf, "SP"}, {2, cnpj, "SP"}, {2, cpf, "PR"}} {
		if err := checkQRVersionForIssuer(c.v, c.doc, c.uf); err != nil {
			t.Errorf("%+v: %v", c, err)
		}
	}
}
