package xmlops

import "strings"

// emissionSpec/eventSpec mirror py-dfe's _EMISSION/_EVENT tuples
// (py-dfe/py_dfe/xmlops/processor.py).
type emissionSpec struct{ procTag, docTag, protTag, versao string }
type eventSpec struct{ procTag, versao, eventTag, retEventTag string }

// emissionServices mirrors py-dfe's _EMISSION.
var emissionServices = map[string]emissionSpec{
	"NFeAutorizacao":   {"nfeProc", "NFe", "protNFe", "4.00"},
	"NfceAutorizacao":  {"nfeProc", "NFe", "protNFe", "4.00"},
	"CTeRecepcaoSinc":  {"cteProc", "CTe", "protCTe", "4.00"},
	"CTeRecepcaoOS":    {"cteOSProc", "CTeOS", "protCTe", "4.00"},
	"CTeRecepcaoGTVe":  {"GTVeProc", "GTVe", "protCTe", "4.00"},
	"CTeRecepcaoSimp":  {"cteSimpProc", "CTeSimp", "protCTe", "4.00"},
	"MDFeRecepcaoSinc": {"mdfeProc", "MDFe", "protMDFe", "3.00"},

	// Inutilização não é emissão, mas tem exatamente a mesma forma processada:
	// documento assinado + resposta da SEFAZ sob uma raiz única. O elemento é
	// ProcInutNFe (P maiúsculo, procInutNFe_v4.00.xsd), e o "documento" é a
	// própria raiz do request — firstByLocal inclui a raiz na busca.
	"NfeInutilizacao": {"ProcInutNFe", "inutNFe", "retInutNFe", "4.00"},
}

// eventServices mirrors py-dfe's _EVENT. py-dfe's own CTeRecepcaoEvento
// entry has a latent bug there: adjacent string literals ("4.00" "eventoCTe")
// concatenate in Python, collapsing what should be a 4-tuple into 3 elements
// — the unpack (`proc_tag, versao, event_tag, ret_event_tag = _EVENT[service]`)
// raises at runtime, which build_processed_xml's blanket try/except silently
// swallows (logs at debug, returns None). CTeRecepcaoEvento functionally
// never gets a processed XML in py-dfe today because of this. Ported here
// with the obviously-intended 4 values instead of reproducing the crash —
// matching an accidental typo helps no one — flagged here for awareness.
var eventServices = map[string]eventSpec{
	"RecepcaoEvento":     {"procEventoNFe", "1.00", "evento", "retEvento"},
	"CTeRecepcaoEvento":  {"procEventoCTe", "4.00", "eventoCTe", "retEventoCTe"},
	"MDFeRecepcaoEvento": {"procEventoMDFe", "3.00", "eventoMDFe", "retEventoMDFe"},
}

// docNamespaces mirrors processor.py's _NS.
var docNamespaces = map[string]string{
	"nfe":  "http://www.portalfiscal.inf.br/nfe",
	"nfce": "http://www.portalfiscal.inf.br/nfe",
	"cte":  "http://www.portalfiscal.inf.br/cte",
	"mdfe": "http://www.portalfiscal.inf.br/mdfe",
}

// BuildProcessedXML returns the processed XML string (nfeProc/cteProc/
// mdfeProc/procEventoNFe/etc — the signed request document and SEFAZ's
// protocol/event response combined into the single document SEFAZ
// convention expects for storage), or ("", false) if service has no
// processed form. Mirrors py-dfe's build_processed_xml
// (py-dfe/py_dfe/xmlops/processor.py), including its behavior of silently
// returning false on any malformed input — a processed-XML failure must
// never break the underlying SEFAZ call it's attached to.
func BuildProcessedXML(docType, service string, requestXML, responseXML []byte) (result string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			result, ok = "", false
		}
	}()

	ns, hasNS := docNamespaces[docType]
	if !hasNS {
		return "", false
	}
	if spec, isEmission := emissionServices[service]; isEmission {
		return buildEmission(ns, spec, requestXML, responseXML)
	}
	if spec, isEvent := eventServices[service]; isEvent {
		return buildEvents(ns, spec, requestXML, responseXML)
	}
	return "", false
}

