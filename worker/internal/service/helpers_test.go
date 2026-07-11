package service

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ---------------------------------------------------------------------------
// findValue
// ---------------------------------------------------------------------------

func TestFindValue_TopLevel(t *testing.T) {
	result := findValue(map[string]any{"cStat": "100"}, "cStat")
	if result == nil || *result != "100" {
		t.Errorf("got %v, want 100", result)
	}
}

func TestFindValue_Nested(t *testing.T) {
	data := map[string]any{
		"retEnviNFe": map[string]any{
			"protNFe": map[string]any{
				"infProt": map[string]any{"cStat": "100"},
			},
		},
	}
	result := findValue(data, "cStat")
	if result == nil || *result != "100" {
		t.Errorf("got %v, want 100", result)
	}
}

func TestFindValue_InList(t *testing.T) {
	data := map[string]any{"items": []any{map[string]any{"cStat": "110"}}}
	result := findValue(data, "cStat")
	if result == nil || *result != "110" {
		t.Errorf("got %v, want 110", result)
	}
}

func TestFindValue_Missing(t *testing.T) {
	if result := findValue(map[string]any{"a": "b"}, "cStat"); result != nil {
		t.Errorf("expected nil, got %v", *result)
	}
}

func TestFindValue_ConvertsToString(t *testing.T) {
	result := findValue(map[string]any{"cStat": 100}, "cStat")
	if result == nil || *result != "100" {
		t.Errorf("got %v, want 100", result)
	}
}

func TestFindValue_NilData(t *testing.T) {
	if result := findValue(nil, "cStat"); result != nil {
		t.Errorf("expected nil, got %v", *result)
	}
}

// ---------------------------------------------------------------------------
// findDict
// ---------------------------------------------------------------------------

func TestFindDict_TopLevel(t *testing.T) {
	data := map[string]any{"infProt": map[string]any{"cStat": "100"}}
	result := findDict(data, "infProt")
	if result == nil || result["cStat"] != "100" {
		t.Errorf("got %v", result)
	}
}

func TestFindDict_Nested(t *testing.T) {
	data := map[string]any{
		"retEnviNFe": map[string]any{
			"protNFe": map[string]any{
				"infProt": map[string]any{"cStat": "100"},
			},
		},
	}
	result := findDict(data, "infProt")
	if result == nil || result["cStat"] != "100" {
		t.Errorf("got %v", result)
	}
}

func TestFindDict_InList(t *testing.T) {
	data := map[string]any{
		"items": []any{map[string]any{"infProt": map[string]any{"nProt": "xyz"}}},
	}
	result := findDict(data, "infProt")
	if result == nil || result["nProt"] != "xyz" {
		t.Errorf("got %v", result)
	}
}

func TestFindDict_Missing(t *testing.T) {
	if result := findDict(map[string]any{"a": "b"}, "infProt"); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFindDict_IgnoresNonDictValue(t *testing.T) {
	if result := findDict(map[string]any{"infProt": "string"}, "infProt"); result != nil {
		t.Errorf("expected nil for non-dict value, got %v", result)
	}
}

func TestFindDict_NilData(t *testing.T) {
	if result := findDict(nil, "infProt"); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// buildUpdateExpression
// ---------------------------------------------------------------------------

func TestBuildUpdateExpression_OnlyStatusAndUpdated(t *testing.T) {
	parts := buildUpdateExpression("authorized", updateAttrs{})

	if !strings.Contains(parts.expression, "#status = :status") {
		t.Errorf("expression missing '#status = :status': %s", parts.expression)
	}
	if !strings.Contains(parts.expression, "updated_at = :updated") {
		t.Errorf("expression missing 'updated_at = :updated': %s", parts.expression)
	}
	if parts.attrNames["#status"] != "status" {
		t.Errorf("attrNames[#status] = %q, want 'status'", parts.attrNames["#status"])
	}
	sv, ok := parts.attrValues[":status"]
	if !ok {
		t.Fatal("attrValues missing :status")
	}
	if s, ok := sv.(*types.AttributeValueMemberS); !ok || s.Value != "authorized" {
		t.Errorf(":status = %v, want authorized", sv)
	}
	if _, ok := parts.attrValues[":updated"]; !ok {
		t.Error("attrValues missing :updated")
	}
}

func TestBuildUpdateExpression_OptionalAttrsIncludedWhenSet(t *testing.T) {
	parts := buildUpdateExpression("authorized", updateAttrs{
		SefazStatus:   new("100"),
		SefazMotive:   new("Autorizado"),
		SefazProtocol: nil,
	})

	if _, ok := parts.attrValues[":sefaz_status"]; !ok {
		t.Error("expected :sefaz_status in attrValues")
	}
	if _, ok := parts.attrValues[":sefaz_motive"]; !ok {
		t.Error("expected :sefaz_motive in attrValues")
	}
	if _, ok := parts.attrValues[":sefaz_protocol"]; ok {
		t.Error("unexpected :sefaz_protocol — nil should be excluded")
	}
	if !strings.Contains(parts.expression, "sefaz_status = :sefaz_status") {
		t.Errorf("expression missing sefaz_status assignment: %s", parts.expression)
	}
}

func TestBuildUpdateExpression_NoNilAttrsInExpression(t *testing.T) {
	parts := buildUpdateExpression("failed", updateAttrs{
		SefazStatus: nil,
		XMLS3Key:    nil,
	})

	if strings.Contains(parts.expression, "sefaz_status") {
		t.Errorf("nil sefaz_status should not appear in expression: %s", parts.expression)
	}
	if strings.Contains(parts.expression, "xml_s3_key") {
		t.Errorf("nil xml_s3_key should not appear in expression: %s", parts.expression)
	}
}
