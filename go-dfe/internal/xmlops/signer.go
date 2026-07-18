// Package xmlops ports py-dfe's XML-DSig signer (py-dfe/py_dfe/xmlops/signer.py)
// to Go. This is the highest-risk file in the go-dfe migration (see
// docs/plans/2026-07-17-go-dfe-migration.md, "o arquivo mais arriscado de
// todo o projeto"): SEFAZ requires a signature built from RSA-SHA1 + SHA1
// digest + plain (non-exclusive) Canonical XML 1.0 (REC-xml-c14n-20010315),
// which no maintained Go library implements (goxmldsig targets exclusive
// C14N + modern algorithms and is a poor fit — see the migration plan for
// why it was rejected). This file hand-ports the exact behavior of py-dfe's
// `_SefazXMLSigner` (a subclass of the Python `signxml` library configured
// with signature_algorithm="rsa-sha1", digest_algorithm="sha1",
// c14n_algorithm=".../REC-xml-c14n-20010315") plus its `_fix_x509_newlines`
// post-processing step, including a hand-written Canonical XML 1.0
// implementation (there is no Go standard-library or well-maintained
// third-party equivalent).
//
// Confidence note (see go-dfe migration plan, "Gate de assinatura"): this
// implementation has been verified against the real `signxml` 5.1.0 Python
// library (the version family pinned by py-dfe's pyproject.toml,
// `signxml>=4.4.0`), configured identically to py-dfe's `_SefazXMLSigner`,
// using a locally generated test RSA key/certificate — reproduced
// independently while building this file and pinned as a fixture in
// signer_test.go (TestSignByteIdenticalToSignxml). That is NOT the same as
// the plan's official gate ("captura output assinado do py-dfe pra corpus
// de documentos reais... compara byte a byte"), which requires a captured
// production py-dfe Lambda run against a dedicated test certificate and
// does not exist yet. Treat this file as passing a strong independent
// cross-check against the upstream signing library, not as having cleared
// the plan's formal byte-identical gate.
package xmlops

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SEFAZ legacy requirement (RSA-SHA1), not a mistake — see package doc.
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Namespace/algorithm URIs used by the signature, matching py-dfe's
// _SefazXMLSigner configuration (py-dfe/py_dfe/xmlops/signer.py) exactly.
const (
	dsigNS = "http://www.w3.org/2000/09/xmldsig#"

	c14nAlgorithmURI      = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"
	envelopedTransformURI = "http://www.w3.org/2000/09/xmldsig#enveloped-signature"
	rsaSHA1SignatureURI   = "http://www.w3.org/2000/09/xmldsig#rsa-sha1"
	sha1DigestURI         = "http://www.w3.org/2000/09/xmldsig#sha1"

	idAttrLocal = "Id"
)

// ============================================================================
// Public API
// ============================================================================

// Sign parses xmlDoc, locates every element matched by idXPath — a
// restricted XPath subset in the ".//{namespaceURI}localName" form used by
// py-dfe's ServiceConfig.sign_id_xpath (py-dfe/py_dfe/services/config.py,
// e.g. ".//{http://www.portalfiscal.inf.br/nfe}infNFe") — and appends an
// enveloped XML-DSig <Signature> as the last child of each matched
// element's parent. This mirrors py-dfe's SefazClient._sign
// (py-dfe/py_dfe/services/base.py): the Reference URI points at the
// matched element's "Id" attribute, but the <Signature> itself is inserted
// as a sibling of that element (under its parent), not inside it — this is
// the standard NFe/CTe/MDFe convention
// (<NFe><infNFe Id="...">...</infNFe><Signature>...</Signature></NFe>),
// verified against the real signxml library's enveloped-signature
// placement, not assumed.
//
// If idXPath matches nothing (or is empty), the whole document element is
// signed in place, matching py-dfe's fallback (`targets = [root]`).
func Sign(xmlDoc []byte, idXPath string, cert *x509.Certificate, key *rsa.PrivateKey) ([]byte, error) {
	doc, err := parseDocument(xmlDoc)
	if err != nil {
		return nil, fmt.Errorf("xmlops: parse: %w", err)
	}
	root := doc.documentElement()
	if root == nil {
		return nil, errors.New("xmlops: document has no root element")
	}

	targets := findByClarkPath(root, idXPath)
	if len(targets) == 0 {
		targets = []*xNode{root}
	}

	for _, target := range targets {
		refID := target.getAttr("", idAttrLocal)

		sigEl, err := buildSignature(target, refID, cert, key)
		if err != nil {
			return nil, err
		}

		if target.parent == nil {
			// target is the document root itself: sign in place (Signature
			// becomes the last child of the root element).
			target.appendChild(sigEl)
			continue
		}
		target.parent.appendChild(sigEl)
	}

	return serializeDocument(doc), nil
}

