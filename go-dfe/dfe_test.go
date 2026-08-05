package dfe

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
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

func TestImplements_NFSe(t *testing.T) {
	for _, svc := range []string{
		constants.ServiceNFSeRecepcao, constants.ServiceNFSeConsulta,
		constants.ServiceNFSeConsultaDPS, constants.ServiceNFSeEvento,
		constants.ServiceNFSeConsultaEvento, constants.ServiceNFSeDistribuicao,
		constants.ServiceNFSeDANFSE, constants.ServiceNFSeParametrosMunicipais,
	} {
		if !Implements(constants.DocTypeNFSE, svc) {
			t.Errorf("Implements(nfse, %q) = false, esperado true", svc)
		}
	}
	if Implements(constants.DocTypeNFSE, "ServicoInexistente") {
		t.Error("Implements aceitou serviço desconhecido")
	}
}

func TestCall_NFSeRequiresProvider(t *testing.T) {
	resp, err := Call(context.Background(), Request{
		DocType: constants.DocTypeNFSE, Service: constants.ServiceNFSeRecepcao,
		Environment: "hom", CertificateB64: "x", Body: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Call devolveu erro cru em vez de Problem: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("StatusCode = %d, esperado 400 para body sem provider", resp.StatusCode)
	}
}
