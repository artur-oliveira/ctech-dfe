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

func hasRule(vs []Violation, id string) bool {
	for _, v := range vs {
		if v.Rule == id {
			return true
		}
	}
	return false
}

func TestRulesWarnDefaultAndBlockOverride(t *testing.T) {
	doc := map[string]any{"infNFe": map[string]any{
		"ide": map[string]any{"mod": "55", "finNFe": "1"},
		"det": []any{map[string]any{"prod": map[string]any{"qCom": "2", "vUnCom": "10", "vProd": "25.00"}}},
	}}
	f, _ := Apply("nfe", doc, nil)
	if !hasRule(f, "I11-10") {
		t.Fatalf("I11-10 deveria disparar: %+v", f)
	}
	for _, x := range f {
		if x.Rule == "I11-10" && x.Severity != SevWarn {
			t.Errorf("padrão deve ser warn: %+v", x)
		}
	}
	if _, blocking := Apply("nfe", doc, Modes{"I11-10": ModeBlock}); !blocking {
		t.Error("override block deveria bloquear")
	}
	if f, _ := Apply("nfe", doc, Modes{"I11-10": ModeOff}); hasRule(f, "I11-10") {
		t.Error("off deveria silenciar")
	}
}

// Choice obrigatória com braços só de elementos opcionais (infNFe do CT-e) é satisfeita vazia; CNPJ|CPF não.
func TestOptionalArm(t *testing.T) {
	opt := &node{Children: []*node{{Name: "infUnidCarga", Choice: 1, Arm: 0}, {Name: "infUnidTransp", Choice: 1, Arm: 1}}}
	if !optionalArm(opt, 1) {
		t.Fatal("braços opcionais deveriam satisfazer a choice vazia")
	}
	req := &node{Children: []*node{{Name: "CNPJ", Min: 1, Choice: 1, Arm: 0}, {Name: "CPF", Min: 1, Choice: 1, Arm: 1}}}
	if optionalArm(req, 1) {
		t.Fatal("CNPJ|CPF exige uma das alternativas")
	}
}
