package ciot

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"time"
)

// Certificate preflight (spec sections 2 and 12): the organization's A1 must be the carrier's (or an IP's), carry
// the ICP-Brasil otherName OID 2.16.76.1.3.3 (CNPJ of the holder) and the client-authentication EKU.
var (
	oidSAN         = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidICPBrasilID = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3}
)

// Certificate configuration errors (alert, not business errors).
var (
	ErrCertNoOID      = errors.New("ciot: certificate has no otherName OID 2.16.76.1.3.3")
	ErrCertNoClient   = errors.New("ciot: certificate lacks the client authentication extended key usage")
	ErrCertNotCarrier = errors.New("ciot: certificate holder is not the carrier (B112)")
	ErrCertExpired    = errors.New("ciot: certificate is not valid now")
)

// otherName is GeneralName [0] { type-id OID, [0] EXPLICIT value }.
type otherName struct {
	ID    asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"tag:0,explicit"`
}

// CertificateHolderID returns the value of the OID 2.16.76.1.3.3 otherName (the holder's CNPJ), or ErrCertNoOID.
func CertificateHolderID(cert *x509.Certificate) (string, error) {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSAN) {
			continue
		}
		var seq asn1.RawValue
		if _, err := asn1.Unmarshal(ext.Value, &seq); err != nil {
			return "", fmt.Errorf("ciot: parse subjectAltName: %w", err)
		}
		rest := seq.Bytes
		for len(rest) > 0 {
			var gn asn1.RawValue
			var err error
			if rest, err = asn1.Unmarshal(rest, &gn); err != nil {
				return "", fmt.Errorf("ciot: parse subjectAltName: %w", err)
			}
			if gn.Class != asn1.ClassContextSpecific || gn.Tag != 0 {
				continue // not an otherName
			}
			var on otherName
			if _, err := asn1.Unmarshal(retagSequence(gn.FullBytes), &on); err != nil || !on.ID.Equal(oidICPBrasilID) {
				continue
			}
			var s string
			if _, err := asn1.Unmarshal(on.Value.Bytes, &s); err != nil {
				return "", fmt.Errorf("ciot: parse OID value: %w", err)
			}
			return NormalizeDoc(s), nil
		}
	}
	return "", ErrCertNoOID
}

// CheckCertificate enforces the preflight. carrierDoc is the CPF/CNPJ of the carrier (empty when unknown);
// paymentInstitution relaxes the carrier match, since an IP calls on behalf of others (B112).
func CheckCertificate(cert *x509.Certificate, carrierDoc string, paymentInstitution bool, now time.Time) error {
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return ErrCertExpired
	}
	client := false
	for _, u := range cert.ExtKeyUsage {
		client = client || u == x509.ExtKeyUsageClientAuth || u == x509.ExtKeyUsageAny
	}
	if len(cert.ExtKeyUsage) == 0 || !client {
		return ErrCertNoClient
	}
	id, err := CertificateHolderID(cert)
	if err != nil {
		return err
	}
	if !paymentInstitution && carrierDoc != "" && id != NormalizeDoc(carrierDoc) {
		return ErrCertNotCarrier
	}
	return nil
}

// retagSequence turns the implicit [0] constructed otherName back into a universal SEQUENCE for decoding.
func retagSequence(full []byte) []byte {
	out := append([]byte{}, full...)
	out[0] = 0x30
	return out
}