// buildSignature builds the <Signature> element for the given target
// element (the element whose "Id" attribute equals refID), matching the
// structure produced by signxml's XMLSigner._build_sig + _add_key_info for
// py-dfe's configuration: method=enveloped, signature_algorithm=rsa-sha1,
// digest_algorithm=sha1, c14n_algorithm=REC-xml-c14n-20010315, with the
// Signature/SignedInfo elements in the default (unprefixed) ds namespace
// (py-dfe's _SefazXMLSigner.__init__ sets `self.namespaces = {None: ds}`).
func buildSignature(target *xNode, refID string, cert *x509.Certificate, key *rsa.PrivateKey) (*xNode, error) {
	// Digest: C14N of the referenced element itself (not its parent), as an
	// independent standalone tree — see materialize() doc comment for why
	// this step (rather than canonicalizing the element in place inside
	// the larger document) is required for byte-identical output.
	standalone := materialize(target)
	digestInput := canonicalizeElement(standalone)
	digest := sha1.Sum(digestInput) //nolint:gosec // SEFAZ legacy requirement, see package doc.

	reference := newElem("Reference")
	reference.attrs = []attrNode{{local: "URI", value: "#" + refID}}
	transforms := newElem("Transforms")
	transforms.appendChild(algorithmElem("Transform", envelopedTransformURI))
	transforms.appendChild(algorithmElem("Transform", c14nAlgorithmURI))
	reference.appendChild(transforms)
	reference.appendChild(algorithmElem("DigestMethod", sha1DigestURI))
	digestValue := newElem("DigestValue")
	digestValue.appendChild(newText(base64.StdEncoding.EncodeToString(digest[:])))
	reference.appendChild(digestValue)

	signedInfo := newElem("SignedInfo")
	// signxml passes nsmap=self.namespaces explicitly when creating
	// SignedInfo (in addition to Signature) — physically redundant with
	// the parent's declaration, but it changes how SignedInfo canonicalizes
	// as the standalone top of its own node-set (see canonicalizeElement
	// call below); it does not appear in the final non-canonical output
	// because that declaration is redundant with the ancestor's (verified
	// against real signxml output).
	signedInfo.nsDecls = []nsDecl{{prefix: "", uri: dsigNS}}
	signedInfo.appendChild(algorithmElem("CanonicalizationMethod", c14nAlgorithmURI))
	signedInfo.appendChild(algorithmElem("SignatureMethod", rsaSHA1SignatureURI))
	signedInfo.appendChild(reference)

	signedInfoC14N := canonicalizeElement(signedInfo)
	signedInfoDigest := sha1.Sum(signedInfoC14N) //nolint:gosec // SEFAZ legacy requirement, see package doc.
	signature, err := rsa.SignPKCS1v15(nil, key, crypto.SHA1, signedInfoDigest[:])
	if err != nil {
		return nil, fmt.Errorf("xmlops: sign SignedInfo: %w", err)
	}

	signatureValue := newElem("SignatureValue")
	signatureValue.appendChild(newText(base64.StdEncoding.EncodeToString(signature)))

	// _fix_x509_newlines (py-dfe/py_dfe/xmlops/signer.py) strips embedded
	// newlines/spaces that signxml's PEM-derived X509Certificate text
	// otherwise contains (cert.public_bytes(Encoding.PEM) wraps at 64
	// chars). We never introduce them: base64-encode the raw DER cert
	// bytes directly as a single line. stripCertWhitespace is applied
	// anyway as a defensive equivalent of that fix (see signer_test.go).
	x509Certificate := newElem("X509Certificate")
	x509Certificate.appendChild(newText(stripCertWhitespace(base64.StdEncoding.EncodeToString(cert.Raw))))
	x509Data := newElem("X509Data")
	x509Data.appendChild(x509Certificate)
	keyInfo := newElem("KeyInfo")
	keyInfo.appendChild(x509Data)

	sig := newElem("Signature")
	sig.nsDecls = []nsDecl{{prefix: "", uri: dsigNS}}
	sig.appendChild(signedInfo)
	sig.appendChild(signatureValue)
	sig.appendChild(keyInfo)
	return sig, nil
}

func algorithmElem(local, algorithm string) *xNode {
	el := newElem(local)
	el.attrs = []attrNode{{local: "Algorithm", value: algorithm}}
	return el
}

