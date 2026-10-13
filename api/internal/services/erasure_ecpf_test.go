package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// icpBrasilSAN builds a SubjectAltName with the ICP-Brasil "pessoa física"
// otherName: birth date (8) + CPF (11) + NIS (11) + RG (15) + issuer (6).
func icpBrasilSAN(t *testing.T, cpf string) pkix.Extension {
	t.Helper()
	inner, err := asn1.Marshal("01011990" + cpf + "00000000000" + "000000000000000" + "SSPPI ")
	if err != nil {
		t.Fatal(err)
	}
	on := struct {
		TypeID asn1.ObjectIdentifier
		Value  asn1.RawValue
	}{oidICPBrasilHolderPF, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: inner}}
	onBytes, err := asn1.MarshalWithParams(on, "tag:0")
	if err != nil {
		t.Fatal(err)
	}
	san, err := asn1.Marshal([]asn1.RawValue{{FullBytes: onBytes}})
	if err != nil {
		t.Fatal(err)
	}
	return pkix.Extension{Id: oidSubjectAltName, Value: san}
}

func ecpfPFX(t *testing.T, cn string, ext []pkix.Extension) ([]byte, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtraExtensions: ext,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pfx, err := pkcs12.Modern2023.Encode(key, cert, nil, "pw")
	if err != nil {
		t.Fatal(err)
	}
	return pfx, cert
}

func TestHolderCPF_ReadsICPBrasilOtherName(t *testing.T) {
	_, cert := ecpfPFX(t, "FULANO DE TAL", []pkix.Extension{icpBrasilSAN(t, "12345678909")})
	if got := holderCPF(cert); got != "12345678909" {
		t.Fatalf("holderCPF = %q, want 12345678909", got)
	}
}

func TestHolderCPF_EmptyWithoutTheOID(t *testing.T) {
	_, cert := ecpfPFX(t, "EMPRESA LTDA:12345678000195", nil)
	if got := holderCPF(cert); got != "" {
		t.Fatalf("holderCPF = %q, want empty for an e-CNPJ", got)
	}
}

func TestCertificateHeldBy(t *testing.T) {
	bySAN, _ := ecpfPFX(t, "FULANO DE TAL", []pkix.Extension{icpBrasilSAN(t, "12345678909")})
	byCN, _ := ecpfPFX(t, "FULANO DE TAL:12345678909", nil)
	other, _ := ecpfPFX(t, "OUTRA PESSOA:98765432100", nil)

	if !certificateHeldBy(bySAN, "pw", "12345678909") {
		t.Error("SAN holder not matched")
	}
	if !certificateHeldBy(byCN, "pw", "12345678909") {
		t.Error("CN fallback not matched")
	}
	if certificateHeldBy(other, "pw", "12345678909") {
		t.Error("another person's e-CPF matched")
	}
	if certificateHeldBy(bySAN, "wrong-password", "12345678909") {
		t.Error("an unreadable PFX must not match")
	}
	if certificateHeldBy(bySAN, "pw", "") {
		t.Error("an empty CPF must match nothing")
	}
}
