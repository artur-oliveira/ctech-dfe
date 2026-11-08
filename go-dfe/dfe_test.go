package dfe

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
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

// SignText round trip with a throwaway certificate generated here (never a real one): the
// signature the egress returns must verify against the public key (RSA PKCS1v15, SHA-1).
func TestCallSignText(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TEST"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pfx, err := pkcs12.Modern.Encode(key, cert, nil, "pw")
	if err != nil {
		t.Fatal(err)
	}
	req := Request{
		DocType: constants.DocTypeNFCE, Service: constants.ServiceSignText,
		CertificateB64: base64.StdEncoding.EncodeToString(pfx), CertificatePassword: "pw",
		Body: map[string]any{constants.BodyKeyText: "35200514200166000187650010000000011000000018|3|2|08|10.00|2|12345678909"},
	}
	if !Implements(req.DocType, req.Service) {
		t.Fatal("SignText must be in the allowlist")
	}
	resp, err := Call(context.Background(), req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("Call: %v %+v", err, resp)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(out[constants.BodyKeySignature])
	if err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum([]byte(req.Body[constants.BodyKeyText].(string)))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA1, sum[:], sig); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}
	// empty text is a 400 problem, not a signature of nothing
	req.Body = map[string]any{}
	if resp, _ := Call(context.Background(), req); resp.StatusCode != 400 {
		t.Errorf("empty text status = %d, want 400", resp.StatusCode)
	}
}

// SignOnly returns the signed enviNFe (Signature after infNFeSupl, inside NFe) and never needs an
// endpoint; the same bytes feed the later NFeAutorizacao with Body["@signed_xml"].
func TestCallSignOnly(t *testing.T) {
	req := Request{
		DocType: constants.DocTypeNFCE, Service: constants.ServiceSignOnly, UF: "SP", Environment: "hom",
		CertificateB64: buildTestPFX(t, "pw"), CertificatePassword: "pw",
		Body: map[string]any{"enviNFe": map[string]any{
			"@versao": "4.00", "idLote": "1", "indSinc": "1",
			"NFe": map[string]any{
				"infNFe":     map[string]any{"@Id": "NFe35200514200166000187650010000000011000000018", "@versao": "4.00", "ide": map[string]any{"cUF": "35"}},
				"infNFeSupl": map[string]any{"qrCode": "https://x?p=1", "urlChave": "https://x"},
			},
		}},
	}
	if !Implements(req.DocType, req.Service) {
		t.Fatal("SignOnly must be in the allowlist")
	}
	resp, err := Call(context.Background(), req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("Call: %v %+v", err, resp)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	x := out[constants.BodyKeyXML]
	i, j, k := strings.Index(x, "</infNFe>"), strings.Index(x, "<infNFeSupl>"), strings.Index(x, "<Signature")
	if i < 0 || !(i < j && j < k) || !strings.Contains(x, "<DigestValue>") {
		t.Fatalf("unexpected signed xml: %s", x)
	}
}

// SignOnly for MDF-e and CT-e (offline tpEmis 2): the signed document comes back with the Signature
// after infXXX and the supplementary group, and SignText is allowed for the QR Code sign parameter.
func TestCallSignOnlyMDFeCTe(t *testing.T) {
	cases := []struct {
		docType, root, inf, supl string
	}{
		{constants.DocTypeMDFE, "MDFe", "infMDFe", "infMDFeSupl"},
		{constants.DocTypeCTE, "CTe", "infCte", "infCTeSupl"},
	}
	for _, c := range cases {
		if !Implements(c.docType, constants.ServiceSignText) || !Implements(c.docType, constants.ServiceSignOnly) {
			t.Fatalf("%s: SignText/SignOnly must be in the allowlist", c.docType)
		}
		req := Request{
			DocType: c.docType, Service: constants.ServiceSignOnly, UF: "SP", Environment: "hom",
			CertificateB64: buildTestPFX(t, "pw"), CertificatePassword: "pw",
			Body: map[string]any{c.root: map[string]any{
				c.inf:  map[string]any{"@Id": c.root + "35200514200166000187580010000000011000000018", "ide": map[string]any{"cUF": "35"}},
				c.supl: map[string]any{"qrCod": "https://x"},
			}},
		}
		resp, err := Call(context.Background(), req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%s: Call: %v %+v", c.docType, err, resp)
		}
		var out map[string]string
		if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
			t.Fatal(err)
		}
		x := out[constants.BodyKeyXML]
		i, k := strings.Index(x, "</"+c.inf+">"), strings.Index(x, "<Signature")
		if i < 0 || k < i || !strings.Contains(x, "<DigestValue>") {
			t.Fatalf("%s: unexpected signed xml: %s", c.docType, x)
		}
	}
}

// SignOnly for the NF-e (EPEC tpEmis 4): signed before the event, transmitted later with @signed_xml.
func TestCallSignOnlyNFe(t *testing.T) {
	req := Request{
		DocType: constants.DocTypeNFE, Service: constants.ServiceSignOnly, UF: "SP", Environment: "hom",
		CertificateB64: buildTestPFX(t, "pw"), CertificatePassword: "pw",
		Body: map[string]any{"enviNFe": map[string]any{
			"@versao": "4.00", "idLote": "1", "indSinc": "1",
			"NFe": map[string]any{
				"infNFe": map[string]any{"@Id": "NFe35200514200166000187550010000000014000000018", "@versao": "4.00", "ide": map[string]any{"cUF": "35"}},
			},
		}},
	}
	if !Implements(req.DocType, req.Service) {
		t.Fatal("SignOnly must be in the nfe allowlist")
	}
	resp, err := Call(context.Background(), req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("Call: %v %+v", err, resp)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	if x := out[constants.BodyKeyXML]; !strings.Contains(x, "<Signature") || !strings.Contains(x, "<DigestValue>") {
		t.Fatalf("unexpected signed xml: %s", x)
	}
}
