package services

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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
