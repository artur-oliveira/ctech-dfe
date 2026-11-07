package xmlops

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
)

// SignText returns Base64(RSA-PKCS1v15-SHA1(text)) with the issuer's key. It is the same primitive
// signer.go uses for SignatureValue (rsa-sha1 over a SHA-1 digest), applied to a plain string
// instead of a canonicalized SignedInfo; the C14N code is not involved. Used for the NFC-e QR Code
// v3 offline "assinatura" parameter (Manual DANFE NFC-e 4.4.2).
func SignText(text string, key *rsa.PrivateKey) (string, error) {
	digest := sha1.Sum([]byte(text)) //nolint:gosec // SEFAZ legacy requirement, see signer.go package doc.
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, digest[:])
	if err != nil {
		return "", fmt.Errorf("xmlops: sign text: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}
