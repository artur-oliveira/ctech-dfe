package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// buildTestPFX generates a self-signed RSA cert + key and encodes them as a
// PKCS12/PFX blob, returning the base64 form NewClient/ParseInfo expect.
func buildTestPFX(t *testing.T, password string) (string, *x509.Certificate) {
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

	return base64.StdEncoding.EncodeToString(pfxData), cert
}

func TestNewClient_Success(t *testing.T) {
	certB64, wantCert := buildTestPFX(t, "s3cr3t")

	client, err := NewClient(certB64, "s3cr3t")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	tlsCfg := transport.TLSClientConfig
	if tlsCfg == nil {
		t.Fatal("expected non-nil TLSClientConfig")
	}
	if !tlsCfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true (SEFAZ server chain is deliberately not validated)")
	}
	if tlsCfg.GetClientCertificate == nil {
		t.Fatal("expected GetClientCertificate (always sends the client certificate)")
	}
	gotPtr, err := tlsCfg.GetClientCertificate(&tls.CertificateRequestInfo{})
	if err != nil {
		t.Fatalf("GetClientCertificate: %v", err)
	}
	got := *gotPtr
	if len(got.Certificate) != 1 || string(got.Certificate[0]) != string(wantCert.Raw) {
		t.Error("client's leaf certificate does not match the generated test certificate")
	}
	if _, ok := got.PrivateKey.(*rsa.PrivateKey); !ok {
		t.Errorf("expected RSA private key, got %T", got.PrivateKey)
	}
}

func TestNewClient_WrongPassword(t *testing.T) {
	certB64, _ := buildTestPFX(t, "s3cr3t")

	_, err := NewClient(certB64, "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestNewClient_CorruptPFX(t *testing.T) {
	_, err := NewClient(base64.StdEncoding.EncodeToString([]byte("not a pfx")), "any")
	if err == nil {
		t.Fatal("expected error for corrupt PFX, got nil")
	}
}

func TestNewClient_InvalidBase64(t *testing.T) {
	_, err := NewClient("not-valid-base64!!!", "any")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestParseInfo(t *testing.T) {
	certB64, wantCert := buildTestPFX(t, "s3cr3t")

	info, err := ParseInfo(certB64, "s3cr3t")
	if err != nil {
		t.Fatalf("ParseInfo: %v", err)
	}
	if info.CN != wantCert.Subject.CommonName {
		t.Errorf("CN = %q, want %q", info.CN, wantCert.Subject.CommonName)
	}
	if info.IsExpired {
		t.Error("expected a freshly-minted cert to not be expired")
	}
	if !info.NotAfter.Equal(wantCert.NotAfter) {
		t.Errorf("NotAfter = %v, want %v", info.NotAfter, wantCert.NotAfter)
	}
}

func TestLoad_SetsDialTimeout(t *testing.T) {
	certB64, _ := buildTestPFX(t, "s3cr3t")
	client, _, _, err := Load(certB64, "s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if tr.DialContext == nil {
		t.Fatal("DialContext is nil: TCP connect has no timeout")
	}
}

// Regressão do 403 do Sefin Nacional (IIS, "Access is denied") em produção: o servidor anuncia a
// lista de CAs aceitas (só a raiz) e o PFX do contribuinte traz só a folha, sem a AC intermediária.
// Com `Certificates`, o Go compara a cadeia com essa lista e, sem casamento, NÃO envia certificado
// (o OpenSSL/curl e o cliente Python original sempre enviam). Load deve enviar sempre.
func TestLoad_SendsCertificateWhenChainDoesNotMatchAcceptableCAs(t *testing.T) {
	newCA := func(cn string, parent *x509.Certificate, parentKey *rsa.PrivateKey, serial int64) (*x509.Certificate, *rsa.PrivateKey) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn},
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
			IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		}
		signer, signerKey := tmpl, key
		if parent != nil {
			signer, signerKey = parent, parentKey
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
		if err != nil {
			t.Fatal(err)
		}
		c, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		return c, key
	}
	root, rootKey := newCA("AC Raiz de Teste", nil, nil, 1)
	inter, interKey := newCA("AC Intermediaria de Teste", root, rootKey, 2)

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "EMPRESA TESTE:00000000000000"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, inter, &leafKey.PublicKey, interKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}
	// PFX só com a folha: a intermediária que a emitiu não vai na cadeia enviada.
	pfx, err := pkcs12.Modern.Encode(leafKey, leaf, nil, "s3cr3t")
	if err != nil {
		t.Fatal(err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(root)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "403 - Forbidden: Access is denied.", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	// ClientCAs preenchido faz o servidor anunciar a lista de CAs aceitas, como o IIS do Sefin.
	srv.TLS = &tls.Config{ClientAuth: tls.RequestClientCert, ClientCAs: pool}
	srv.StartTLS()
	defer srv.Close()

	client, err := NewClient(base64.StdEncoding.EncodeToString(pfx), "s3cr3t")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, esperado 200: o cliente não enviou o certificado", resp.StatusCode)
	}
}
