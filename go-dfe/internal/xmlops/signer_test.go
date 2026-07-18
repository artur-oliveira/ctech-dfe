package xmlops

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // matching signer.go's legacy SEFAZ algorithm, see its package doc.
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// C14N correctness: W3C Canonical XML 1.0 spec test vectors
// (https://www.w3.org/TR/2001/REC-xml-c14n-20010315/, section 3). Input and
// expected output below are transcribed verbatim from the spec's own HTML
// comments (the literal source text, not the display-formatted <code>
// blocks with &nbsp;/<br/> markup). Only the "uncommented"/no-comments
// canonical form is checked, since that's py-dfe's configuration
// (c14n_algorithm does not end in "#WithComments").
//
// 3.4 (Character Modifications), 3.5 (Entity References) and 3.7 (Document
// Subsets via XPath) are intentionally NOT ported: 3.4/3.5 rely on
// DTD-declared default attributes and internal/external entities, and 3.7
// relies on XPath node-set selection — this signer deliberately does not
// implement DTD/entity processing (fiscal XML never has a DOCTYPE, and
// signxml itself rejects DTDs outright, so parity doesn't require it). 3.3
// is ported below with its DOCTYPE-derived default-attribute injection
// (on <e9>) removed for the same reason, verified sub-element-by-sub-element
// instead of as a full document.
// ============================================================================