// stripCertWhitespace mirrors py-dfe's _fix_x509_newlines regex
// (`.replace("\n", "").replace(" ", "")` on the X509Certificate text node):
// strips newlines and spaces from a base64 string. Applied defensively even
// though our own base64 encoding never introduces them.
func stripCertWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// findByClarkPath finds descendant elements of root matching xpath, a
// restricted subset of the ".//{namespaceURI}localName" form used
// throughout py-dfe/py_dfe/services/config.py (ServiceConfig.sign_id_xpath).
// Only descendants are matched (root itself is excluded), matching
// Python's `root.findall(xpath)` semantics for this xpath shape.
func findByClarkPath(root *xNode, xpath string) []*xNode {
	uri, local, ok := parseClarkPath(xpath)
	if !ok {
		return nil
	}
	var out []*xNode
	var walk func(n *xNode)
	walk = func(n *xNode) {
		for _, c := range n.children {
			if c.kind != kindElement {
				continue
			}
			if c.uri == uri && c.local == local {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(root)
	return out
}

// parseClarkPath parses ".//{namespaceURI}localName" into its parts.
func parseClarkPath(xpath string) (uri, local string, ok bool) {
	const prefix = ".//{"
	if !strings.HasPrefix(xpath, prefix) {
		return "", "", false
	}
	rest := xpath[len(prefix):]
	end := strings.IndexByte(rest, '}')
	if end < 0 || end+1 >= len(rest) {
		return "", "", false
	}
	return rest[:end], rest[end+1:], true
}

// ============================================================================
// Node model
// ============================================================================

type nodeKind uint8

const (
	kindDocument nodeKind = iota
	kindElement
	kindText
	kindComment
	kindPI
)

// nsDecl is a namespace declaration physically present on an element, i.e.
// an `xmlns="uri"` (prefix == "") or `xmlns:prefix="uri"` attribute. uri ==
// "" is a valid, meaningful value: it undeclares the default namespace.
type nsDecl struct {
	prefix string
	uri    string
}

// attrNode is a regular (non-namespace-declaring) attribute. Per XML
// Namespaces, an attribute with no prefix is never in a namespace even if a
// default xmlns is in scope, so uri is "" for unprefixed attributes.
type attrNode struct {
	prefix string
	uri    string
	local  string
	value  string
}

func (a attrNode) qualifiedName() string {
	if a.prefix == "" {
		return a.local
	}
	return a.prefix + ":" + a.local
}

// xNode is a node in the parsed XML tree. Elements carry both the resolved
// namespace URI (uri) and the literal prefix used in the source (prefix) —
// Canonical XML 1.0 never rewrites prefixes, so the original prefix must be
// retained verbatim for output, while uri is needed to resolve
// Reference/xpath lookups and for C14N's expanded-name attribute sort.
type xNode struct {
	kind     nodeKind
	prefix   string
	uri      string
	local    string // element/PI local name (PI target for kindPI)
	attrs    []attrNode
	nsDecls  []nsDecl
	text     string // text/comment content, or PI data (kindPI)
	children []*xNode
	parent   *xNode
}

func newElem(local string) *xNode {
	return &xNode{kind: kindElement, local: local}
}

func newText(s string) *xNode {
	return &xNode{kind: kindText, text: s}
}

func (n *xNode) appendChild(c *xNode) {
	c.parent = n
	n.children = append(n.children, c)
}

func (n *xNode) qualifiedName() string {
	if n.prefix == "" {
		return n.local
	}
	return n.prefix + ":" + n.local
}

func (n *xNode) getAttr(uri, local string) string {
	for _, a := range n.attrs {
		if a.uri == uri && a.local == local {
			return a.value
		}
	}
	return ""
}

// documentElement returns the single root element child of a kindDocument
// node (nil if none — malformed input).
func (n *xNode) documentElement() *xNode {
	for _, c := range n.children {
		if c.kind == kindElement {
			return c
		}
	}
	return nil
}

// ============================================================================
// Parser
//
// Hand-written because Canonical XML needs low-level control that Go's
// encoding/xml does not expose: which namespace declarations are physically
// present on which element (as opposed to resolved/inherited), the literal
// prefix used at each element/attribute (not just the resolved namespace
// URI), and prolog/epilog PIs and comments outside the document element
// (needed to reproduce W3C C14N spec test vectors, see signer_test.go).
// Scope is intentionally narrow: no DTD/entity processing beyond the five
// predefined XML entities and numeric character references (fiscal XML
// never has a DOCTYPE; signxml itself rejects DTDs outright, so parity
// doesn't require supporting them).
// ============================================================================

type xmlParser struct {
	data []byte
	pos  int
}

func parseDocument(data []byte) (*xNode, error) {
	data = normalizeLineEndings(data)
	p := &xmlParser{data: data}

	doc := &xNode{kind: kindDocument}

	p.skipXMLDecl()
	if err := p.parseMisc(doc, true); err != nil {
		return nil, err
	}

	p.skipSpace()
	if !p.hasPrefix("<") || p.peekAt(1) == '?' || p.peekAt(1) == '!' {
		return nil, errors.New("xmlops: expected document element")
	}
	root, err := p.parseElement(nil) // root.parent stays nil: it has no real ancestor.
	if err != nil {
		return nil, err
	}
	doc.children = append(doc.children, root)

	if err := p.parseMisc(doc, false); err != nil {
		return nil, err
	}
	return doc, nil
}

// normalizeLineEndings applies XML 1.0's end-of-line handling (2.11):
// CRLF and lone CR are normalized to LF, uniformly across the whole
// document (including inside tags/attribute values), before any parsing.
func normalizeLineEndings(data []byte) []byte {
	if !bytesContainsCR(data) {
		return data
	}
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if data[i] == '\r' {
			out = append(out, '\n')
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
			}
			continue
		}
		out = append(out, data[i])
	}
	return out
}

func bytesContainsCR(data []byte) bool {
	for _, b := range data {
		if b == '\r' {
			return true
		}
	}
	return false
}

func (p *xmlParser) eof() bool { return p.pos >= len(p.data) }

func (p *xmlParser) peekAt(off int) byte {
	if p.pos+off >= len(p.data) {
		return 0
	}
	return p.data[p.pos+off]
}

func (p *xmlParser) hasPrefix(s string) bool {
	return strings.HasPrefix(string(p.data[p.pos:]), s)
}

func (p *xmlParser) skipSpace() {
	for !p.eof() {
		switch p.data[p.pos] {
		case ' ', '\t', '\n':
			p.pos++
		default:
			return
		}
	}
}

func isNameStartByte(b byte) bool {
	return b == '_' || b == ':' ||
		(b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b >= 0x80
}

func isNameByte(b byte) bool {
	return isNameStartByte(b) || b == '-' || b == '.' || (b >= '0' && b <= '9')
}

func (p *xmlParser) parseName() (string, error) {
	start := p.pos
	if p.eof() || !isNameStartByte(p.data[p.pos]) {
		return "", fmt.Errorf("xmlops: expected name at offset %d", p.pos)
	}
	p.pos++
	for !p.eof() && isNameByte(p.data[p.pos]) {
		p.pos++
	}
	return string(p.data[start:p.pos]), nil
}

// skipXMLDecl skips a leading `<?xml ... ?>` declaration, if present.
func (p *xmlParser) skipXMLDecl() {
	if strings.HasPrefix(string(p.data[p.pos:]), "<?xml") {
		// Must be "<?xml" followed by whitespace or "?" (not e.g. "<?xml-stylesheet").
		next := p.peekAt(5)
		if next == ' ' || next == '\t' || next == '\n' || next == '?' {
			end := strings.Index(string(p.data[p.pos:]), "?>")
			if end >= 0 {
				p.pos += end + 2
			}
		}
	}
}

// parseMisc consumes comments, PIs, DOCTYPE (skipped, not modeled) and
// whitespace at the top level (outside the document element), appending
// only comment/PI nodes to doc — matching the C14N model where prolog
// whitespace carries no information (spec 3.1: consecutive top-level items
// are joined by a single newline on output, not the original spacing).
func (p *xmlParser) parseMisc(doc *xNode, beforeRoot bool) error {
	for {
		p.skipSpace()
		if p.eof() {
			return nil
		}
		switch {
		case p.hasPrefix("<!--"):
			c, err := p.parseComment()
			if err != nil {
				return err
			}
			doc.appendChild(c)
		case p.hasPrefix("<?"):
			pi, err := p.parsePI()
			if err != nil {
				return err
			}
			doc.appendChild(pi)
		case beforeRoot && p.hasPrefix("<!DOCTYPE"):
			if err := p.skipDoctype(); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (p *xmlParser) skipDoctype() error {
	// Balance nested [ ... ] (internal subset) and bail at the matching
	// top-level '>'; no entity/attribute-default processing (see package
	// doc — fiscal XML never has a DOCTYPE).
	depth := 0
	for !p.eof() {
		switch p.data[p.pos] {
		case '[':
			depth++
		case ']':
			depth--
		case '>':
			if depth <= 0 {
				p.pos++
				return nil
			}
		}
		p.pos++
	}
	return errors.New("xmlops: unterminated DOCTYPE")
}

func (p *xmlParser) parseComment() (*xNode, error) {
	p.pos += len("<!--")
	end := strings.Index(string(p.data[p.pos:]), "-->")
	if end < 0 {
		return nil, errors.New("xmlops: unterminated comment")
	}
	text := string(p.data[p.pos : p.pos+end])
	p.pos += end + len("-->")
	return &xNode{kind: kindComment, text: text}, nil
}

// parsePI parses `<?target data?>`. Per C14N spec 3.1, only the leading
// whitespace between target and data is dropped; everything else in data
// (including trailing whitespace and internal newlines) is preserved
// verbatim, and if there is no data at all the PI renders as `<?target?>`
// with no separating space.
func (p *xmlParser) parsePI() (*xNode, error) {
	p.pos += len("<?")
	target, err := p.parseName()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	end := strings.Index(string(p.data[p.pos:]), "?>")
	if end < 0 {
		return nil, errors.New("xmlops: unterminated processing instruction")
	}
	data := string(p.data[p.pos : p.pos+end])
	p.pos += end + len("?>")
	return &xNode{kind: kindPI, local: target, text: data}, nil
}

// parseElement parses a full element, including its attributes and
// children (recursively), starting at '<'. parent is linked immediately
// (rather than by the caller after parseElement returns) because
// resolvePrefix below needs a complete ancestor chain to resolve a
// namespace prefix this element doesn't redeclare itself.
func (p *xmlParser) parseElement(parent *xNode) (*xNode, error) {
	p.pos++ // '<'
	qname, err := p.parseName()
	if err != nil {
		return nil, err
	}
	prefix, local := splitQName(qname)

	el := &xNode{kind: kindElement, prefix: prefix, local: local, parent: parent}

	for {
		p.skipSpace()
		if p.eof() {
			return nil, errors.New("xmlops: unterminated start tag")
		}
		if p.data[p.pos] == '/' || p.data[p.pos] == '>' {
			break
		}
		aqname, err := p.parseName()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.eof() || p.data[p.pos] != '=' {
			return nil, fmt.Errorf("xmlops: expected '=' after attribute name %q", aqname)
		}
		p.pos++
		p.skipSpace()
		rawVal, err := p.parseQuoted()
		if err != nil {
			return nil, err
		}
		val, err := decodeText(rawVal, true)
		if err != nil {
			return nil, err
		}

		switch {
		case aqname == "xmlns":
			el.nsDecls = append(el.nsDecls, nsDecl{prefix: "", uri: val})
		case strings.HasPrefix(aqname, "xmlns:"):
			el.nsDecls = append(el.nsDecls, nsDecl{prefix: aqname[len("xmlns:"):], uri: val})
		default:
			aprefix, alocal := splitQName(aqname)
			el.attrs = append(el.attrs, attrNode{prefix: aprefix, local: alocal, value: val})
		}
	}

	// Resolve namespace URIs now that we know this element's own nsDecls,
	// walking ancestors for anything not locally declared.
	el.uri = resolvePrefix(el, el.prefix)
	for i := range el.attrs {
		if el.attrs[i].prefix != "" {
			el.attrs[i].uri = resolvePrefix(el, el.attrs[i].prefix)
		}
	}

	if p.data[p.pos] == '/' {
		p.pos++ // '/'
		if p.eof() || p.data[p.pos] != '>' {
			return nil, errors.New("xmlops: malformed self-closing tag")
		}
		p.pos++ // '>'
		return el, nil
	}
	p.pos++ // '>'

	if err := p.parseContent(el); err != nil {
		return nil, err
	}
	return el, nil
}

// parseContent parses child nodes until the matching end tag (consumed).
func (p *xmlParser) parseContent(parent *xNode) error {
	var textBuf strings.Builder
	flushText := func() {
		if textBuf.Len() > 0 {
			parent.appendChild(&xNode{kind: kindText, text: textBuf.String()})
			textBuf.Reset()
		}
	}

	for {
		if p.eof() {
			return fmt.Errorf("xmlops: unterminated element %q", parent.qualifiedName())
		}
		switch {
		case p.hasPrefix("</"):
			flushText()
			p.pos += 2
			name, err := p.parseName()
			if err != nil {
				return err
			}
			p.skipSpace()
			if p.eof() || p.data[p.pos] != '>' {
				return fmt.Errorf("xmlops: malformed end tag for %q", name)
			}
			p.pos++
			return nil
		case p.hasPrefix("<!--"):
			flushText()
			c, err := p.parseComment()
			if err != nil {
				return err
			}
			parent.appendChild(c)
		case p.hasPrefix("<![CDATA["):
			raw, err := p.parseCDATA()
			if err != nil {
				return err
			}
			textBuf.WriteString(raw)
		case p.hasPrefix("<?"):
			flushText()
			pi, err := p.parsePI()
			if err != nil {
				return err
			}
			parent.appendChild(pi)
		case p.data[p.pos] == '<':
			flushText()
			child, err := p.parseElement(parent)
			if err != nil {
				return err
			}
			parent.children = append(parent.children, child)
		default:
			start := p.pos
			for !p.eof() && p.data[p.pos] != '<' {
				p.pos++
			}
			decoded, err := decodeText(string(p.data[start:p.pos]), false)
			if err != nil {
				return err
			}
			textBuf.WriteString(decoded)
		}
	}
}

func (p *xmlParser) parseCDATA() (string, error) {
	p.pos += len("<![CDATA[")
	end := strings.Index(string(p.data[p.pos:]), "]]>")
	if end < 0 {
		return "", errors.New("xmlops: unterminated CDATA section")
	}
	text := string(p.data[p.pos : p.pos+end])
	p.pos += end + len("]]>")
	return text, nil
}

// parseQuoted parses a '...' or "..." quoted value, returning its raw
// (still entity-encoded) content.
func (p *xmlParser) parseQuoted() (string, error) {
	if p.eof() || (p.data[p.pos] != '"' && p.data[p.pos] != '\'') {
		return "", errors.New("xmlops: expected quoted attribute value")
	}
	quote := p.data[p.pos]
	p.pos++
	start := p.pos
	for !p.eof() && p.data[p.pos] != quote {
		p.pos++
	}
	if p.eof() {
		return "", errors.New("xmlops: unterminated attribute value")
	}
	val := string(p.data[start:p.pos])
	p.pos++ // closing quote
	return val, nil
}

func splitQName(qname string) (prefix, local string) {
	if i := strings.IndexByte(qname, ':'); i >= 0 {
		return qname[:i], qname[i+1:]
	}
	return "", qname
}

// resolvePrefix resolves prefix ("" for the default namespace) to a
// namespace URI by walking up from el (inclusive) through ancestors,
// nearest declaration wins. Returns "" if unbound (no namespace).
func resolvePrefix(el *xNode, prefix string) string {
	for n := el; n != nil; n = n.parent {
		for _, d := range n.nsDecls {
			if d.prefix == prefix {
				return d.uri
			}
		}
	}
	return ""
}

// decodeText decodes XML entity/character references in raw text.
// isAttr controls XML 1.0 §3.3.3 attribute-value normalization: a literal
// (non-referenced) tab/newline is replaced by a single space; characters
// produced by a character reference are not normalized and keep their
// literal value (this is what W3C C14N spec example 3.4 exercises).
func decodeText(raw string, isAttr bool) (string, error) {
	if !strings.ContainsRune(raw, '&') && (!isAttr || (!strings.ContainsRune(raw, '\t') && !strings.ContainsRune(raw, '\n'))) {
		return raw, nil
	}
	var b strings.Builder
	i := 0
	for i < len(raw) {
		c := raw[i]
		switch {
		case c == '&':
			semi := strings.IndexByte(raw[i:], ';')
			if semi < 0 {
				return "", errors.New("xmlops: unterminated entity reference")
			}
			ent := raw[i+1 : i+semi]
			i += semi + 1
			switch {
			case ent == "amp":
				b.WriteByte('&')
			case ent == "lt":
				b.WriteByte('<')
			case ent == "gt":
				b.WriteByte('>')
			case ent == "apos":
				b.WriteByte('\'')
			case ent == "quot":
				b.WriteByte('"')
			case strings.HasPrefix(ent, "#x") || strings.HasPrefix(ent, "#X"):
				r, err := parseCharRef(ent[2:], 16)
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
			case strings.HasPrefix(ent, "#"):
				r, err := parseCharRef(ent[1:], 10)
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
			default:
				return "", fmt.Errorf("xmlops: unsupported entity &%s;", ent)
			}
		case isAttr && (c == '\t' || c == '\n'):
			b.WriteByte(' ')
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}

func parseCharRef(digits string, base int) (rune, error) {
	var n int64
	for _, d := range []byte(digits) {
		var v int64
		switch {
		case d >= '0' && d <= '9':
			v = int64(d - '0')
		case base == 16 && d >= 'a' && d <= 'f':
			v = int64(d-'a') + 10
		case base == 16 && d >= 'A' && d <= 'F':
			v = int64(d-'A') + 10
		default:
			return 0, fmt.Errorf("xmlops: invalid character reference digit %q", d)
		}
		n = n*int64(base) + v
	}
	return rune(n), nil
}

// ============================================================================
// materialize: prepare a real document's element for standalone
// canonicalization.
//
// signxml's XMLProcessor.get_root (used whenever a Reference resolves to a
// sub-element of the signed document) does not canonicalize that element
// in place; it first does `self._fromstring(self._tostring(data))` — i.e.
// a normal (non-C14N) serialize of just that element, then a fresh parse
// of the result as an independent document. Because normal serialization
// of a detached-view sub-element physically renders any namespace it only
// inherits from real ancestors, the reparsed copy has that namespace as a
// genuine local declaration.
//
// This matters because canonicalizing the *same* element in place (without
// this round trip) produces different, incorrect bytes: verified
// empirically against signxml/lxml directly (spurious `xmlns=""` on
// descendants — a known lxml/libxml2 C14N quirk, see signxml's own comment
// referencing https://github.com/XML-Security/signxml/issues/193). Skipping
// this step would silently produce a signature py-dfe's SEFAZ-facing
// counterpart does not produce.
//
// materialize reproduces the effect of that round trip directly on the
// tree (no re-serialize/re-parse needed): it copies el's own in-scope
// namespace declarations (its own plus anything inherited from real
// ancestors and not already declared on el itself) onto a detached
// top-level copy, then returns that copy for canonicalizeElement to treat
// as a genuine, ancestor-free root.
// ============================================================================

func materialize(el *xNode) *xNode {
	declaredHere := make(map[string]bool, len(el.nsDecls))
	for _, d := range el.nsDecls {
		declaredHere[d.prefix] = true
	}

	var inherited []nsDecl
	seen := map[string]bool{}
	for n := el.parent; n != nil; n = n.parent {
		for _, d := range n.nsDecls {
			if declaredHere[d.prefix] || seen[d.prefix] {
				continue
			}
			seen[d.prefix] = true
			inherited = append(inherited, d)
		}
	}

	copyNode := *el
	copyNode.parent = nil
	copyNode.nsDecls = append(append([]nsDecl{}, inherited...), el.nsDecls...)
	return &copyNode
}

// ============================================================================
// Canonical XML 1.0 (REC-xml-c14n-20010315), plain/non-exclusive,
// without comments — matching py-dfe's c14n_algorithm and with_comments
// configuration exactly (signxml._c14n: with_comments is only true if the
// algorithm URI ends in "#WithComments", which py-dfe's does not).
//
// canonicalizeElement treats el as a genuine, ancestor-free root (see
// materialize above for how real document sub-elements get here).
// canonicalizeDocument additionally handles a full document's prolog/
// epilog PIs and comments, per W3C C14N spec section 3.1.
// ============================================================================

func canonicalizeElement(el *xNode) []byte {
	var buf strings.Builder
	serializeElementC14N(&buf, el, map[string]string{})
	return []byte(buf.String())
}

func canonicalizeDocument(doc *xNode) []byte {
	var parts []string
	for _, c := range doc.children {
		switch c.kind {
		case kindComment:
			continue // with_comments=false
		case kindPI:
			var b strings.Builder
			serializePIC14N(&b, c)
			parts = append(parts, b.String())
		case kindElement:
			var b strings.Builder
			serializeElementC14N(&b, c, map[string]string{})
			parts = append(parts, b.String())
		}
	}
	return []byte(strings.Join(parts, "\n"))
}

func serializeElementC14N(buf *strings.Builder, el *xNode, parentNS map[string]string) {
	newNS := make(map[string]string, len(parentNS)+len(el.nsDecls))
	for k, v := range parentNS {
		newNS[k] = v
	}

	var toRender []nsDecl
	for _, d := range el.nsDecls {
		if cur := newNS[d.prefix]; cur != d.uri {
			toRender = append(toRender, d)
			newNS[d.prefix] = d.uri
		}
	}
	sort.Slice(toRender, func(i, j int) bool { return toRender[i].prefix < toRender[j].prefix })

	sortedAttrs := append([]attrNode{}, el.attrs...)
	sort.Slice(sortedAttrs, func(i, j int) bool {
		if sortedAttrs[i].uri != sortedAttrs[j].uri {
			return sortedAttrs[i].uri < sortedAttrs[j].uri
		}
		return sortedAttrs[i].local < sortedAttrs[j].local
	})

	buf.WriteByte('<')
	buf.WriteString(el.qualifiedName())
	for _, d := range toRender {
		buf.WriteByte(' ')
		if d.prefix == "" {
			buf.WriteString("xmlns=\"")
		} else {
			buf.WriteString("xmlns:")
			buf.WriteString(d.prefix)
			buf.WriteString("=\"")
		}
		buf.WriteString(escapeC14NAttr(d.uri))
		buf.WriteByte('"')
	}
	for _, a := range sortedAttrs {
		buf.WriteByte(' ')
		buf.WriteString(a.qualifiedName())
		buf.WriteString("=\"")
		buf.WriteString(escapeC14NAttr(a.value))
		buf.WriteByte('"')
	}
	buf.WriteByte('>')

	for _, child := range el.children {
		switch child.kind {
		case kindElement:
			serializeElementC14N(buf, child, newNS)
		case kindText:
			buf.WriteString(escapeC14NText(child.text))
		case kindComment:
			// dropped: with_comments=false
		case kindPI:
			serializePIC14N(buf, child)
		}
	}

	buf.WriteString("</")
	buf.WriteString(el.qualifiedName())
	buf.WriteByte('>')
}

func serializePIC14N(buf *strings.Builder, pi *xNode) {
	buf.WriteString("<?")
	buf.WriteString(pi.local)
	if pi.text != "" {
		buf.WriteByte(' ')
		buf.WriteString(pi.text)
	}
	buf.WriteString("?>")
}

// escapeC14NText escapes text content per C14N spec 2.6: &, <, > always;
// literal CR as &#xD; (any bare CR surviving to this point must have come
// from a character reference, since normalizeLineEndings already removed
// literal ones — re-escaping it prevents a subsequent XML parse from
// silently normalizing it away).
func escapeC14NText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\r':
			b.WriteString("&#xD;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeC14NAttr escapes attribute values per C14N spec 2.6: &, <, " plus
// tab/LF/CR as hex character references.
func escapeC14NAttr(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '"':
			b.WriteString("&quot;")
		case '\t':
			b.WriteString("&#x9;")
		case '\n':
			b.WriteString("&#xA;")
		case '\r':
			b.WriteString("&#xD;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// ============================================================================
// Normal (non-canonical) serialization of the final signed document —
// matching py-dfe's `etree.tostring(signed, xml_declaration=False)`:
// self-closing tags for empty elements, attributes kept in their original/
// construction order (not sorted), namespace declarations only rendered
// when new/changed relative to the nearest rendered ancestor (same
// redundancy rule as C14N, verified empirically against lxml), decimal
// (not hex) character references for control characters.
// ============================================================================

func serializeDocument(doc *xNode) []byte {
	var buf strings.Builder
	first := true
	sep := func() {
		if !first {
			buf.WriteByte('\n')
		}
		first = false
	}
	for _, c := range doc.children {
		switch c.kind {
		case kindComment:
			sep()
			buf.WriteString("<!--")
			buf.WriteString(c.text)
			buf.WriteString("-->")
		case kindPI:
			sep()
			serializePINormal(&buf, c)
		case kindElement:
			sep()
			serializeElementNormal(&buf, c, map[string]string{})
		}
	}
	return []byte(buf.String())
}

func serializePINormal(buf *strings.Builder, pi *xNode) {
	serializePIC14N(buf, pi) // identical rendering rules
}

func serializeElementNormal(buf *strings.Builder, el *xNode, parentNS map[string]string) {
	newNS := make(map[string]string, len(parentNS)+len(el.nsDecls))
	for k, v := range parentNS {
		newNS[k] = v
	}

	var toRender []nsDecl
	for _, d := range el.nsDecls {
		if cur := newNS[d.prefix]; cur != d.uri {
			toRender = append(toRender, d)
			newNS[d.prefix] = d.uri
		}
	}

	buf.WriteByte('<')
	buf.WriteString(el.qualifiedName())
	for _, d := range toRender {
		buf.WriteByte(' ')
		if d.prefix == "" {
			buf.WriteString("xmlns=\"")
		} else {
			buf.WriteString("xmlns:")
			buf.WriteString(d.prefix)
			buf.WriteString("=\"")
		}
		buf.WriteString(escapeNormalAttr(d.uri))
		buf.WriteByte('"')
	}
	for _, a := range el.attrs {
		buf.WriteByte(' ')
		buf.WriteString(a.qualifiedName())
		buf.WriteString("=\"")
		buf.WriteString(escapeNormalAttr(a.value))
		buf.WriteByte('"')
	}

	if len(el.children) == 0 {
		buf.WriteString("/>")
		return
	}
	buf.WriteByte('>')
	for _, child := range el.children {
		switch child.kind {
		case kindElement:
			serializeElementNormal(buf, child, newNS)
		case kindText:
			buf.WriteString(escapeNormalText(child.text))
		case kindComment:
			buf.WriteString("<!--")
			buf.WriteString(child.text)
			buf.WriteString("-->")
		case kindPI:
			serializePINormal(buf, child)
		}
	}
	buf.WriteString("</")
	buf.WriteString(el.qualifiedName())
	buf.WriteByte('>')
}

// escapeNormalText matches lxml's non-canonical text serialization: &, <, >
// always escaped; a literal CR escaped as &#13; (decimal) so it survives a
// subsequent parse instead of being silently normalized to LF. Tab/LF are
// left unescaped (safe to represent literally in text content).
func escapeNormalText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\r':
			b.WriteString("&#13;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeNormalAttr matches lxml's non-canonical attribute serialization:
// &, <, >, " escaped; tab/LF/CR escaped as decimal character references
// (&#9;/&#10;/&#13;) — verified empirically against lxml.etree.tostring.
func escapeNormalAttr(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\t':
			b.WriteString("&#9;")
		case '\n':
			b.WriteString("&#10;")
		case '\r':
			b.WriteString("&#13;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
