// Package certificate ports py-dfe's certificate manager
// (py-dfe/py_dfe/certificate/manager.py) to Go: PFX/PKCS12 decode, private
// key extraction, and mTLS client construction for SEFAZ communication.
// Per py-dfe/CLAUDE.md ("Certificate Handling (MUST NOT simplify)"), the
// mTLS setup here is deliberate and must not be simplified or bypassed.
package certificate

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// Info holds certificate metadata callers may need for error messages (e.g.
// "certificate expired") — mirrors what py-dfe's manager exposes via its
// `certificate` property (a cryptography x509.Certificate, from which CN and
// validity dates are read by callers).
type Info struct {
	CN        string
	NotBefore time.Time
	NotAfter  time.Time
	IsExpired bool
}

// decode base64-decodes and PKCS12-decodes certificateB64/password into the
// leaf certificate, its RSA private key, and the CA chain. Shared by
// NewClient and ParseInfo so both go through the exact same load path.
func decode(certificateB64, password string) (*x509.Certificate, *rsa.PrivateKey, []*x509.Certificate, error) {
	pfxData, err := base64.StdEncoding.DecodeString(certificateB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("certificate: invalid base64: %w", err)
	}

	privateKey, cert, caCerts, err := pkcs12.DecodeChain(pfxData, password)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("certificate: failed to load PFX certificate: %w", err)
	}

	// py-dfe requires RSA (it signs with RSA-SHA1 downstream) — mirror that
	// here rather than letting a non-RSA key fail obscurely later.
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, nil, fmt.Errorf("certificate: private key must be RSA, got %T", privateKey)
	}

	return cert, rsaKey, caCerts, nil
}

// Load decodes a base64-encoded PFX/PKCS12 blob (as arrives in
// Request.CertificateB64) once and returns everything a SEFAZ caller needs
// from it: an *http.Client configured for mTLS, plus the leaf certificate
// and RSA private key themselves (needed separately for XML-DSig signing —
// see go-dfe/internal/xmlops/signer.go — which mTLS's tls.Certificate does
// not expose in usable form).
//
// InsecureSkipVerify is deliberate: SEFAZ's server certificate chain is not
// validated by design (Brazilian government PKI quirks) — mirrors py-dfe's
// ssl_context() (ctx.verify_mode = ssl.CERT_NONE, ctx.check_hostname = False).
// This is not a bug; do not "fix" it.
func Load(certificateB64, password string) (*http.Client, *x509.Certificate, *rsa.PrivateKey, error) {
	cert, rsaKey, caCerts, err := decode(certificateB64, password)
	if err != nil {
		return nil, nil, nil, err
	}

	rawCerts := make([][]byte, 0, 1+len(caCerts))
	rawCerts = append(rawCerts, cert.Raw)
	for _, ca := range caCerts {
		rawCerts = append(rawCerts, ca.Raw)
	}

	tlsCert := tls.Certificate{
		Certificate: rawCerts,
		PrivateKey:  rsaKey,
		Leaf:        cert,
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{tlsCert},
				Renegotiation:      tls.RenegotiateOnceAsClient,
				InsecureSkipVerify: true, // SEFAZ server chain deliberately not validated — see doc comment above.
			},
		},
	}
	return client, cert, rsaKey, nil
}

// NewClient is Load without the certificate/key — for callers that only
// need the mTLS http.Client (e.g. non-signing SEFAZ calls).
func NewClient(certificateB64, password string) (*http.Client, error) {
	client, _, _, err := Load(certificateB64, password)
	return client, err
}

// ParseInfo decodes the same PFX/PKCS12 blob as NewClient and returns its
// certificate metadata, without building an http.Client. Callers that need
// to report certificate errors (e.g. "certificate expired") can use this
// instead of reaching into a *tls.Config.
func ParseInfo(certificateB64, password string) (*Info, error) {
	cert, _, _, err := decode(certificateB64, password)
	if err != nil {
		return nil, err
	}

	return &Info{
		CN:        cert.Subject.CommonName,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		IsExpired: time.Now().UTC().After(cert.NotAfter),
	}, nil
}
