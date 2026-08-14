package dfe

import "gopkg.aoctech.app/dfe/go-dfe/internal/xmlops"

// BuildXMLFragment serializes body into an XML fragment with rootTag as its
// root element, reusing xmlops.BuildXML's dict<->XML convention (@attr,
// #text, xsdorder-based child ordering). Exported for callers outside this
// module — worker/internal/service/distribution.go needs it to rebuild the
// nfeProc document from the protNFe dict a consulta protocolo response
// carries (see docs/specs/2026-08-13-importacao-nfe-xml.md).
func BuildXMLFragment(body map[string]any, rootTag, xmlns string) ([]byte, error) {
	return xmlops.BuildXML(body, rootTag, xmlns)
}