func buildEmission(ns string, spec emissionSpec, requestXML, responseXML []byte) (string, bool) {
	reqRoot, respRoot, err := parseRoots(requestXML, responseXML)
	if err != nil {
		return "", false
	}

	docEl := firstByLocal(reqRoot, spec.docTag)
	protEl := firstByLocal(respRoot, spec.protTag)
	if docEl == nil || protEl == nil {
		return "", false
	}

	proc := newElem(spec.procTag)
	proc.nsDecls = []nsDecl{{prefix: "", uri: ns}}
	proc.attrs = []attrNode{{local: "versao", value: spec.versao}}
	proc.appendChild(deepCopyNode(docEl))
	proc.appendChild(deepCopyNode(protEl))

	return serializeStandalone(proc), true
}

func buildEvents(ns string, spec eventSpec, requestXML, responseXML []byte) (string, bool) {
	reqRoot, respRoot, err := parseRoots(requestXML, responseXML)
	if err != nil {
		return "", false
	}

	events := allByLocal(reqRoot, spec.eventTag)
	if len(events) == 0 {
		events = []*xNode{reqRoot}
	}
	retEvents := allByLocal(respRoot, spec.retEventTag)
	if len(retEvents) == 0 {
		return "", false
	}

	n := len(events)
	if len(retEvents) < n {
		n = len(retEvents)
	}
	results := make([]string, 0, n)
	for i := 0; i < n; i++ {
		proc := newElem(spec.procTag)
		proc.nsDecls = []nsDecl{{prefix: "", uri: ns}}
		proc.attrs = []attrNode{{local: "versao", value: spec.versao}}
		proc.appendChild(asLocalTag(events[i], "evento", ns))
		proc.appendChild(asLocalTag(retEvents[i], "retEvento", ns))
		results = append(results, serializeStandalone(proc))
	}
	if len(results) == 0 {
		return "", false
	}
	if len(results) == 1 {
		return results[0], true
	}
	// Multiple events: py-dfe returns a JSON array of XML strings.
	return jsonStringArray(results), true
}

func parseRoots(requestXML, responseXML []byte) (reqRoot, respRoot *xNode, err error) {
	reqDoc, err := parseDocument(requestXML)
	if err != nil {
		return nil, nil, err
	}
	respDoc, err := parseDocument(responseXML)
	if err != nil {
		return nil, nil, err
	}
	reqRoot = reqDoc.documentElement()
	respRoot = respDoc.documentElement()
	if reqRoot == nil || respRoot == nil {
		return nil, nil, errNoRootElement
	}
	return reqRoot, respRoot, nil
}

var errNoRootElement = &noRootElementError{}

type noRootElementError struct{}

func (*noRootElementError) Error() string { return "xmlops: document has no root element" }

// firstByLocal / allByLocal search root (inclusive) and all descendants for
// element(s) with the given local tag name, mirroring processor.py's
// _first_by_local/_all_by_local (etree.QName(...).localname comparisons,
// namespace-agnostic).
func firstByLocal(root *xNode, local string) *xNode {
	els := allByLocal(root, local)
	if len(els) == 0 {
		return nil
	}
	return els[0]
}

func allByLocal(root *xNode, local string) []*xNode {
	var out []*xNode
	var walk func(n *xNode)
	walk = func(n *xNode) {
		if n.kind == kindElement && n.local == local {
			out = append(out, n)
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)
	return out
}

// deepCopyNode returns a detached deep copy of n and its subtree, mirroring
// processor.py's _deep_copy (copy.deepcopy).
func deepCopyNode(n *xNode) *xNode {
	cp := *n
	cp.parent = nil
	cp.attrs = append([]attrNode{}, n.attrs...)
	cp.nsDecls = append([]nsDecl{}, n.nsDecls...)
	cp.children = make([]*xNode, len(n.children))
	for i, c := range n.children {
		child := deepCopyNode(c)
		child.parent = &cp
		cp.children[i] = child
	}
	return &cp
}

// asLocalTag returns a deep copy of el with its own tag renamed to
// localTag/ns if it doesn't already match, mirroring processor.py's
// _as_local_tag.
func asLocalTag(el *xNode, localTag, ns string) *xNode {
	cp := deepCopyNode(el)
	if cp.local != localTag {
		cp.local = localTag
		cp.prefix = ""
		cp.uri = ns
	}
	return cp
}

// serializeStandalone renders el as a standalone top-level element (with its
// own namespace declaration), matching processor.py's
// etree.tostring(proc, encoding="unicode", xml_declaration=False).
func serializeStandalone(el *xNode) string {
	el.parent = nil
	doc := &xNode{kind: kindDocument, children: []*xNode{el}}
	return string(serializeDocument(doc))
}

func jsonStringArray(items []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range s {
			switch r {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			default:
				b.WriteRune(r)
			}
		}
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}
