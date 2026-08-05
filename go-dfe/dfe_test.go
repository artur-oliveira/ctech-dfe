package dfe

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

// buildTestPFX generates a self-signed RSA cert + key and encodes them as a
// PKCS12/PFX blob, returning the base64 form certificate.Load expects.
// Mirrors internal/certificate/manager_test.go's buildTestPFX (unexported
// there, so duplicated here for this package's tests).
func buildTestPFX(t *testing.T, password string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "TEST COMPANY:12345678000195"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	pfxData, err := pkcs12.Modern.Encode(key, cert, nil, password)
	if err != nil {
		t.Fatalf("pkcs12 Encode: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pfxData)
}

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

// TestCall_NFSeReachesProviderConstruction usa um certificado válido (ao
// contrário de TestCall_NFSeRequiresProvider, que falha em certificate.Load
// antes de alcançar qualquer código deste wiring) para provar que Call
// realmente chega em newNFSeProvider/nfse.Dispatch, e não só na validação de
// certificado.
func TestCall_NFSeReachesProviderConstruction(t *testing.T) {
	certB64 := buildTestPFX(t, "s3cr3t")
	resp, err := Call(context.Background(), Request{
		DocType: constants.DocTypeNFSE, Service: constants.ServiceNFSeRecepcao,
		Environment: "hom", CertificateB64: certB64, CertificatePassword: "s3cr3t",
		Body: map[string]any{"provider": "provider-inexistente"},
	})
	if err != nil {
		t.Fatalf("Call devolveu erro cru em vez de Problem: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("StatusCode = %d, esperado 400", resp.StatusCode)
	}
	var p Problem
	if jsonErr := json.Unmarshal([]byte(resp.Body), &p); jsonErr != nil {
		t.Fatalf("response body is not a Problem: %v", jsonErr)
	}
	if !strings.Contains(p.Detail, "provider desconhecido") {
		t.Errorf("Problem.Detail = %q, esperado mencionar provider desconhecido (prova que newNFSeProvider foi alcançado)", p.Detail)
	}
}
