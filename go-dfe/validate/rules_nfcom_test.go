package validate

import (
	"fmt"
	"testing"
)

func tNFComKey() string {
	return tKey("35", "2611", tCNPJ1, "62", "001", "000000010", "1", "0"+"1234567")
}

// tNFComGood devolve o conteúdo de <NFCom> de uma nota normal que não dispara nenhuma regra.
func tNFComGood() m {
	key := tNFComKey()
	return m{"infNFCom": m{
		"@Id": "NFCom" + key,
		"ide": m{
			"cUF": "35", "tpAmb": "2", "mod": "62", "serie": "1", "nNF": "10", "cNF": "1234567", "cDV": key[43:],
			"dhEmi": "2026-11-05T10:00:00-03:00", "tpEmis": "1", "nSiteAutoriz": "0", "finNFCom": "0", "tpFat": "0",
		},
		"emit":      m{"CNPJ": tCNPJ1, "IE": "111111111111"},
		"dest":      m{"xNome": "X", "CNPJ": tCNPJ2, "indIEDest": "9"},
		"assinante": m{"dContratoIni": "2026-01-01", "dContratoFim": "2026-12-31"},
		"det":       []any{m{"prod": m{"qFaturada": "2.0000", "vItem": "10.00", "vDesc": "1.00", "vOutro": "0.50", "vProd": "19.50"}}},
		"gFat":      m{"CompetFat": "202611"},
	}}
}

func nfcomFinding(t *testing.T, doc m, id string) bool {
	t.Helper()
	for _, r := range Rules("nfcom") {
		if r.ID == id {
			return len(r.Check(doc)) > 0
		}
	}
	t.Fatalf("regra %s não registrada", id)
	return false
}

func TestNFComGoodDocHasNoFindings(t *testing.T) {
	found, blocking := Apply("nfcom", tNFComGood(), nil)
	if len(found) != 0 || blocking {
		t.Fatalf("achados inesperados: %+v", found)
	}
}

func TestNFComRules(t *testing.T) {
	inf := func(d m) m { return d["infNFCom"].(m) }
	cases := []struct {
		id     string
		mutate func(d m)
	}{
		{"G04", func(d m) { inf(d)["ide"].(m)["dhCont"] = "2026-11-05T09:00:00-03:00" }},
		{"G05", func(d m) { inf(d)["ide"].(m)["tpEmis"] = "2" }},
		{"G06", func(d m) {
			i := inf(d)["ide"].(m)
			i["tpEmis"], i["dhCont"], i["xJust"] = "2", "2026-11-05T11:00:00-03:00", "queda do link de dados"
		}},
		{"G08", func(d m) {
			i := inf(d)["ide"].(m)
			i["tpEmis"], i["dhCont"], i["xJust"], i["finNFCom"] = "2", "2026-11-05T09:00:00-03:00", "queda do link de dados", "3"
		}},
		{"G09", func(d m) {
			inf(d)["@Id"] = "NFCom" + tKey("35", "2611", tCNPJ2, "62", "001", "000000010", "1", "01234567")
		}},
		{"G10", func(d m) {
			inf(d)["@Id"] = "NFCom" + tKey("35", "2101", tCNPJ1, "62", "001", "000000010", "1", "01234567")
		}},
		{"G11", func(d m) { k := tNFComKey(); inf(d)["@Id"] = "NFCom" + k[:43] + fmt.Sprint((int(k[43]-'0')+1)%10) }},
		{"G12", func(d m) { inf(d)["emit"].(m)["CNPJ"] = "11222333000182" }},
		{"G13", func(d m) { delete(inf(d)["emit"].(m), "IE") }},
		{"G22", func(d m) { inf(d)["dest"].(m)["CNPJ"] = "11444777000162" }},
		{"G23", func(d m) { x := inf(d)["dest"].(m); delete(x, "CNPJ"); x["CPF"] = "52998224726" }},
		{"G24", func(d m) { x := inf(d)["dest"].(m); delete(x, "CNPJ"); x["idOutros"] = "P123"; x["IE"] = "ISENTO" }},
		{"G25", func(d m) { x := inf(d)["dest"].(m); x["indIEDest"] = "1"; x["IE"] = "ISENTO" }},
		{"G26", func(d m) { x := inf(d)["dest"].(m); x["indIEDest"] = "2"; x["IE"] = "123456" }},
		{"G27", func(d m) { inf(d)["dest"].(m)["IE"] = "123456" }},
		{"G35", func(d m) { inf(d)["assinante"].(m)["dContratoFim"] = "2025-12-31" }},
		{"G43", func(d m) { inf(d)["ide"].(m)["finNFCom"] = "3" }},
		{"G44", func(d m) { inf(d)["gSub"] = m{"chNFCom": tNFComKey(), "motSub": "01"} }},
		{"G67", func(d m) { inf(d)["ide"].(m)["tpFat"] = "2"; delete(inf(d), "gFat") }},
		{"G68", func(d m) { inf(d)["gCofat"] = m{"chNFComLocal": tNFComKey()} }},
		{"G78", func(d m) { delete(inf(d), "gFat") }},
		{"G79", func(d m) { inf(d)["ide"].(m)["tpFat"] = "1" }},
		{"G80", func(d m) { inf(d)["ide"].(m)["finNFCom"] = "4"; inf(d)["gFatCentral"] = m{"CNPJ": tCNPJ1} }},
		{"G86", func(d m) { inf(d)["det"].([]any)[0].(m)["prod"].(m)["vProd"] = "25.00" }},
	}
	for _, c := range cases {
		good := tNFComGood()
		if nfcomFinding(t, good, c.id) {
			t.Errorf("%s: documento bom já dispara a regra", c.id)
			continue
		}
		bad := tNFComGood()
		c.mutate(bad)
		if !nfcomFinding(t, bad, c.id) {
			t.Errorf("%s: documento ruim não dispara a regra", c.id)
		}
	}
}

func TestNFComRuleMetadata(t *testing.T) {
	want := map[string]int{"G04": 415, "G05": 416, "G09": 227, "G11": 253, "G12": 207, "G22": 422, "G86": 435}
	got := map[string]Rule{}
	for _, r := range Rules("nfcom") {
		got[r.ID] = r
	}
	for id, cs := range want {
		if got[id].CStat != cs || got[id].Doc != "nfcom" {
			t.Errorf("%s: cStat=%d doc=%q", id, got[id].CStat, got[id].Doc)
		}
	}
	for _, id := range []string{"G09", "G11", "G12", "G22", "G23"} {
		if got[id].Mode != ModeBlock {
			t.Errorf("%s deveria bloquear (rejeição certa e determinística)", id)
		}
	}
	if got["G86"].Mode != ModeWarn {
		t.Error("G86 deveria apenas avisar")
	}
}
