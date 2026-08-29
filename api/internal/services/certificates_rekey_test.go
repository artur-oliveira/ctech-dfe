package services

import "testing"

// The certificate must belong to the company it is uploaded for. Under the old
// key this compared against the CNPJ_ prefix; a company id matches no prefix,
// so the function fell through to its final `return nil` and accepted ANY
// certificate for ANY company — a validation that fails open, silently.
func TestACertificateMustMatchTheExpectedDocument(t *testing.T) {
	info := &CertInfo{CNPJ: "11222333000181"}
	if err := MatchOrgDocument("11222333000181", info); err != nil {
		t.Fatalf("the matching certificate was refused: %v", err)
	}
	if err := MatchOrgDocument("99888777000166", info); err == nil {
		t.Fatal("another company's certificate was accepted")
	}
}

func TestACPFCertificateMustMatchToo(t *testing.T) {
	info := &CertInfo{CPF: "52998224725"}
	if err := MatchOrgDocument("52998224725", info); err != nil {
		t.Fatalf("the matching certificate was refused: %v", err)
	}
	if err := MatchOrgDocument("12345678909", info); err == nil {
		t.Fatal("another person's certificate was accepted")
	}
}

// A certificate that carries neither is not ours to judge — some carry only a
// CN. The check was always permissive there and stays so; what it must not do
// is be permissive because it could not read the key.
func TestACertificateWithNoDocumentIsNotRefused(t *testing.T) {
	if err := MatchOrgDocument("11222333000181", &CertInfo{}); err != nil {
		t.Fatalf("a certificate with no document was refused: %v", err)
	}
}

// An unknown expected document refuses everything. Accepting would be the
// fail-open this change exists to close: an issuer with no document on record
// must not be a company any certificate fits.
func TestAnUnknownExpectedDocumentRefuses(t *testing.T) {
	if err := MatchOrgDocument("", &CertInfo{CNPJ: "11222333000181"}); err == nil {
		t.Fatal("a certificate was accepted for a company with no document")
	}
}

// A CNPJ certificate for a CPF company, and the reverse. The old prefix check
// answered these by construction; the document check has to answer them itself.
func TestTheKindsMustNotCross(t *testing.T) {
	if err := MatchOrgDocument("52998224725", &CertInfo{CNPJ: "11222333000181"}); err == nil {
		t.Fatal("a CNPJ certificate was accepted for a CPF issuer")
	}
	if err := MatchOrgDocument("11222333000181", &CertInfo{CPF: "52998224725"}); err == nil {
		t.Fatal("a CPF certificate was accepted for a CNPJ issuer")
	}
}

// The path is built from the partition key, whatever era it belongs to, and is
// never parsed back. Existing objects keep the path they were written under and
// resolve through the certificate row's stored s3_key — which is why the S3
// side of this re-key needs no migration at all.
func TestCertificateS3KeyCarriesTheKeyWhicheverEra(t *testing.T) {
	const md5 = "d41d8cd98f00b204e9800998ecf8427e"
	if got := CertificateS3Key("0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70", md5); got !=
		"certs/0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70/"+md5+".pfx" {
		t.Errorf("company id path = %q", got)
	}
	if got := CertificateS3Key("CNPJ_11222333000181", md5); got !=
		"certs/CNPJ_11222333000181/"+md5+".pfx" {
		t.Errorf("legacy path = %q — an existing object would stop resolving", got)
	}
}
