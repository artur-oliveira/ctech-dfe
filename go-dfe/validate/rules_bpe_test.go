package validate

import "testing"

func tBPeKey() string {
	return tKey("35", "2611", tCNPJ1, "63", "001", "000000010", "1", "01234567")
}

// tBPeGood: conteúdo de <BPe> de um bilhete normal que não dispara nenhuma regra.
func tBPeGood() m {
	key := tBPeKey()
	return m{"infBPe": m{
		"@Id": "BPe" + key,
		"ide": m{
			"cUF": "35", "tpAmb": "2", "mod": "63", "serie": "1", "nBP": "10", "cBP": "01234567", "cDV": key[43:],
			"modal": "1", "dhEmi": "2026-11-05T10:00:00-03:00", "tpEmis": "1", "tpBPe": "0", "UFIni": "SP", "UFFim": "RJ",
		},
		"emit":        m{"CNPJ": tCNPJ1, "enderEmit": m{"UF": "SP"}},
		"comp":        m{"CNPJ": tCNPJ2},
		"infPassagem": m{"dhEmb": "2026-11-20T10:00:00-03:00", "infPassageiro": m{"CPF": tCPF1}},
		"infViagem":   []any{m{"tpTrecho": "1", "dhViagem": "2026-11-20T10:00:00-03:00"}},
		"infValorBPe": m{
			"vBP": "100.00", "vDesconto": "10.00", "vPgto": "90.00", "vTroco": "0.00", "tpDesconto": "01", "xDesconto": "Idoso",
			"Comp": []any{m{"tpComp": "01", "vComp": "60.00"}, m{"tpComp": "02", "vComp": "40.00"}},
		},
		"pag": []any{m{"tPag": "01", "vPag": "90.00"}},
	}}
}

func bpeFinding(t *testing.T, doc m, id string) bool {
	t.Helper()
	for _, r := range Rules("bpe") {
		if r.ID == id {
			return len(r.Check(doc)) > 0
		}
	}
	t.Fatalf("regra %s não registrada", id)
	return false
}

func TestBPeGoodDocHasNoFindings(t *testing.T) {
	if found, blocking := Apply("bpe", tBPeGood(), nil); len(found) != 0 || blocking {
		t.Fatalf("achados inesperados: %+v", found)
	}
}

func TestBPeRules(t *testing.T) {
	inf := func(d m) m { return d["infBPe"].(m) }
	ide := func(d m) m { return inf(d)["ide"].(m) }
	val := func(d m) m { return inf(d)["infValorBPe"].(m) }
	cases := []struct {
		id     string
		mutate func(d m)
	}{
		{"F05", func(d m) { ide(d)["dhCont"] = "2026-11-05T09:00:00-03:00" }},
		{"F06", func(d m) { ide(d)["tpEmis"] = "2" }},
		{"F07", func(d m) {
			i := ide(d)
			i["tpEmis"], i["dhCont"], i["xJust"] = "2", "2026-11-05T11:00:00-03:00", "queda do link de dados"
		}},
		{"F09", func(d m) {
			inf(d)["@Id"] = "BPe" + tKey("35", "2611", tCNPJ2, "63", "001", "000000010", "1", "01234567")
		}},
		{"F10", func(d m) {
			inf(d)["@Id"] = "BPe" + tKey("35", "2401", tCNPJ1, "63", "001", "000000010", "1", "01234567")
		}},
		{"F11", func(d m) { k := tBPeKey(); inf(d)["@Id"] = "BPe" + k[:43] + string(rune('0'+(int(k[43]-'0')+1)%10)) }},
		{"F13", func(d m) { ide(d)["UFIni"] = "MG" }},
		{"F18", func(d m) { inf(d)["infPassagem"].(m)["infPassageiro"].(m)["CPF"] = "52998224726" }},
		{"F21", func(d m) { inf(d)["infViagem"].([]any)[0].(m)["tpTrecho"] = "3" }},
		{"F22", func(d m) { inf(d)["infViagem"].([]any)[0].(m)["dhConexao"] = "2026-11-20T12:00:00-03:00" }},
		{"F23", func(d m) {
			v := inf(d)["infViagem"].([]any)[0].(m)
			v["tpTrecho"], v["dhConexao"] = "3", "2026-11-20T09:00:00-03:00"
		}},
		{"F24", func(d m) { inf(d)["emit"].(m)["CNPJ"] = "11222333000182" }},
		{"F34", func(d m) { inf(d)["comp"].(m)["CNPJ"] = "11444777000162" }},
		{"F35", func(d m) { c := inf(d)["comp"].(m); delete(c, "CNPJ"); c["CPF"] = "52998224726" }},
		{"F49", func(d m) { inf(d)["infPassagem"].(m)["dhEmb"] = "2026-11-01T10:00:00-03:00" }},
		{"F64", func(d m) { val(d)["Comp"].([]any)[0].(m)["vComp"] = "10.00" }},
		{"F65", func(d m) { delete(val(d), "xDesconto") }},
		{"F66", func(d m) { val(d)["vDesconto"] = "0.00"; val(d)["vPgto"] = "100.00" }},
		{"F67", func(d m) { delete(val(d), "tpDesconto") }},
		{"F69", func(d m) { inf(d)["pag"].([]any)[0].(m)["vPag"] = "50.00" }},
		{"F70", func(d m) { val(d)["vPgto"] = "50.00"; inf(d)["pag"].([]any)[0].(m)["vPag"] = "50.00" }},
	}
	for _, c := range cases {
		if bpeFinding(t, tBPeGood(), c.id) {
			t.Errorf("%s: documento bom já dispara a regra", c.id)
			continue
		}
		bad := tBPeGood()
		c.mutate(bad)
		if !bpeFinding(t, bad, c.id) {
			t.Errorf("%s: documento ruim não dispara a regra", c.id)
		}
	}
}

// TM e TA usam as mesmas regras do comum; o TA soma as de IBS/CBS (rules_bpe_ibscbs.go).
func TestBPeRulesCoverVariants(t *testing.T) {
	if len(Rules("bpetm")) != len(Rules("bpe")) {
		t.Errorf("bpetm: %d regras, bpe tem %d", len(Rules("bpetm")), len(Rules("bpe")))
	}
	if len(Rules("bpeta")) != len(Rules("bpe"))+len(ibsRules()) {
		t.Errorf("bpeta: %d regras, esperado %d", len(Rules("bpeta")), len(Rules("bpe"))+len(ibsRules()))
	}
}

func TestBPeRuleMetadata(t *testing.T) {
	want := map[string]int{"F05": 415, "F06": 416, "F09": 227, "F11": 253, "F24": 207, "F34": 422, "F64": 436, "F69": 438}
	got := map[string]Rule{}
	for _, r := range Rules("bpe") {
		got[r.ID] = r
	}
	for id, cs := range want {
		if got[id].CStat != cs {
			t.Errorf("%s: cStat=%d, queria %d", id, got[id].CStat, cs)
		}
	}
	for _, id := range []string{"F09", "F11", "F18", "F24", "F34", "F35"} {
		if got[id].Mode != ModeBlock {
			t.Errorf("%s deveria bloquear", id)
		}
	}
	if got["F64"].Mode != ModeWarn {
		t.Error("F64 deveria apenas avisar")
	}
}
