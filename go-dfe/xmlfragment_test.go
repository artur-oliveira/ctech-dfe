package dfe

import "strings"

import "testing"

func TestBuildXMLFragment_ProtNFe(t *testing.T) {
	body := map[string]any{
		"@versao": "4.00",
		"infProt": map[string]any{
			"tpAmb":    "2",
			"chNFe":    "22260811647612000197550000000000501454670090",
			"dhRecbto": "2026-08-08T17:05:06-03:00",
			"nProt":    "322260000016670",
			"digVal":   "cKFyNtF4cg+d63/SRv0ezXGoef8=",
			"cStat":    "100",
			"xMotivo":  "Autorizado o uso da NF-e",
		},
	}
	out, err := BuildXMLFragment(body, "protNFe", "http://www.portalfiscal.inf.br/nfe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	xml := string(out)
	if !strings.Contains(xml, `<protNFe`) || !strings.Contains(xml, `versao="4.00"`) {
		t.Fatalf("missing root/attr: %s", xml)
	}
	if !strings.Contains(xml, `xmlns="http://www.portalfiscal.inf.br/nfe"`) {
		t.Fatalf("missing xmlns: %s", xml)
	}
	if !strings.Contains(xml, "<digVal>cKFyNtF4cg+d63/SRv0ezXGoef8=</digVal>") {
		t.Fatalf("missing digVal: %s", xml)
	}
}

func TestBuildXMLFragment_UnknownServiceStillBuilds(t *testing.T) {
	// BuildXMLFragment não faz lookup em dfe.Implements — é só serialização,
	// deve funcionar independente de qualquer gate de promoção.
	out, err := BuildXMLFragment(map[string]any{"#text": "x"}, "foo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "<foo>x</foo>" {
		t.Fatalf("got %s", out)
	}
}
