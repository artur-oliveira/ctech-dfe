package ciot

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"testing"
	"time"
)

// testCert builds a throwaway certificate; holder "" omits the otherName.
func testCert(t *testing.T, holder string, eku []x509.ExtKeyUsage) *x509.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), ExtKeyUsage: eku,
	}
	if holder != "" {
		val, _ := asn1.MarshalWithParams(holder, "utf8")
		on, _ := asn1.Marshal(struct {
			ID    asn1.ObjectIdentifier
			Value asn1.RawValue `asn1:"tag:0,explicit"`
		}{oidICPBrasilID, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: val}})
		// retag the SEQUENCE as GeneralName [0] (constructed, implicit)
		on[0] = 0xA0
		san, _ := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: on})
		tpl.ExtraExtensions = []pkix.Extension{{Id: oidSAN, Value: san}}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCertificateHolderIDAndPreflight(t *testing.T) {
	client := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	c := testCert(t, cnpjA, client)
	if id, err := CertificateHolderID(c); err != nil || id != cnpjA {
		t.Fatalf("%q %v", id, err)
	}
	now := time.Now()
	if err := CheckCertificate(c, cnpjA, false, now); err != nil {
		t.Fatal(err)
	}
	if err := CheckCertificate(c, "52998224725", false, now); !errors.Is(err, ErrCertNotCarrier) {
		t.Fatalf("err %v", err)
	}
	if err := CheckCertificate(c, "52998224725", true, now); err != nil { // payment institution
		t.Fatal(err)
	}
	if err := CheckCertificate(testCert(t, "", client), "", false, now); !errors.Is(err, ErrCertNoOID) {
		t.Fatalf("err %v", err)
	}
	if err := CheckCertificate(testCert(t, cnpjA, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}), "", false, now); !errors.Is(err, ErrCertNoClient) {
		t.Fatalf("err %v", err)
	}
	if err := CheckCertificate(c, "", false, now.Add(2*time.Hour)); !errors.Is(err, ErrCertExpired) {
		t.Fatalf("err %v", err)
	}
}