func TestC14N_3_1_OutsideDocumentElement(t *testing.T) {
	input := "<?xml version=\"1.0\"?>\n\n" +
		"<?xml-stylesheet   href=\"doc.xsl\"\n   type=\"text/xsl\"   ?>\n\n" +
		"<!DOCTYPE doc SYSTEM \"doc.dtd\">\n\n" +
		"<doc>Hello, world!<!-- Comment 1 --></doc>\n\n" +
		"<?pi-without-data     ?>\n\n" +
		"<!-- Comment 2 -->\n\n" +
		"<!-- Comment 3 -->\n"

	want := "<?xml-stylesheet href=\"doc.xsl\"\n   type=\"text/xsl\"   ?>\n" +
		"<doc>Hello, world!</doc>\n" +
		"<?pi-without-data?>"

	doc, err := parseDocument([]byte(input))
	if err != nil {
		t.Fatalf("parseDocument: %v", err)
	}
	got := string(canonicalizeDocument(doc))
	if got != want {
		t.Errorf("canonical form mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestC14N_3_2_WhitespaceInContent(t *testing.T) {
	input := "<doc>\n" +
		"   <clean>   </clean>\n" +
		"   <dirty>   A   B   </dirty>\n" +
		"   <mixed>\n" +
		"      A\n" +
		"      <clean>   </clean>\n" +
		"      B\n" +
		"      <dirty>   A   B   </dirty>\n" +
		"      C\n" +
		"   </mixed>\n" +
		"</doc>"

	// Per the spec: "the input document and canonical form are identical."
	want := input

	doc, err := parseDocument([]byte(input))
	if err != nil {
		t.Fatalf("parseDocument: %v", err)
	}
	got := string(canonicalizeDocument(doc))
	if got != want {
		t.Errorf("canonical form mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestC14N_3_3_StartAndEndTags ports the namespace/attribute-reordering
// half of spec example 3.3 (the DOCTYPE-driven default-attribute-injection
// half, on <e9>, is out of scope — see file doc comment). Each sub-element
// is verified independently via canonicalizeElement, which is valid here
// because <doc> itself (their real parent in the spec's full example)
// declares no namespaces and no attributes, so canonicalizing e5/e6 as
// their own standalone root is byte-identical to their contribution inside
// the full document's canonical form.
func TestC14N_3_3_StartAndEndTags(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty elements become start-end tag pairs",
			input: `<e1   />`,
			want:  `<e1></e1>`,
		},
		{
			name:  "already-open empty element unaffected",
			input: `<e2   ></e2>`,
			want:  `<e2></e2>`,
		},
		{
			name:  "attributes sorted lexicographically by local name",
			input: `<e3    name = "elem3"   id="elem3"    />`,
			want:  `<e3 id="elem3" name="elem3"></e3>`,
		},
		{
			name:  "attributes sorted lexicographically by local name (open form)",
			input: `<e4    name="elem4"   id="elem4"    ></e4>`,
			want:  `<e4 id="elem4" name="elem4"></e4>`,
		},
		{
			name: "namespace axis before attribute axis; namespace URI is the sort key, not prefix",
			input: "<e5 a:attr=\"out\" b:attr=\"sorted\" attr2=\"all\" attr=\"I'm\"\n" +
				"       xmlns:b=\"http://www.ietf.org\"\n" +
				"       xmlns:a=\"http://www.w3.org\"\n" +
				"       xmlns=\"http://example.org\"/>",
			want: `<e5 xmlns="http://example.org" xmlns:a="http://www.w3.org" xmlns:b="http://www.ietf.org" attr="I'm" attr2="all" b:attr="sorted" a:attr="out"></e5>`,
		},
		{
			name: "superfluous/redundant namespace declarations eliminated across nesting, including default-namespace undeclare",
			input: `<e6 xmlns="" xmlns:a="http://www.w3.org">` +
				`<e7 xmlns="http://www.ietf.org">` +
				`<e8 xmlns="" xmlns:a="http://www.w3.org">` +
				`<e9 xmlns="" xmlns:a="http://www.ietf.org"/>` +
				`</e8></e7></e6>`,
			want: `<e6 xmlns:a="http://www.w3.org">` +
				`<e7 xmlns="http://www.ietf.org">` +
				`<e8 xmlns="">` +
				`<e9 xmlns:a="http://www.ietf.org"></e9>` +
				`</e8></e7></e6>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := parseDocument([]byte(tc.input))
			if err != nil {
				t.Fatalf("parseDocument: %v", err)
			}
			got := string(canonicalizeElement(doc.documentElement()))
			if got != tc.want {
				t.Errorf("canonical form mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestC14N_EscapingRules exercises the character-escaping half of spec
// example 3.4 (Character Modifications) without the DTD-dependent parts
// (normId/normNames/ATTLIST-based attribute typing, entity references) —
// those require DTD processing this signer intentionally doesn't
// implement (see file doc comment). Covers: &, <, > in text; &, <, ", tab,
// LF, CR in attribute values; CR surviving via character reference.
func TestC14N_EscapingRules(t *testing.T) {
	input := `<doc><a>1 &lt; 2 &amp; 2 &gt; 1</a><b attr="&lt;&amp;&quot;&#9;&#10;&#13;end"/></doc>`
	want := `<doc><a>1 &lt; 2 &amp; 2 &gt; 1</a><b attr="&lt;&amp;&quot;&#x9;&#xA;&#xD;end"></b></doc>`

	doc, err := parseDocument([]byte(input))
	if err != nil {
		t.Fatalf("parseDocument: %v", err)
	}
	got := string(canonicalizeElement(doc.documentElement()))
	if got != want {
		t.Errorf("canonical form mismatch\n got: %q\nwant: %q", got, want)
	}
}

// TestC14N_LiteralAttributeWhitespaceNormalized checks XML 1.0 §3.3.3
// attribute-value normalization: a literal (non-referenced) tab/newline in
// an attribute value collapses to a single space, while the *same*
// character produced via a numeric character reference does not (this is
// the distinction spec example 3.4 is built around).
func TestC14N_LiteralAttributeWhitespaceNormalized(t *testing.T) {
	input := "<doc a=\"x\ty\nz\" b=\"x&#9;y&#10;z\"/>"
	want := `<doc a="x y z" b="x&#x9;y&#xA;z"></doc>`

	doc, err := parseDocument([]byte(input))
	if err != nil {
		t.Fatalf("parseDocument: %v", err)
	}
	got := string(canonicalizeElement(doc.documentElement()))
	if got != want {
		t.Errorf("canonical form mismatch\n got: %q\nwant: %q", got, want)
	}
}

// ============================================================================
// X509Certificate newline fix (py-dfe's _fix_x509_newlines): the Python
// implementation regex-strips '\n' and ' ' from the X509Certificate text
// node after the fact, because signxml sets that text to a PEM-derived
// string that still contains the 64-column line wrapping. This Go port
// avoids ever introducing them (base64-encodes cert.Raw directly as one
// line) but applies the same stripping defensively; this test pins what
// that stripping actually does, matching the Python regex's exact behavior
// (strip '\n' and ' ' only — nothing else).
// ============================================================================

func TestStripCertWhitespace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"MIIC\ntSCCAZ\n2gAwIB", "MIICtSCCAZ2gAwIB"},
		{"MIIC tSCC AZ2gAwIB", "MIICtSCCAZ2gAwIB"},
		{"MIIC\n tSCC\nAZ2 gAwIB", "MIICtSCCAZ2gAwIB"},
		{"already-clean", "already-clean"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := stripCertWhitespace(tc.in); got != tc.want {
			t.Errorf("stripCertWhitespace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ============================================================================
// Byte-identical cross-check against the real `signxml` Python library.
//
// The key/cert/input/expected-output below were produced once, outside this
// test, by running py-dfe's *actual* signing code path — `_SefazXMLSigner`
// (signature_algorithm="rsa-sha1", digest_algorithm="sha1",
// c14n_algorithm=REC-xml-c14n-20010315) plus `_fix_x509_newlines`, copied
// verbatim from py-dfe/py_dfe/xmlops/signer.py — against signxml 5.1.0 (the
// version family pinned by py-dfe's pyproject.toml) and a locally generated
// self-signed test RSA certificate/key (never a real customer certificate).
//
// This is a genuine cross-implementation check, not self-consistency: it
// independently exercises the real upstream Python signing library this
// port replaces. It is NOT the plan's official gate (a captured production
// py-dfe Lambda run against a dedicated test certificate, compared
// byte-for-byte across a real document corpus) — that corpus does not exist
// yet. See this file's package doc for the same caveat.
// ============================================================================

const fixtureKeyPEM = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDrvWE/pqsNeimD
tC14AE9FcheDJfE5vKd+2/qPzhOGJf/kQJFqVTy8cw6P8sNr0koEoCIFEU14wVvQ
2zzfSKKTdGbwSuFFCTxgduvudNAWRkaiRHwi3LuNGohMuVTtVcOWOz5wxpuO8lpy
oDv1UTJqRKiqEKkT69HWd3bL1YhZJSiV6chnY38jNeqWL+IyNFOPHJKdpAiBP+rx
Meyy5iRgnLqC20yMZ3+zJoB/QoCQ0T9eGLvUDPAxDhEuI/uSWT143OY12QNO9trQ
hLu4PrlfnC7hGL0J22/v2Bi2vBH+g3MvwBnYazhYy2pl15nVBg/C++CpS1uSXyPN
Vkr7QjzTAgMBAAECggEAPs3xfre0kp6dOM1j37iVZfcDdJlDLxKnvRB2LKHGadLt
3a2mECItUDeHBaqzjaI0vg67gYYekbFR+M6v5PzA82/rjNEmOvI+96Q3LwxH8+c9
IjYErHUKMomDDo3BpolW1ktqUzlWcDr5BdjSoITFXbJ1DPnrUbdd7DlfmOaGsNjG
eeYCFtCtx3IFd1CptHKryAFmO+Fxi4rJEdh6LYf2O+R763Lyd+PhKDv+shEltIYn
SDuGp8COoUUCnsI/Jm4h41vH5KVwEMZRPizNb98EvJQyDOSeIr9w8i4fkWxi/PdD
EiXQXXZvGqkT0DKe3rmzfMmtm+r/ioR9y89f+PPMgQKBgQD/2eeqw/SsjJIuRPxW
XMuNQ9wBinzzF00rkx5MityWWD/rgB4RT52LO6YGH1xextXxSqQ+KvohOlvV/6O2
FCs1CX1bX4z7QAyXjZZ+9nc2xKWiPH6KOCMry5og03o4OhzkRB7oROXxyTol8dQB
q+xdlcriiq59h8t1R3jReLOwYwKBgQDr4Hr9fK81s8puHo5+rhwAI//JPbxwp/5K
0vCrvUQW3IMq2t/7U+upkjqGuBu9aswnkVT0UQ3J1Z+zLfXkXH8ylNM1NEpwAmNX
i+r1bkHmfUX6tb9HS/p8EXz9WUIjStkoFtcsA+iPF78JFRXZAYEehay0nBx+ex9g
oizUO/KU0QKBgQDOxWeSPd3u2YiGdmBM15/2IgKbCDZlK87FSZeyGoOdyeKWzCsA
qIxVazaJOi0nt6BN6poEWC1gT07LC1henbwxl+LExtskbyX+EYKwRzYfgBuwmx1V
TXs3OMvufZsH+AdDf75OzufbWVpyMhe55h0XoSifn57Xeri2prWA7QCjqwKBgDrq
/Y2nwVQWrq/G7izybIgUdeXch99T9w7VlcwwIHvdZN4lgeETW0AmCHxyLGup64jO
onvMazdJJvTovAzoldUam48kmptT3WCW0H+xpMBf9kTjdP3oGo83BxN5Yi3Sml+L
JQAXkdV8RvmLzMNBvvDSzwrmG6/0LShEGhKBTtyhAoGAQ2Lj4sGNZaoBpEU2d9mw
DngVsM46P0nefOswQ4LX0+0gaDJhJFgRy1Fv+P8iSZSCeEePqvZ3Zdj9YeiLZ+ZA
/41AsNd/M1p1q6NJK3bo9FDoyKqNCSStLnZGyLw9hON9XKNhK8Kzx0Xg/Z2MVmvV
03vzi2pZI4blvOYtYGRbEkM=
-----END PRIVATE KEY-----`

const fixtureCertPEM = `-----BEGIN CERTIFICATE-----
MIICtTCCAZ2gAwIBAgIBATANBgkqhkiG9w0BAQsFADAeMRwwGgYDVQQDDBNURVNU
RSBHTy1ERkUgU0lHTkVSMB4XDTI2MDEwMTAwMDAwMFoXDTMwMDEwMTAwMDAwMFow
HjEcMBoGA1UEAwwTVEVTVEUgR08tREZFIFNJR05FUjCCASIwDQYJKoZIhvcNAQEB
BQADggEPADCCAQoCggEBAOu9YT+mqw16KYO0LXgAT0VyF4Ml8Tm8p37b+o/OE4Yl
/+RAkWpVPLxzDo/yw2vSSgSgIgURTXjBW9DbPN9IopN0ZvBK4UUJPGB26+500BZG
RqJEfCLcu40aiEy5VO1Vw5Y7PnDGm47yWnKgO/VRMmpEqKoQqRPr0dZ3dsvViFkl
KJXpyGdjfyM16pYv4jI0U48ckp2kCIE/6vEx7LLmJGCcuoLbTIxnf7MmgH9CgJDR
P14Yu9QM8DEOES4j+5JZPXjc5jXZA0722tCEu7g+uV+cLuEYvQnbb+/YGLa8Ef6D
cy/AGdhrOFjLamXXmdUGD8L74KlLW5JfI81WSvtCPNMCAwEAATANBgkqhkiG9w0B
AQsFAAOCAQEASEUPNem4b0h2Hg+VuF6hSzYM/Qc2G4TtqMfj3aUXU00or8cS5ChZ
aGFOMI3b00wxTzqL4z8/pmrZNtz5y2npSu9jXavO9FW3+Fmh1Ck5tm+V+xWMwG9L
epGHBgYGip/5q1e4aOesYeVmqhqogsLvXZScwxXz8AYHlSCOq3oNdhLMHKmmhG3O
yjVFCIFGReKU9eEPbpRATeFeYX3B6chj1C0qfWnhUxeWb2rHxOUVoLsecBhVHJ/y
/xqeDr9mLXiFPELw9blN9GN0qqoAIIaIwi955lAMZ8oJ4X091B5eAt6/fyn/yKhB
apBtp4Zri+0lHp48btYLeZ9/1CrpZrZnqA==
-----END CERTIFICATE-----`

const fixtureSampleXML = `<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe35240114200166000187550010000000011000000015" versao="4.00"><ide><cUF>35</cUF><natOp>Venda</natOp></ide><emit><CNPJ>14200166000187</CNPJ><xNome>EMPRESA TESTE LTDA</xNome></emit></infNFe></NFe>`

// Captured by running py-dfe's real _SefazXMLSigner (signxml 5.1.0) against
// fixtureSampleXML with fixtureKeyPEM/fixtureCertPEM and
// reference_id="NFe35240114200166000187550010000000011000000015" — see
// this file's doc comment above.
const fixtureSignedXML = `<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe35240114200166000187550010000000011000000015" versao="4.00"><ide><cUF>35</cUF><natOp>Venda</natOp></ide><emit><CNPJ>14200166000187</CNPJ><xNome>EMPRESA TESTE LTDA</xNome></emit></infNFe><Signature xmlns="http://www.w3.org/2000/09/xmldsig#"><SignedInfo><CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/><SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"/><Reference URI="#NFe35240114200166000187550010000000011000000015"><Transforms><Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature"/><Transform Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/></Transforms><DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/><DigestValue>jzKpWDR3GD7NgQc3G9X9J0IBMrY=</DigestValue></Reference></SignedInfo><SignatureValue>ywlA5JxIWuXoU+m9LI1Mw0miabaltf7hQc2lQWquHEW6pMVylD7TpB3qRqVU2koVDRS1E3LTNJ2Aullm5/eEbUcGR/DvmX17Aqu4sHnyMCFp9KNLB/1QdlMZWsxiFf8ECXqfTU87FiQQWdAG22lDyOrkaySdebINzrkQlI54A+S2ZhJFakG3EfLYmIU1WIAYsteI402b4BMxq8mhR+/NyUG+bPcIQ5c5W/zhv7nAvGefrtPArU9XbSdCHUFVYPxvlIbZQIu/1W/Ue3ZaDJQvJItxW63U8rSgPm1rxgflRHpKLEbAI3ddbuNjI3kQHZdhMKrswNkrY3tmyyy7xSQ07g==</SignatureValue><KeyInfo><X509Data><X509Certificate>MIICtTCCAZ2gAwIBAgIBATANBgkqhkiG9w0BAQsFADAeMRwwGgYDVQQDDBNURVNURSBHTy1ERkUgU0lHTkVSMB4XDTI2MDEwMTAwMDAwMFoXDTMwMDEwMTAwMDAwMFowHjEcMBoGA1UEAwwTVEVTVEUgR08tREZFIFNJR05FUjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAOu9YT+mqw16KYO0LXgAT0VyF4Ml8Tm8p37b+o/OE4Yl/+RAkWpVPLxzDo/yw2vSSgSgIgURTXjBW9DbPN9IopN0ZvBK4UUJPGB26+500BZGRqJEfCLcu40aiEy5VO1Vw5Y7PnDGm47yWnKgO/VRMmpEqKoQqRPr0dZ3dsvViFklKJXpyGdjfyM16pYv4jI0U48ckp2kCIE/6vEx7LLmJGCcuoLbTIxnf7MmgH9CgJDRP14Yu9QM8DEOES4j+5JZPXjc5jXZA0722tCEu7g+uV+cLuEYvQnbb+/YGLa8Ef6Dcy/AGdhrOFjLamXXmdUGD8L74KlLW5JfI81WSvtCPNMCAwEAATANBgkqhkiG9w0BAQsFAAOCAQEASEUPNem4b0h2Hg+VuF6hSzYM/Qc2G4TtqMfj3aUXU00or8cS5ChZaGFOMI3b00wxTzqL4z8/pmrZNtz5y2npSu9jXavO9FW3+Fmh1Ck5tm+V+xWMwG9LepGHBgYGip/5q1e4aOesYeVmqhqogsLvXZScwxXz8AYHlSCOq3oNdhLMHKmmhG3OyjVFCIFGReKU9eEPbpRATeFeYX3B6chj1C0qfWnhUxeWb2rHxOUVoLsecBhVHJ/y/xqeDr9mLXiFPELw9blN9GN0qqoAIIaIwi955lAMZ8oJ4X091B5eAt6/fyn/yKhBapBtp4Zri+0lHp48btYLeZ9/1CrpZrZnqA==</X509Certificate></X509Data></KeyInfo></Signature></NFe>`

func loadFixtureCertKey(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	keyBlock, _ := pem.Decode([]byte(fixtureKeyPEM))
	if keyBlock == nil {
		t.Fatal("failed to decode fixture private key PEM")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey: %v", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("fixture key is not RSA: %T", keyAny)
	}

	certBlock, _ := pem.Decode([]byte(fixtureCertPEM))
	if certBlock == nil {
		t.Fatal("failed to decode fixture certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert, key
}

func TestSignByteIdenticalToSignxml(t *testing.T) {
	cert, key := loadFixtureCertKey(t)

	got, err := Sign([]byte(fixtureSampleXML), ".//{http://www.portalfiscal.inf.br/nfe}infNFe", cert, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if string(got) != fixtureSignedXML {
		t.Errorf("signed output does not match signxml-produced fixture\n got: %s\nwant: %s", got, fixtureSignedXML)
	}
}

// ============================================================================
// End-to-end sign+verify round trip, independent of the signxml fixture:
// generates its own self-signed test RSA certificate, signs a sample
// fiscal-shaped XML fragment, then independently re-derives the digest by
// re-canonicalizing the signed document's own infNFe element and verifies
// the embedded SignatureValue with Go's own rsa.VerifyPKCS1v15 — proving
// the signer is internally self-consistent (digest, C14N and signature all
// agree with each other) even without an external corpus.
// ============================================================================

func generateSelfSignedCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "go-dfe test signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert, key
}

func TestSignVerifyRoundTrip(t *testing.T) {
	cert, key := generateSelfSignedCert(t)

	const refID = "NFe41240114200166000187550010000000011000000099"
	const idXPath = ".//{http://www.portalfiscal.inf.br/nfe}infNFe"
	input := `<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="` + refID +
		`" versao="4.00"><ide><cUF>41</cUF><natOp>Venda de mercadoria &amp; servico</natOp></ide></infNFe></NFe>`

	signed, err := Sign([]byte(input), idXPath, cert, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	doc, err := parseDocument(signed)
	if err != nil {
		t.Fatalf("parseDocument(signed output): %v", err)
	}
	root := doc.documentElement()

	// 1. Structural shape: Signature is a sibling of infNFe, appended last,
	// under NFe — the enveloped placement this package's doc comment claims.
	if len(root.children) != 2 {
		t.Fatalf("expected NFe to have 2 children (infNFe, Signature), got %d", len(root.children))
	}
	infNFe := root.children[0]
	sigEl := root.children[1]
	if infNFe.uri != "http://www.portalfiscal.inf.br/nfe" || infNFe.local != "infNFe" {
		t.Fatalf("first child is not infNFe: %+v", infNFe)
	}
	if sigEl.uri != dsigNS || sigEl.local != "Signature" {
		t.Fatalf("second (last) child is not Signature: %+v", sigEl)
	}

	// 2. Re-derive the Reference digest independently by re-canonicalizing
	// the (still-embedded) infNFe element the same way buildSignature did,
	// and compare against the DigestValue actually embedded in the output.
	wantDigest := sha1.Sum(canonicalizeElement(materialize(infNFe))) //nolint:gosec
	wantDigestB64 := base64.StdEncoding.EncodeToString(wantDigest[:])

	signedInfo := findChild(t, sigEl, "SignedInfo")
	reference := findChild(t, signedInfo, "Reference")
	digestValueEl := findChild(t, reference, "DigestValue")
	gotDigestB64 := textOf(digestValueEl)
	if gotDigestB64 != wantDigestB64 {
		t.Errorf("DigestValue mismatch\n got: %s\nwant: %s", gotDigestB64, wantDigestB64)
	}

	// 3. Verify the SignatureValue against SignedInfo's own canonical form,
	// using Go's own rsa.VerifyPKCS1v15 (not the code path used to produce
	// it) plus the public key from the certificate — proving the signature
	// was produced correctly from what SignedInfo actually canonicalizes to.
	// materialize is needed here too: SignedInfo's xmlns is redundant with
	// its Signature ancestor's, so it is elided from the final
	// non-canonical output we just re-parsed (see buildSignature's comment)
	// — re-canonicalizing it standalone needs that declaration restored,
	// same as any other element extracted from a larger parsed document.
	signedInfoC14N := canonicalizeElement(materialize(signedInfo))
	signedInfoDigest := sha1.Sum(signedInfoC14N) //nolint:gosec

	sigValueEl := findChild(t, sigEl, "SignatureValue")
	sigBytes, err := base64.StdEncoding.DecodeString(textOf(sigValueEl))
	if err != nil {
		t.Fatalf("decode SignatureValue: %v", err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("cert public key is not RSA: %T", cert.PublicKey)
	}
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA1, signedInfoDigest[:], sigBytes); err != nil {
		t.Errorf("rsa.VerifyPKCS1v15 failed: %v", err)
	}

	// 4. X509Certificate embeds the certificate's raw DER, base64-encoded,
	// with no whitespace (the newline-fix behavior, see
	// TestStripCertWhitespace for the isolated unit test of the helper).
	x509CertEl := findChild(t, findChild(t, findChild(t, sigEl, "KeyInfo"), "X509Data"), "X509Certificate")
	certB64 := textOf(x509CertEl)
	if strings.ContainsAny(certB64, "\n ") {
		t.Errorf("X509Certificate text contains whitespace: %q", certB64)
	}
	decodedCert, err := base64.StdEncoding.DecodeString(certB64)
	if err != nil {
		t.Fatalf("decode X509Certificate: %v", err)
	}
	if !bytes.Equal(decodedCert, cert.Raw) {
		t.Errorf("X509Certificate does not decode back to the original certificate DER")
	}
}

func findChild(t *testing.T, parent *xNode, local string) *xNode {
	t.Helper()
	for _, c := range parent.children {
		if c.kind == kindElement && c.local == local {
			return c
		}
	}
	t.Fatalf("no <%s> child found under <%s>", local, parent.qualifiedName())
	return nil
}

func textOf(el *xNode) string {
	var b strings.Builder
	for _, c := range el.children {
		if c.kind == kindText {
			b.WriteString(c.text)
		}
	}
	return b.String()
}
