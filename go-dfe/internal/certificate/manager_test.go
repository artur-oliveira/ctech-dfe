package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net/http"
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
	if len(tlsCfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
	got := tlsCfg.Certificates[0]
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
