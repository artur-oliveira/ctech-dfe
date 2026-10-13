package services

import (
	"crypto/x509"
	"encoding/asn1"
)

var (
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	// oidICPBrasilHolderPF is the ICP-Brasil "dados do titular pessoa física"
	// otherName: birth date (8 digits) followed by the CPF (11 digits).
	oidICPBrasilHolderPF = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 1}
)

const (
	icpBirthDateLen = 8
	cpfLen          = 11
)

// holderCPF returns the CPF of an e-CPF's holder, read from the SubjectAltName
// otherName 2.16.76.1.3.1, or "" when the certificate carries none.
func holderCPF(cert *x509.Certificate) string {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}
		var names []asn1.RawValue
		if _, err := asn1.Unmarshal(ext.Value, &names); err != nil {
			return ""
		}
		for _, n := range names {
			if n.Class != asn1.ClassContextSpecific || n.Tag != 0 {
				continue
			}
			var on struct {
				TypeID asn1.ObjectIdentifier
				Value  asn1.RawValue // [0] EXPLICIT wrapper around the string
			}
			if _, err := asn1.UnmarshalWithParams(n.FullBytes, &on, "tag:0"); err != nil || !on.TypeID.Equal(oidICPBrasilHolderPF) {
				continue
			}
			var inner asn1.RawValue
			if _, err := asn1.Unmarshal(on.Value.Bytes, &inner); err != nil {
				continue
			}
			digits := stripNonDigits(string(inner.Bytes))
			if len(digits) >= icpBirthDateLen+cpfLen {
				return digits[icpBirthDateLen : icpBirthDateLen+cpfLen]
			}
		}
	}
	return ""
}

// certificateHeldBy reports whether pfx is an e-CPF of cpf (D14). A PFX that
// cannot be opened matches nothing. The CPF is compared in memory only.
func certificateHeldBy(pfx []byte, password, cpf string) bool {
	if cpf == "" {
		return false
	}
	cert, _, info, err := ParsePFX(pfx, password)
	if err != nil {
		return false
	}
	return holderCPF(cert) == cpf || info.CPF == cpf
}

func stripNonDigits(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
