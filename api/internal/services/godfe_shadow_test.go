package services

import "testing"

func TestMapToDfeRequest(t *testing.T) {
	payload := map[string]any{
		"cnpj": "12345678000195", "certificate_b64": "b64", "certificate_password": "pw",
		"uf": "SP", "environment": "prod", "doc_type": "nfe", "service": "NfeConsultaCadastro",
		"body": map[string]any{"ConsCad": map[string]any{}},
	}
	req, ok := mapToDfeRequest(payload)
	if !ok {
		t.Fatal("expected ok=true for well-formed payload")
	}
	if req.DocType != "nfe" || req.Service != "NfeConsultaCadastro" || req.UF != "SP" {
		t.Errorf("req = %+v", req)
	}

	// Incomplete payloads must be skipped, not treated as errors.
	if _, ok := mapToDfeRequest(map[string]any{"xml": "<NFe/>"}); ok {
		t.Error("expected ok=false for payload missing doc_type/service/uf/body")
	}
}
