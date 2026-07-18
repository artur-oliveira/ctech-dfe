// Package xmlops ports py-dfe's dict<->XML conversion
// (py-dfe/py_dfe/xmlops/builder.py) to Go: building the fiscal XML body from
// a JSON-shaped map (the same shape as Request.Body, see ../../request.go),
// and parsing a SEFAZ XML response back into that same shape for callers
// like worker/internal/service/distribution.go (asMap/asSlice on
// respBody["retDistDFeInt"]["loteDistDFeInt"]["docZip"], etc).
//
// Convention (identical to py-dfe's docstring):
//   - a "@attr" key becomes an XML attribute named "attr"
//   - the "@xmlns" key sets the default namespace from that element down
//   - the "#text" key becomes the element's text content
//   - any other key becomes a child element; a slice value creates one
//     sibling element per item, all sharing the tag
//   - a bare scalar value (no map wrapper) becomes the element's text
//
// Data shape: BuildXML/ParseXML use plain map[string]any (not an ordered
// map/KV-pair structure), matching Request.Body's existing type. Attribute
// order is not fiscally significant (XML attributes are unordered per the
// XML/XSD spec) so attributes are emitted alphabetically for determinism.
// Child element order IS fiscally significant (XSD complexTypes are
// xs:sequence), so BuildXML resolves it from xsdorder.Resolve — a 1:1 port
// of py-dfe's XSD_ORDER table — exactly mirroring py-dfe's ancestor-scoped
// lookup. That table covers every real NF-e/NFC-e/CT-e/MDF-e element.
//
// ponytail: for a tag with no xsdorder entry (no real fiscal element should
// hit this), children fall back to alphabetical key order for determinism,
// since a plain Go map has no insertion order to fall back to the way
// py-dfe's dict does. Upgrade path: add the tag's sequence to xsdorder.
package xmlops

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"gopkg.aoctech.app/dfe/go-dfe/internal/xmlops/xsdorder"
)

// BuildXML converts body into an XML document with rootTag as its single
// root element and xmlns as the element's default namespace (pass "" for
// none). body may itself carry its own "@xmlns" (or a nested element may),
// which overrides xmlns from that point down — mirroring py-dfe's
// dict_to_xml/_build_element exactly.
func BuildXML(body map[string]any, rootTag, xmlns string) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeElement(&buf, rootTag, body, xmlns, "", ""); err != nil {
		return nil, fmt.Errorf("xmlops: build xml: %w", err)
	}
	return buf.Bytes(), nil
}

// writeElement writes <tag>...</tag> (or a self-closed <tag/>) to buf.
// inheritedNS is the default namespace value to use for tag absent a local
// "@xmlns" override (mirrors py-dfe's inherited_ns parameter). declaredNS is
// the namespace URI already emitted via xmlns= somewhere in the currently
// open ancestor chain ("" if none yet) — used only to avoid redeclaring an
// already-active default namespace, matching lxml's namespace reconciliation
// (py-dfe never passes an explicit nsmap to sub-elements, only to the root;
// libxml2 reuses an already-declared namespace instead of repeating it).
// ancestorPath is the colon-joined tag path of tag's ancestors ("" at the
// root), used for xsdorder.Resolve.
func writeElement(buf *bytes.Buffer, tag string, value any, inheritedNS, declaredNS, ancestorPath string) error {
	localNS := inheritedNS
	attrs := map[string]string{}
	children := map[string]any{}
	var text string
	hasText := false

	switch v := value.(type) {
	case map[string]any:
		for k, val := range v {
			switch {
			case k == "@xmlns":
				s, err := stringifyScalar(val)
				if err != nil {
					return fmt.Errorf("%s.@xmlns: %w", tag, err)
				}
				localNS = s
			case strings.HasPrefix(k, "@"):
				s, err := stringifyScalar(val)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", tag, k, err)
				}
				attrs[k[1:]] = s
			case k == "#text":
				s, err := stringifyScalar(val)
				if err != nil {
					return fmt.Errorf("%s.#text: %w", tag, err)
				}
				text, hasText = s, true
			default:
				children[k] = val
			}
		}
	case nil:
		// Empty element: no text, no children.
	default:
		s, err := stringifyScalar(v)
		if err != nil {
			return fmt.Errorf("%s: %w", tag, err)
		}
		text, hasText = s, true
	}

	buf.WriteByte('<')
	buf.WriteString(tag)

	newDeclaredNS := declaredNS
	if localNS != "" && localNS != declaredNS {
		buf.WriteString(` xmlns="`)
		buf.WriteString(escapeAttr(localNS))
		buf.WriteByte('"')
		newDeclaredNS = localNS
	}

	attrKeys := make([]string, 0, len(attrs))
	for k := range attrs {
		attrKeys = append(attrKeys, k)
	}
	sort.Strings(attrKeys)
	for _, k := range attrKeys {
		buf.WriteByte(' ')
		buf.WriteString(k)
		buf.WriteString(`="`)
		buf.WriteString(escapeAttr(attrs[k]))
		buf.WriteByte('"')
	}

	if !hasText && len(children) == 0 {
		buf.WriteString("/>")
		return nil
	}
	buf.WriteByte('>')

	if hasText {
		buf.WriteString(escapeText(text))
	}

	childPath := tag
	if ancestorPath != "" {
		childPath = ancestorPath + ":" + tag
	}
	order, ok := xsdorder.Resolve(ancestorPath, tag)

	keys := make([]string, 0, len(children))
	for k := range children {
		keys = append(keys, k)
	}
	if ok {
		rank := make(map[string]int, len(order))
		for i, name := range order {
			rank[name] = i
		}
		sort.Slice(keys, func(i, j int) bool {
			ri, iok := rank[keys[i]]
			if !iok {
				ri = len(order)
			}
			rj, jok := rank[keys[j]]
			if !jok {
				rj = len(order)
			}
			if ri != rj {
				return ri < rj
			}
			return keys[i] < keys[j]
		})
	} else {
		sort.Strings(keys)
	}

	for _, k := range keys {
		items, isList := asSliceAny(children[k])
		if !isList {
			if err := writeElement(buf, k, children[k], localNS, newDeclaredNS, childPath); err != nil {
				return err
			}
			continue
		}
		for _, item := range items {
			if err := writeElement(buf, k, item, localNS, newDeclaredNS, childPath); err != nil {
				return err
			}
		}
	}

	buf.WriteString("</")
	buf.WriteString(tag)
	buf.WriteByte('>')
	return nil
}

