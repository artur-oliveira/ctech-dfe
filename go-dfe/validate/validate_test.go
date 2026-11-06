package validate

import "testing"

func TestLoadAllSpecs(t *testing.T) {
	for _, n := range []string{"nfe", "mdfe", "cte", "cteos", "ctesimp", "mdfe_rodo", "mdfe_aereo", "mdfe_aquav", "mdfe_ferrov",
		"cte_rodo", "cte_rodoos", "cte_aereo", "cte_aquav", "cte_duto", "cte_multimodal"} {
		if _, err := Load(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	if _, err := Load("nope"); err == nil {
		t.Error("esperava erro para esquema inexistente")
	}
}

func codes(vs []Violation) map[string]bool {
	m := map[string]bool{}
	for _, v := range vs {
		m[v.Path+"|"+v.Code] = true
	}
	return m
}

func TestNFeSchemaFindings(t *testing.T) {
	s, err := Load("nfe")
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"infNFe": map[string]any{
			"@versao": "4.00",
			"@Id":     "NFe123",
			"ide":     map[string]any{"cUF": "35", "mod": "99", "bogus": "x"},
		},
	}
	got := codes(s.Validate(doc, Options{Skip: []string{"Signature"}}))
	for _, want := range []string{
		"NFe/infNFe/@Id|pattern",
		"NFe/infNFe/ide/mod|enum",
		"NFe/infNFe/ide/bogus|unknown_tag",
		"NFe/infNFe/ide/natOp|required",
		"NFe/infNFe/emit|required",
	} {
		if !got[want] {
			t.Errorf("faltou %s; got %v", want, got)
		}
	}
	if got["NFe/Signature|required"] {
		t.Error("Signature deveria ser ignorada")
	}
}

func TestCountDigits(t *testing.T) {
	cases := []struct {
		s       string
		tot, fr int
	}{{"10.50", 4, 2}, {"0.0001", 4, 4}, {"123", 3, 0}, {"0", 1, 0}}
	for _, c := range cases {
		if tt, f := countDigits(c.s); tt != c.tot || f != c.fr {
			t.Errorf("%s: %d,%d", c.s, tt, f)
		}
	}
}

func TestRulesWarnDefaultAndBlockOverride(t *testing.T) {
	doc := map[string]any{"infNFe": map[string]any{
		"det":   []any{map[string]any{"prod": map[string]any{"qCom": "2", "vUnCom": "10", "vProd": "25.00"}}},
		"total": map[string]any{"ICMSTot": map[string]any{"vProd": "25.00", "vNF": "25.00"}},
	}}
	f, blocking := Apply("nfe", doc, nil)
	if blocking || len(f) != 1 || f[0].Rule != "I11-vProd" || f[0].Severity != SevWarn {
		t.Fatalf("default warn: %+v blocking=%v", f, blocking)
	}
	_, blocking = Apply("nfe", doc, Modes{"I11-vProd": ModeBlock})
	if !blocking {
		t.Error("override block deveria bloquear")
	}
	if f, _ := Apply("nfe", doc, Modes{"I11-vProd": ModeOff}); len(f) != 0 {
		t.Errorf("off: %+v", f)
	}
}

func TestNFeOverlayAcceptsNTFields(t *testing.T) {
	s, _ := Load("nfe")
	doc := map[string]any{"infNFe": map[string]any{
		"emit": map[string]any{"tpRegimeIBSCBS": "1"},
		"det": map[string]any{"prod": map[string]any{"vUnComLiq": "10.5", "vProdLiq": "21.00", "cAtivSN": "123"},
			"imposto": map[string]any{"IBSCBS": map[string]any{"gTribSN": map[string]any{
				"gIBSSN": map[string]any{"pIBSSN": "0.1000", "vIBSSN": "0.02"}}}}},
		"total": map[string]any{"vProdLiqTot": "21.00", "IBSCBSTot": map[string]any{"gTribSN": map[string]any{"vIBSSN": "0.02"}}},
	}}
	for _, v := range s.Validate(doc, Options{}) {
		if v.Code == CodeUnknownTag {
			t.Errorf("overlay não cobre %s", v.Path)
		}
	}
	doc["infNFe"].(map[string]any)["emit"].(map[string]any)["tpRegimeIBSCBS"] = "9"
	if !codes(s.Validate(doc, Options{}))["NFe/infNFe/emit/tpRegimeIBSCBS|enum"] {
		t.Error("enum do overlay não vale")
	}
}
