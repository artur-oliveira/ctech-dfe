package service

import "testing"

func TestNormalizeSefazEnvironment(t *testing.T) {
	tests := []struct{ in, want string }{
		{sefazEnvProd, envProd},
		{sefazEnvHom, envHom},
		{"prod", "prod"},
		{"hom", "hom"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeSefazEnvironment(tt.in); got != tt.want {
			t.Errorf("normalizeSefazEnvironment(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMapToDfeRequest(t *testing.T) {
	payload := map[string]any{
		"cnpj": "12345678000195", "certificate_b64": "b64", "certificate_password": "pw",
		"uf": "AN", "environment": sefazEnvHom, "doc_type": "nfe", "service": "NFeDistribuicaoDFe",
		"body": map[string]any{"distDFeInt": map[string]any{}},
	}
	req, ok := mapToDfeRequest(payload)
	if !ok {
		t.Fatal("expected ok=true for well-formed payload")
	}
	if req.DocType != "nfe" || req.Service != "NFeDistribuicaoDFe" || req.Environment != envHom {
		t.Errorf("req = %+v", req)
	}

	if _, ok := mapToDfeRequest(map[string]any{"doc_type": "nfe"}); ok {
		t.Error("expected ok=false for payload missing body/uf/service")
	}
}
