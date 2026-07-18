package dfe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestImplements(t *testing.T) {
	tests := []struct {
		docType, service string
		want             bool
	}{
		{"nfe", "NfeStatusServico", true},
		{"nfe", "NFeRetAutorizacao", true},
		{"nfe", "NFeAutorizacao", true}, // signed op, not promoted yet
		{"nfce", "NfeConsultaProtocolo", true},
		{"nfce", "NFeRetAutorizacao", true},
		{"cte", "CTeStatusServico", true},
		{"cte", "CTeRecepcaoSinc", true}, // signed op, not promoted yet
		{"mdfe", "MDFeConsNaoEnc", true},
		{"nfe", "NotAService", false},
		{"unknown", "NfeStatusServico", false},
	}
	for _, tt := range tests {
		if got := Implements(tt.docType, tt.service); got != tt.want {
			t.Errorf("Implements(%q, %q) = %v, want %v", tt.docType, tt.service, got, tt.want)
		}
	}
}

func TestCall_NotImplemented(t *testing.T) {
	_, err := Call(context.Background(), Request{DocType: "nfe", Service: "NotImplemented"})
	if err == nil {
		t.Fatal("expected error for unimplemented service, got nil")
	}
}

func TestCall_MissingCertificateReturnsProblemResponse(t *testing.T) {
	resp, err := Call(context.Background(), Request{
		DocType: "nfe", Service: "NfeStatusServico", UF: "SP", Environment: "hom",
		Body: map[string]any{"consStatServ": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", resp.StatusCode)
	}
	var p Problem
	if jsonErr := json.Unmarshal([]byte(resp.Body), &p); jsonErr != nil {
		t.Fatalf("response body is not a Problem: %v", jsonErr)
	}
	if !strings.Contains(p.Detail, "certificate") {
		t.Errorf("Problem.Detail = %q, want it to mention the certificate requirement", p.Detail)
	}
}

func TestCall_InvalidCertificateReturnsProblemResponse(t *testing.T) {
	resp, err := Call(context.Background(), Request{
		DocType: "nfe", Service: "NfeStatusServico", UF: "SP", Environment: "hom",
		CertificateB64: "not-valid-base64!!!", CertificatePassword: "x",
		Body: map[string]any{"consStatServ": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", resp.StatusCode)
	}
}