// asSliceAny reports whether v is a slice/array (any element type, matching
// Python's isinstance(v, list) duck typing) and returns its elements as
// []any. Strings are excluded (they are slices of bytes at the reflect
// level, but must be treated as scalar text).
func asSliceAny(v any) ([]any, bool) {
	if v == nil {
		return nil, false
	}
	if _, ok := v.(string); ok {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out, true
}

// stringifyScalar renders v (an attribute, #text, or bare element value) as
// its XML text, matching py-dfe's str(value) for str/int/float.
func stringifyScalar(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), nil
	case fmt.Stringer:
		return x.String(), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("xmlops: unsupported scalar type %T", v)
	}
}

var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;")
)

func escapeText(s string) string { return textEscaper.Replace(s) }
func escapeAttr(s string) string { return attrEscaper.Replace(s) }

// ParseXML parses an XML document (e.g. a SEFAZ SOAP response body) into a
// map keyed by the root element's local name, mirroring py-dfe's
// parse_xml_bytes/xml_to_dict: "@attr" keys for attributes, "#text" for an
// element's own text alongside attributes/children, repeated child tags
// collapsed to a single value when there is exactly one (a list otherwise),
// and a leaf element with neither attributes nor children returned as a
// plain string. Namespace URIs are dropped, matching py-dfe (only the local
// tag name survives).
func ParseXML(xmlBytes []byte) (map[string]any, error) {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("xmlops: parse xml: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		val, err := parseElement(dec, start)
		if err != nil {
			return nil, err
		}
		return map[string]any{start.Name.Local: val}, nil
	}
}

// parseElement consumes tokens up to and including start's matching
// EndElement and returns the dict/string value for it.
func parseElement(dec *xml.Decoder, start xml.StartElement) (any, error) {
	attribs := map[string]any{}
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue // namespace declaration, not a content attribute
		}
		attribs["@"+a.Name.Local] = a.Value
	}

	children := map[string][]any{}
	var childOrder []string
	var text strings.Builder

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("xmlops: parse xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			val, err := parseElement(dec, t)
			if err != nil {
				return nil, err
			}
			if _, seen := children[t.Name.Local]; !seen {
				childOrder = append(childOrder, t.Name.Local)
			}
			children[t.Name.Local] = append(children[t.Name.Local], val)
		case xml.EndElement:
			flat := map[string]any{}
			for _, k := range childOrder {
				v := children[k]
				if len(v) == 1 {
					flat[k] = v[0]
				} else {
					flat[k] = v
				}
			}
			trimmed := strings.TrimSpace(text.String())
			if len(attribs) == 0 && len(flat) == 0 {
				return trimmed, nil
			}
			result := make(map[string]any, len(attribs)+len(flat)+1)
			for k, v := range attribs {
				result[k] = v
			}
			for k, v := range flat {
				result[k] = v
			}
			if trimmed != "" {
				result["#text"] = trimmed
			}
			return result, nil
		}
	}
}
