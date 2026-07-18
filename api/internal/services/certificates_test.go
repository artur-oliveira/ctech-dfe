package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// makePFX builds a self-signed cert whose CN is `cn`, wraps it + a fresh RSA key
// into a password-protected PKCS#12 blob, and returns the blob.
func makePFX(t *testing.T, cn string, notAfter time.Time) ([]byte, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	const pw = "secret"
	pfx, err := pkcs12.Modern2023.Encode(key, cert, nil, pw)
	if err != nil {
		t.Fatal(err)
	}
	return pfx, pw
}

func TestParsePFX_ExtractsCNPJ(t *testing.T) {
	pfx, pw := makePFX(t, "EMPRESA TESTE LTDA:12345678000195", time.Now().Add(365*24*time.Hour))
	_, _, info, err := ParsePFX(pfx, pw)
	if err != nil {
		t.Fatal(err)
	}
	if info.CNPJ != "12345678000195" {
		t.Errorf("CNPJ = %q, want 12345678000195", info.CNPJ)
	}
	if info.CPF != "" {
		t.Errorf("CPF should be empty for an e-CNPJ, got %q", info.CPF)
	}
	if info.IsExpired {
		t.Error("cert should not be expired")
	}
}

func TestParsePFX_ExtractsCPF(t *testing.T) {
	pfx, pw := makePFX(t, "FULANO DE TAL:12345678901", time.Now().Add(24*time.Hour))
	_, _, info, err := ParsePFX(pfx, pw)
	if err != nil {
		t.Fatal(err)
	}
	if info.CPF != "12345678901" {
		t.Errorf("CPF = %q, want 12345678901", info.CPF)
	}
	if info.CNPJ != "" {
		t.Errorf("CNPJ should be empty for an e-CPF, got %q", info.CNPJ)
	}
}

func TestParsePFX_WrongPassword(t *testing.T) {
	pfx, _ := makePFX(t, "EMPRESA:12345678000195", time.Now().Add(24*time.Hour))
	if _, _, _, err := ParsePFX(pfx, "wrong"); err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestMatchOrgDocument(t *testing.T) {
	tests := []struct {
		name    string
		orgPK   string
		info    *CertInfo
		wantErr bool
	}{
		{"cnpj match", "CNPJ_12345678000195", &CertInfo{CNPJ: "12345678000195"}, false},
		{"cnpj mismatch", "CNPJ_12345678000195", &CertInfo{CNPJ: "99999999000199"}, true},
		{"cpf match", "CPF_12345678901", &CertInfo{CPF: "12345678901"}, false},
		{"cpf mismatch", "CPF_12345678901", &CertInfo{CPF: "00000000000"}, true},
		{"no doc in CN is allowed", "CNPJ_12345678000195", &CertInfo{}, false},
		{"cpf org with cnpj cert (no cpf) allowed", "CPF_12345678901", &CertInfo{CNPJ: "12345678000195"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := MatchOrgDocument(tc.orgPK, tc.info)
			if (err != nil) != tc.wantErr {
				t.Errorf("MatchOrgDocument(%s) err=%v, wantErr=%v", tc.orgPK, err, tc.wantErr)
			}
		})
	}
}

func TestCnpjRoot(t *testing.T) {
	if got := cnpjRoot("CNPJ_12345678000195"); got != "12345678" {
		t.Errorf("cnpjRoot = %q, want 12345678", got)
	}
	if got := cnpjRoot("CNPJ_12345678000276"); got != "12345678" {
		t.Errorf("filial root = %q, want 12345678 (same as matriz)", got)
	}
	if got := cnpjRoot("CPF_12345678901"); got != "" {
		t.Errorf("cnpjRoot for CPF = %q, want empty", got)
	}
}
