package xmlops

import (
	"reflect"
	"testing"
)

// TestBuildXML_SimpleFlat covers a flat element with no attributes/namespace
// (e.g. consStatServ children), mirroring py-dfe's test_simple_element /
// test_hash_text_key.
func TestBuildXML_SimpleFlat(t *testing.T) {
	body := map[string]any{
		"tpAmb": "2",
		"cUF":   "35",
		"xServ": "STATUS",
	}

	xmlBytes, err := BuildXML(body, "consStatServ", "")
	if err != nil {
		t.Fatalf("BuildXML: %v", err)
	}

	got, err := ParseXML(xmlBytes)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	want := map[string]any{"consStatServ": map[string]any{
		"tpAmb": "2",
		"cUF":   "35",
		"xServ": "STATUS",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

// TestBuildXML_AttributesAndNamespace covers @attr keys and the @xmlns
// default-namespace convention, mirroring py-dfe's
// test_attributes_from_at_prefix / test_namespace_from_xmlns / test_roundtrip.
func TestBuildXML_AttributesAndNamespace(t *testing.T) {
	body := map[string]any{
		"@versao": "4.00",
		"tpAmb":   "2",
		"cUF":     "35",
		"xServ":   "STATUS",
	}

	xmlBytes, err := BuildXML(body, "consStatServ", "http://www.portalfiscal.inf.br/nfe")
	if err != nil {
		t.Fatalf("BuildXML: %v", err)
	}

	// xsdorder's "consStatServ" entry mandates tpAmb, cUF, xServ (not
	// alphabetical) — this is the whole point of consulting the table
	// instead of falling back to Go's undefined map iteration order.
	const want = `<consStatServ xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><tpAmb>2</tpAmb><cUF>35</cUF><xServ>STATUS</xServ></consStatServ>`
	if string(xmlBytes) != want {
		t.Fatalf("BuildXML output mismatch:\n got=%s\nwant=%s", xmlBytes, want)
	}

	back, err := ParseXML(xmlBytes)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	wantDict := map[string]any{"consStatServ": map[string]any{
		"@versao": "4.00",
		"tpAmb":   "2",
		"cUF":     "35",
		"xServ":   "STATUS",
	}}
	if !reflect.DeepEqual(back, wantDict) {
		t.Fatalf("round trip mismatch:\n got=%#v\nwant=%#v", back, wantDict)
	}
}

// TestBuildXML_NestedRepeatedChildren covers nested dicts and a list value
// producing repeated sibling elements (multiple NF-e "det" line items),
// shaped after py-dfe/tests/integration/fiscal_payloads.py's build_nfe.
func TestBuildXML_NestedRepeatedChildren(t *testing.T) {
	body := map[string]any{
		"@versao": "4.00",
		"ide": map[string]any{
			"cUF": "35",
			"mod": "55",
			"nNF": "1",
		},
		"det": []any{
			map[string]any{
				"@nItem": "1",
				"prod": map[string]any{
					"cProd": "1",
					"xProd": "ITEM 1",
					"vProd": "10.00",
				},
			},
			map[string]any{
				"@nItem": "2",
				"prod": map[string]any{
					"cProd": "2",
					"xProd": "ITEM 2",
					"vProd": "20.00",
				},
			},
		},
	}

	xmlBytes, err := BuildXML(body, "infNFe", "http://www.portalfiscal.inf.br/nfe")
	if err != nil {
		t.Fatalf("BuildXML: %v", err)
	}

	got, err := ParseXML(xmlBytes)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	want := map[string]any{"infNFe": map[string]any{
		"@versao": "4.00",
		"ide": map[string]any{
			"cUF": "35",
			"mod": "55",
			"nNF": "1",
		},
		"det": []any{
			map[string]any{
				"@nItem": "1",
				"prod": map[string]any{
					"cProd": "1",
					"xProd": "ITEM 1",
					"vProd": "10.00",
				},
			},
			map[string]any{
				"@nItem": "2",
				"prod": map[string]any{
					"cProd": "2",
					"xProd": "ITEM 2",
					"vProd": "20.00",
				},
			},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got=%#v\nwant=%#v", got, want)
	}

	// infNFe's xsdorder entry orders ide before det ("ide", "emit", ...,
	// "det", ...) regardless of the map iteration order used to build body.
	idePos := indexOf(t, string(xmlBytes), "<ide>")
	detPos := indexOf(t, string(xmlBytes), "<det ")
	if idePos < 0 || detPos < 0 || idePos > detPos {
		t.Fatalf("expected <ide> before <det> per xsdorder, got: %s", xmlBytes)
	}
}

// TestParseXML_XMLToDictToXML starts from raw XML (as SEFAZ would send it in
// a SOAP response), parses to dict, and rebuilds it, verifying the dict shape
// used by worker/internal/service/distribution.go (asMap/asSlice on
// retDistDFeInt/loteDistDFeInt/docZip).
func TestParseXML_XMLToDictToXML(t *testing.T) {
	xmlIn := []byte(`<retDistDFeInt versao="1.01"><cStat>138</cStat><xMotivo>Documento localizado</xMotivo><ultNSU>10</ultNSU><maxNSU>20</maxNSU><loteDistDFeInt><docZip NSU="11" schema="resNFe_v1.01.xsd">AAAA</docZip><docZip NSU="12" schema="resNFe_v1.01.xsd">BBBB</docZip></loteDistDFeInt></retDistDFeInt>`)

	dict, err := ParseXML(xmlIn)
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}

	ret, ok := dict["retDistDFeInt"].(map[string]any)
	if !ok {
		t.Fatalf("retDistDFeInt not a map: %#v", dict["retDistDFeInt"])
	}
	if ret["cStat"] != "138" {
		t.Fatalf("cStat = %v, want 138", ret["cStat"])
	}
	lote, ok := ret["loteDistDFeInt"].(map[string]any)
	if !ok {
		t.Fatalf("loteDistDFeInt not a map: %#v", ret["loteDistDFeInt"])
	}
	docZips, ok := lote["docZip"].([]any)
	if !ok || len(docZips) != 2 {
		t.Fatalf("docZip = %#v, want a 2-element list", lote["docZip"])
	}
	first, ok := docZips[0].(map[string]any)
	if !ok || first["@NSU"] != "11" || first["#text"] != "AAAA" {
		t.Fatalf("docZip[0] = %#v", docZips[0])
	}

	// Rebuild from the parsed dict and confirm the content survives a second
	// parse (XML -> dict -> XML -> dict).
	body, _ := dict["retDistDFeInt"].(map[string]any)
	xmlOut, err := BuildXML(body, "retDistDFeInt", "")
	if err != nil {
		t.Fatalf("BuildXML: %v", err)
	}
	dict2, err := ParseXML(xmlOut)
	if err != nil {
		t.Fatalf("ParseXML (second pass): %v", err)
	}
	if !reflect.DeepEqual(dict, dict2) {
		t.Fatalf("XML->dict->XML->dict mismatch:\n first=%#v\nsecond=%#v", dict, dict2)
	}
}

func indexOf(t *testing.T, s, sub string) int {
	t.Helper()
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
