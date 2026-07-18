// Package soap ports py-dfe's SOAP 1.2 envelope handling
// (py-dfe/py_dfe/soap/envelope.py) to Go: wrapping a signed/unsigned fiscal
// XML body for a specific SEFAZ service call (Build), and unwrapping SEFAZ's
// SOAP response to get at the inner result XML (ParseResult).
//
// All service/UF-specific lookups (header/body/result element names, the
// wrapped-body special cases, WSDL operation names, header version override)
// come from the constants package — this file has no lookup tables of its own.
package soap

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

const soap12NS = "http://www.w3.org/2003/05/soap-envelope"

// Builder builds SOAP 1.2 envelopes for one (docType, uf, service) call,
// mirroring py-dfe's SOAPEnvelopeBuilder. All lookups happen once in
// NewBuilder so Build/ContentType are pure formatting.
type Builder struct {
	wsdlNS    string
	operation string
	version   string
	cUF       string
	wrapped   bool
	headerEl  string
	bodyEl    string
}

// NewBuilder resolves the WSDL namespace/operation/element names for
// (docType, uf, service). Returns an error if service is unknown for docType,
// mirroring py-dfe's SOAPError("Unknown service ...").
func NewBuilder(docType, uf, service string) (*Builder, error) {
	wsdlName, ok := constants.WSDLServiceByDocType[docType][service]
	if !ok {
		return nil, fmt.Errorf("soap: unknown service %q for doc_type %q", service, docType)
	}

	operation, err := constants.WSDLOperation(uf, docType, service)
	if err != nil {
		return nil, fmt.Errorf("soap: %w", err)
	}

	elems, ok := constants.SOAPElements[docType]
	if !ok {
		return nil, fmt.Errorf("soap: no SOAP element names for doc_type %q", docType)
	}

	version := constants.SchemaVersion[docType]
	if v, ok := constants.SOAPHeaderVersionOverride[service]; ok {
		version = v
	}

	wrapped := constants.SOAPWrappedBodyServices[service] || constants.IsSOAPWrappedBodyOverride(uf, service)

	return &Builder{
		wsdlNS:    fmt.Sprintf("%s/wsdl/%s", constants.DocNamespace[docType], wsdlName),
		operation: operation,
		version:   version,
		cUF:       strconv.Itoa(constants.UFIBGE[uf]), // 0 for unknown uf, matching py-dfe's UF_IBGE.get(uf, 0)
		wrapped:   wrapped,
		headerEl:  elems.Header,
		bodyEl:    elems.Body,
	}, nil
}

// ContentType returns the Content-Type header value including the SOAP
// action, matching py-dfe's SOAPEnvelopeBuilder.content_type.
func (b *Builder) ContentType() string {
	return fmt.Sprintf(`application/soap+xml; charset=utf-8; action="%s/%s"`, b.wsdlNS, b.operation)
}

// Build wraps payloadXML (already-serialized fiscal XML, e.g. the output of
// the fiscal XML builder/signer) in a SOAP 1.2 envelope ready to POST to
// SEFAZ. If gzipPayload is true, payloadXML is gzip-compressed and
// base64-encoded into the body element instead of embedded as XML (some
// services require this). If includeHeader is true, a SOAP Header carrying
// cUF/versaoDados is added.
func (b *Builder) Build(payloadXML []byte, gzipPayload, includeHeader bool) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<soap12:Envelope xmlns:soap12="` + soap12NS + `" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema">`)

	if includeHeader {
		buf.WriteString(`<soap12:Header>`)
		fmt.Fprintf(&buf, `<%s xmlns="%s"><cUF>%s</cUF><versaoDados>%s</versaoDados></%s>`,
			b.headerEl, b.wsdlNS, b.cUF, b.version, b.headerEl)
		buf.WriteString(`</soap12:Header>`)
	}

	buf.WriteString(`<soap12:Body>`)
	if b.wrapped {
		fmt.Fprintf(&buf, `<%s xmlns="%s"><%s>`, b.operation, b.wsdlNS, b.bodyEl)
	} else {
		fmt.Fprintf(&buf, `<%s xmlns="%s">`, b.bodyEl, b.wsdlNS)
	}

	if gzipPayload {
		var gz bytes.Buffer
		zw := gzip.NewWriter(&gz)
		if _, err := zw.Write(payloadXML); err != nil {
			return nil, fmt.Errorf("soap: gzip payload: %w", err)
		}
		if err := zw.Close(); err != nil {
			return nil, fmt.Errorf("soap: gzip payload: %w", err)
		}
		buf.WriteString(base64.StdEncoding.EncodeToString(gz.Bytes()))
	} else {
		buf.Write(payloadXML)
	}

	if b.wrapped {
		fmt.Fprintf(&buf, `</%s></%s>`, b.bodyEl, b.operation)
	} else {
		fmt.Fprintf(&buf, `</%s>`, b.bodyEl)
	}
	buf.WriteString(`</soap12:Body></soap12:Envelope>`)

	return buf.Bytes(), nil
}

// ParseResult unwraps raw SOAP 1.2 response bytes down to the inner result
// XML: it locates the SOAP Body, then searches it (at any depth, since
// operation-result wrappers vary) for the element named after docType's
// Result element (constants.SOAPElements[docType].Result, e.g.
// "nfeResultMsg") and returns that element's raw bytes.
//
// If no such element is found, it falls back to the Body's first child
// element, mirroring py-dfe's extract_body (py-dfe/py_dfe/soap/envelope.py),
// which does not check element names at all.
func ParseResult(docType string, soapResponse []byte) ([]byte, error) {
	elems, ok := constants.SOAPElements[docType]
	if !ok {
		return nil, fmt.Errorf("soap: no SOAP element names for doc_type %q", docType)
	}

	body, found, err := findElement(soapResponse, soap12NS, "Body")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("soap: SOAP Body element not found in response")
	}

	if result, found, err := findElement(body, "", elems.Result); err != nil {
		return nil, err
	} else if found {
		return result, nil
	}

	return firstChild(body)
}

// findElement does a document-order search (which, since xml.Decoder emits
// tokens depth-first, is effectively a DFS over the whole tree) for the first
// element whose local name is local and — if ns is non-empty — whose
// namespace is ns. It returns the raw bytes spanning that element's start and
// end tags.
func findElement(data []byte, ns, local string) ([]byte, bool, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("soap: parse xml: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != local || (ns != "" && se.Name.Space != ns) {
			continue
		}
		if err := dec.Skip(); err != nil {
			return nil, false, fmt.Errorf("soap: parse xml: %w", err)
		}
		return data[start:dec.InputOffset()], true, nil
	}
}

// firstChild returns the raw bytes of the first child element of elementXML
// (which must be a single well-formed element, e.g. a SOAP Body), i.e. the
// element one level down from elementXML's own root tag.
func firstChild(elementXML []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(elementXML))
	depth := 0
	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("soap: SOAP Body is empty")
		}
		if err != nil {
			return nil, fmt.Errorf("soap: parse xml: %w", err)
		}
		if _, ok := tok.(xml.StartElement); ok {
			depth++
			if depth == 2 {
				if err := dec.Skip(); err != nil {
					return nil, fmt.Errorf("soap: parse xml: %w", err)
				}
				return elementXML[start:dec.InputOffset()], nil
			}
		}
	}
}
