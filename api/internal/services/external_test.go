package services

import (
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ---------------------------------------------------------------------------
// pdfRequestPayload / decodePDFResponse (PDF generation)
// ---------------------------------------------------------------------------

func TestPDFRequestPayload_NormalizesCNPJandUF(t *testing.T) {
	// CPF issuer (11 digits) and empty UF must be normalized to satisfy py-dfe.
	p := pdfRequestPayload(DocTypeMDFe, ServiceGerarDamdfe, "", "12345678901", "<xml/>", true)
	if p["cnpj"] != pdfPlaceholderCNPJ {
		t.Errorf("cnpj = %v, want placeholder", p["cnpj"])
	}
	if p["uf"] != "AN" {
		t.Errorf("uf = %v, want AN", p["uf"])
	}
	if p["doc_type"] != DocTypeMDFe || p["service"] != ServiceGerarDamdfe {
		t.Errorf("doc_type/service mismatch: %v / %v", p["doc_type"], p["service"])
	}
	body := p["body"].(map[string]any)
	if body["xml"] != "<xml/>" || body["canceled"] != true {
		t.Errorf("body mismatch: %v", body)
	}
}

func TestPDFRequestPayload_KeepsValidCNPJandUF(t *testing.T) {
	p := pdfRequestPayload(DocTypeNFCe, ServiceGerarDanfe, "SP", "12345678000195", "<x/>", false)
	if p["cnpj"] != "12345678000195" {
		t.Errorf("cnpj = %v, want kept", p["cnpj"])
	}
	if p["uf"] != "SP" {
		t.Errorf("uf = %v, want SP", p["uf"])
	}
}

func TestDecodePDFResponse_OK(t *testing.T) {
	want := []byte("%PDF-1.4 fake")
	resp := map[string]any{"pdf_b64": base64.StdEncoding.EncodeToString(want)}
	got, err := decodePDFResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDecodePDFResponse_Empty(t *testing.T) {
	if _, err := decodePDFResponse(map[string]any{}); err == nil {
		t.Error("expected error for missing pdf_b64")
	}
}

func TestDecodePDFResponse_InvalidBase64(t *testing.T) {
	if _, err := decodePDFResponse(map[string]any{"pdf_b64": "!!!not base64!!!"}); err == nil {
		t.Error("expected error for invalid base64")
	}
}

// ---------------------------------------------------------------------------
// avStr
// ---------------------------------------------------------------------------

func TestAvStr_Found(t *testing.T) {
	item := map[string]types.AttributeValue{
		"s3_key": &types.AttributeValueMemberS{Value: "certs/CNPJ_123/abc.pfx"},
	}
	if got := avStr(item, "s3_key"); got != "certs/CNPJ_123/abc.pfx" {
		t.Errorf("got %q", got)
	}
}

func TestAvStr_Missing(t *testing.T) {
	item := map[string]types.AttributeValue{}
	if got := avStr(item, "s3_key"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// getStr / getMap / asSliceOfMaps
// ---------------------------------------------------------------------------

func TestGetStr_Present(t *testing.T) {
	m := map[string]any{"cStat": "111"}
	if got := getStr(m, "cStat"); got != "111" {
		t.Errorf("got %q", got)
	}
}

func TestGetStr_Missing(t *testing.T) {
	if got := getStr(map[string]any{}, "key"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestGetStr_NilMap(t *testing.T) {
	if got := getStr(nil, "key"); got != "" {
		t.Errorf("expected empty for nil map, got %q", got)
	}
}

func TestGetMap_Present(t *testing.T) {
	inner := map[string]any{"a": "b"}
	m := map[string]any{"inner": inner}
	if got := getMap(m, "inner"); got["a"] != "b" {
		t.Errorf("got %v", got)
	}
}

func TestGetMap_Missing(t *testing.T) {
	if got := getMap(map[string]any{}, "x"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestAsSliceOfMaps_SingleMap(t *testing.T) {
	m := map[string]any{
		"infCad": map[string]any{"UF": "SP"},
	}
	result := asSliceOfMaps(m, "infCad")
	if len(result) != 1 || result[0]["UF"] != "SP" {
		t.Errorf("got %v", result)
	}
}

func TestAsSliceOfMaps_List(t *testing.T) {
	m := map[string]any{
		"infCad": []any{
			map[string]any{"UF": "SP"},
			map[string]any{"UF": "RJ"},
		},
	}
	result := asSliceOfMaps(m, "infCad")
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[1]["UF"] != "RJ" {
		t.Errorf("got %v", result[1])
	}
}

func TestAsSliceOfMaps_Missing(t *testing.T) {
	if result := asSliceOfMaps(map[string]any{}, "key"); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// stripPKPrefixExt
// ---------------------------------------------------------------------------

func TestStripPKPrefixExt_CNPJ(t *testing.T) {
	if got := StripPKPrefix("CNPJ_12345678000195"); got != "12345678000195" {
		t.Errorf("got %q", got)
	}
}

func TestStripPKPrefixExt_CPF(t *testing.T) {
	if got := StripPKPrefix("CPF_12345678901"); got != "12345678901" {
		t.Errorf("got %q", got)
	}
}

func TestStripPKPrefixExt_NoPrefix(t *testing.T) {
	if got := StripPKPrefix("12345678000195"); got != "12345678000195" {
		t.Errorf("got %q", got)
	}
}

// ---------------------------------------------------------------------------
// optStr / firstNonEmpty
// ---------------------------------------------------------------------------

func TestOptStr_NonEmpty(t *testing.T) {
	p := optStr("hello")
	if p == nil || *p != "hello" {
		t.Errorf("got %v", p)
	}
}

func TestOptStr_Empty(t *testing.T) {
	if p := optStr(""); p != nil {
		t.Errorf("expected nil, got %v", p)
	}
}

func TestFirstNonEmpty_UsesA(t *testing.T) {
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("got %q", got)
	}
}

func TestFirstNonEmpty_FallsBackToB(t *testing.T) {
	if got := firstNonEmpty("", "b"); got != "b" {
		t.Errorf("got %q", got)
	}
}
