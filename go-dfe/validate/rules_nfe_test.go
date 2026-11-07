package validate

import (
	"fmt"
	"strings"
	"testing"
)

const (
	nfeTCNPJ  = "11222333000181" // CNPJ de exemplo com DV válido (não é de empresa real)
	nfeTCNPJ2 = "99999999000191"
	nfeTCPF   = "52998224725"
)

func nfeTKey(uf, aamm, doc, mod, serie, nNF, tpEmis, cNF string) string {
	k := uf + aamm + doc + mod + serie + nNF + tpEmis + cNF
	return k + fmt.Sprint(nfeKeyDV(k))
}

// nfeTSet grava v em m[path] (criando grupos); v nil remove. Lista ([]any) entra pelo primeiro elemento.
func nfeTSet(m map[string]any, path string, v any) {
	parts := strings.Split(path, "/")
	cur := m
	for _, p := range parts[:len(parts)-1] {
		switch t := cur[p].(type) {
		case map[string]any:
			cur = t
		case []any:
			cur = t[0].(map[string]any)
		default:
			n := map[string]any{}
			cur[p] = n
			cur = n
		}
	}
	last := parts[len(parts)-1]
	if v == nil {
		delete(cur, last)
		return
	}
	cur[last] = v
}

// nfeTBase devolve um documento válido (modelo 55 ou 65) que não dispara nenhuma regra.
func nfeTBase(mod string) (doc, inf map[string]any) {
	serie, nNF, cNF, tpImp := "001", "000000123", "87654321", "1"
	if mod == "65" {
		tpImp = "4"
	}
	key := nfeTKey("35", "2611", nfeTCNPJ, mod, serie, nNF, "1", cNF)
	inf = map[string]any{
		"@Id": "NFe" + key,
		"ide": map[string]any{"cUF": "35", "cNF": cNF, "mod": mod, "serie": "1", "nNF": "123", "dhEmi": "2026-11-05T10:00:00-03:00",
			"tpNF": "1", "idDest": "1", "cMunFG": "3550308", "tpImp": tpImp, "tpEmis": "1", "tpAmb": "1", "finNFe": "1",
			"indFinal": "1", "indPres": "1", "indIntermed": "0", "procEmi": "0"},
		"emit": map[string]any{"CNPJ": nfeTCNPJ, "IE": "123456789012", "CRT": "3",
			"enderEmit": map[string]any{"cMun": "3550308", "UF": "SP"}},
		"det": []any{map[string]any{"@nItem": "1",
			"prod": map[string]any{"CFOP": "5102", "cEAN": "SEM GTIN", "cEANTrib": "SEM GTIN", "NCM": "12345678",
				"qCom": "2.0000", "vUnCom": "10.00", "vProd": "20.00", "qTrib": "2.0000", "vUnTrib": "10.00", "indTot": "1"},
			"imposto": map[string]any{
				"ICMS":   map[string]any{"ICMS00": map[string]any{"orig": "0", "CST": "00", "modBC": "3", "vBC": "20.00", "pICMS": "18.00", "vICMS": "3.60"}},
				"PIS":    map[string]any{"PISNT": map[string]any{"CST": "07"}},
				"COFINS": map[string]any{"COFINSNT": map[string]any{"CST": "07"}},
			}}},
		"total": map[string]any{"ICMSTot": map[string]any{"vBC": "20.00", "vICMS": "3.60", "vICMSDeson": "0.00", "vBCST": "0.00", "vST": "0.00",
			"vProd": "20.00", "vFrete": "0.00", "vSeg": "0.00", "vDesc": "0.00", "vII": "0.00", "vIPI": "0.00", "vPIS": "0.00",
			"vCOFINS": "0.00", "vOutro": "0.00", "vNF": "20.00"}},
		"transp": map[string]any{"modFrete": "9"},
		"pag":    map[string]any{"detPag": []any{map[string]any{"tPag": "01", "vPag": "20.00"}}},
	}
	doc = map[string]any{"infNFe": inf}
	if mod == "55" {
		inf["dest"] = map[string]any{"CPF": nfeTCPF, "xNome": "Fulano", "indIEDest": "9",
			"enderDest": map[string]any{"cMun": "3550308", "UF": "SP", "cPais": "1058"}}
	} else {
		doc["infNFeSupl"] = map[string]any{"qrCode": "https://example.invalid/qr?p=" + key + "|2|1|1|abcdef"}
	}
	return doc, inf
}

func TestNFeBaseDocsClean(t *testing.T) {
	for _, mod := range []string{"55", "65"} {
		doc, _ := nfeTBase(mod)
		for _, r := range nfeRules {
			if msgs := r.Check(doc); len(msgs) > 0 {
				t.Errorf("mod %s: regra %s dispara no documento válido: %v", mod, r.ID, msgs)
			}
		}
	}
}

func TestNFeRuleCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range nfeRules {
		if seen[r.ID] {
			t.Errorf("id duplicado %s", r.ID)
		}
		seen[r.ID] = true
		if r.CStat == 0 || r.Doc != "nfe" || r.Check == nil || (r.Mode != ModeWarn && r.Mode != ModeBlock) {
			t.Errorf("regra %s mal formada", r.ID)
		}
	}
	if len(nfeRules) < 60 {
		t.Errorf("catálogo NF-e com %d regras; esperado ao menos 60", len(nfeRules))
	}
}

type nfeCase struct {
	id, mod string
	mut     func(doc, inf map[string]any)
}

func nfeM(path string, v any) func(doc, inf map[string]any) {
	return func(doc, inf map[string]any) { nfeTSet(inf, path, v) }
}

func nfeMICMS(g map[string]any) func(doc, inf map[string]any) {
	return func(doc, inf map[string]any) { nfeTSet(inf, "det/imposto/ICMS", g) }
}

func nfeMMulti(fs ...func(doc, inf map[string]any)) func(doc, inf map[string]any) {
	return func(doc, inf map[string]any) {
		for _, f := range fs {
			f(doc, inf)
		}
	}
}

func nfeMRefs(tag string, keys ...string) func(doc, inf map[string]any) {
	return func(doc, inf map[string]any) {
		var l []any
		for _, k := range keys {
			l = append(l, map[string]any{tag: k})
		}
		nfeTSet(inf, "ide/NFref", l)
	}
}

func nfeCases() []nfeCase {
	own := nfeTKey("35", "2611", nfeTCNPJ, "55", "001", "000000123", "1", "87654321")
	k55 := nfeTKey("35", "2610", nfeTCNPJ, "55", "001", "000000001", "1", "12345678")
	bad := k55[:43] + "0"
	if k55[43] == '0' {
		bad = k55[:43] + "1"
	}
	k57 := nfeTKey("35", "2610", nfeTCNPJ, "57", "001", "000000001", "1", "12345678")
	tot := func(tag, v string) func(doc, inf map[string]any) { return nfeM("total/ICMSTot/"+tag, v) }
	pag := func(f map[string]any) func(doc, inf map[string]any) {
		return nfeM("pag/detPag", []any{f})
	}
	sn := func(grp, csosn string) map[string]any {
		return map[string]any{grp: map[string]any{"orig": "0", "CSOSN": csosn}}
	}
	cst := func(cst string, extra map[string]any) map[string]any {
		g := map[string]any{"orig": "0", "CST": cst}
		for k, v := range extra {
			g[k] = v
		}
		return map[string]any{"ICMS" + cst: g}
	}
	issqn := nfeM("det/imposto/ISSQN", map[string]any{"vBC": "20.00"})
	itot := func(path string) func(doc, inf map[string]any) { return nfeM("total/IBSCBSTot/"+path, "5.00") }
	sq := func(tag string) func(doc, inf map[string]any) { return nfeM("total/ISSQNtot/"+tag, "5.00") }
	nfref := func(f map[string]any) func(doc, inf map[string]any) { return nfeM("ide/NFref", []any{f}) }
	qr := func(v string) func(doc, inf map[string]any) {
		return func(doc, inf map[string]any) { doc["infNFeSupl"] = map[string]any{"qrCode": v} }
	}
	return []nfeCase{
		// identificação
		{"A03-10", "55", nfeM("ide/cNF", "11111112")},
		{"B23-10", "55", nfeM("@Id", "NFe"+own[:43]+string(rune('0'+(own[43]-'0'+1)%10)))},
		{"B03-10", "55", nfeM("ide/cNF", "12345678")},
		{"B26-10", "55", nfeM("ide/serie", "900")},
		{"B26-20", "55", nfeMMulti(nfeM("ide/procEmi", "1"), nfeM("ide/serie", "5"))},
		{"B26-24", "55", nfeMMulti(nfeM("ide/procEmi", "4"), nfeM("ide/serie", "5"))},
		{"B26-26", "55", nfeM("ide/procEmi", "4")},
		{"B26-28", "55", nfeM("infPAA", map[string]any{})},
		{"B26-30", "55", nfeMMulti(nfeM("ide/procEmi", "1"), nfeM("ide/serie", "900"), nfeM("ide/tpEmis", "2"))},
		{"B26-50", "65", nfeM("ide/procEmi", "3")},
		{"B28-10", "55", nfeM("ide/xJust", "teste de justificativa")},
		{"B28-20", "55", nfeM("ide/tpEmis", "9")},
		{"B22-10", "55", nfeMMulti(nfeM("ide/tpEmis", "9"), nfeM("ide/tpImp", "1"))},
		{"B21-20", "55", nfeM("ide/tpImp", "4")},
		{"B21-10", "65", nfeM("ide/tpImp", "1")},
		{"B22-34", "65", nfeM("ide/tpEmis", "2")},
		{"D01-10", "55", nfeM("avulsa", map[string]any{})},
		{"D01-20", "55", nfeM("ide/procEmi", "1")},
		{"B12-20", "55", nfeM("ide/cMunFG", "4150308")},
		{"C10-20", "55", nfeM("emit/enderEmit/cMun", "4150308")},
		{"B10-40", "55", nfeM("ide/dhSaiEnt", "2026-11-04T10:00:00-03:00")},
		{"B10a-30", "55", nfeMMulti(nfeM("ide/dPrevEntrega", "2026-11-01"))},
		{"B10a-20", "55", nfeM("ide/dPrevEntrega", "2027-03-01")},
		{"B10a-40", "55", nfeMMulti(nfeM("ide/dPrevEntrega", "2026-11-10"), nfeM("ide/finNFe", "2"))},
		{"B10a-50", "55", nfeMMulti(nfeM("ide/dPrevEntrega", "2026-11-10"), nfeM("transp/modFrete", "1"))},
		{"B25-30", "55", nfeM("ide/finNFe", "2")},
		{"B25-40", "55", nfeMMulti(nfeM("ide/finNFe", "4"), nfeMRefs("refNFe", k55, k55[:43]+"0"))},
		{"B25-65", "55", nfeMMulti(nfeM("ide/tpNFCredito", "02"), nfeMRefs("refNFe", k55))},
		{"B25-110", "55", nfeMMulti(nfeM("ide/finNFe", "5"), nfeM("ide/tpNF", "1"))},
		{"B25-120", "55", nfeMMulti(nfeM("ide/finNFe", "6"), nfeM("ide/tpNF", "0"))},
		{"B25-80", "55", nfeM("ide/finNFe", "5")},
		{"B25.1-10", "55", nfeM("ide/tpNFDebito", "01")},
		{"B25.1-20", "55", nfeM("ide/finNFe", "6")},
		{"B25.2-10", "55", nfeM("ide/tpNFCredito", "01")},
		{"B25.2-20", "55", nfeMMulti(nfeM("ide/finNFe", "5"), nfeM("ide/tpNF", "0"))},
		{"B25.2-30", "55", nfeM("ide/tpNFCredito", "02")},
		{"B25.2-40", "55", nfeM("ide/tpNFCredito", "03")},
		{"B25-100", "55", nfeMMulti(nfeM("ide/finNFe", "5"), nfeMRefs("refNFe", k57))},
		{"E16a-30", "55", nfeM("dest/indIEDest", "2")},
		{"B25-90", "55", nfeM("det/imposto/ICMS", nil)},
		{"B34-10", "55", nfeM("ide/gCompraGov", map[string]any{"tpOperGov": "2"})},
		{"B25b-40", "55", nfeMMulti(nfeM("ide/indPres", "5"))},
		{"B25c-10", "55", nfeM("ide/indIntermed", nil)},
		{"B25c-20", "55", nfeMMulti(nfeM("ide/indPres", "0"))},
		{"YB01-10", "55", nfeM("ide/indIntermed", "1")},
		{"YB01-20", "55", nfeM("infIntermed", map[string]any{"CNPJ": nfeTCNPJ})},
		{"YB02-10", "55", nfeMMulti(nfeM("ide/indIntermed", "1"), nfeM("infIntermed", map[string]any{"CNPJ": "11222333000182"}))},
		{"ZE01-10", "55", nfeM("infSolicNFF", map[string]any{})},
		{"ZE01-20", "55", nfeM("ide/tpEmis", "3")},
		{"ZD02-10", "55", nfeM("infRespTec", map[string]any{"CNPJ": "11222333000182"})},
		{"ZA01-20", "55", nfeM("exporta", map[string]any{})},
		{"H02-10", "55", nfeM("det/@nItem", "2")},
		// NFC-e / DANFE tipo 2
		{"B10-10", "65", nfeM("ide/dhSaiEnt", "2026-11-05T10:00:00-03:00")},
		{"B11-10", "65", nfeM("ide/tpNF", "0")},
		{"B11a-10", "65", nfeM("ide/idDest", "2")},
		{"B25-20", "65", nfeM("ide/finNFe", "3")},
		{"B25a-10", "65", nfeM("ide/indFinal", "0")},
		{"B25b-20", "65", nfeM("ide/indPres", "2")},
		{"BA01-10", "65", nfeMRefs("refNFe", k55)},
		{"E01-20", "65", nfeM("ide/indPres", "4")},
		{"X02-10", "65", nfeM("transp/modFrete", "0")},
		{"I17b-10", "65", nfeM("det/prod/indTot", "0")},
		{"C18-10", "65", nfeM("emit/IEST", "123456789012")},
		{"W16-30", "65", nfeM("total/ICMSTot/vNF", "300000.00")},
		{"W16-60", "65", nfeM("total/ICMSTot/vNF", "20000.00")},
		{"F01-10", "65", nfeM("retirada", map[string]any{})},
		{"G01-10", "65", nfeM("entrega", map[string]any{})},
		{"X11-10", "65", nfeM("transp/retTransp", map[string]any{})},
		{"X18-10", "65", nfeM("transp/veicTransp", map[string]any{})},
		{"X22-10", "65", nfeM("transp/reboque", map[string]any{})},
		{"X25a-10", "65", nfeM("transp/vagao", "X")},
		{"X25b-10", "65", nfeM("transp/balsa", "X")},
		{"Y01-10", "65", nfeM("cobr", map[string]any{})},
		{"ZB01-10", "65", nfeM("compra", map[string]any{})},
		{"ZC01-10", "65", nfeM("cana", map[string]any{})},
		{"J01-10", "65", nfeM("det/prod/veicProd", map[string]any{})},
		{"L01-10", "65", nfeM("det/prod/arma", map[string]any{})},
		{"O01-10", "65", nfeM("det/imposto/IPI", map[string]any{})},
		{"P01-10", "65", nfeM("det/imposto/II", map[string]any{})},
		{"R01-10", "65", nfeM("det/imposto/PISST", map[string]any{})},
		{"T01-10", "65", nfeM("det/imposto/COFINSST", map[string]any{})},
		{"UA01-20", "65", nfeM("det/impostoDevol", map[string]any{})},
		{"NA01-10", "65", nfeM("det/imposto/ICMSUFDest", map[string]any{})},
		{"N12-50", "65", nfeMICMS(map[string]any{"ICMSPart": map[string]any{"CST": "10"}})},
		{"N12-60", "65", nfeMICMS(map[string]any{"ICMSST": map[string]any{"CST": "41"}})},
		{"B10a-10", "65", nfeM("ide/dPrevEntrega", "2026-11-10")},
		{"B25d-10", "65", nfeM("ide/cIndOp", "010101")},
		{"B25b-60", "65", nfeM("ide/cMunFGIBS", "3550308")},
		{"B25b-70", "65", nfeM("ide/indPres", "5")},
		{"C17-42", "65", nfeM("emit/IE", nil)},
		{"E16a-10", "65", nfeM("dest", map[string]any{"indIEDest": "1"})},
		{"E17-10", "65", nfeMMulti(nfeM("ide/idDest", "3"), nfeM("dest", map[string]any{"IE": "123456789012"}))},
		{"E18-10", "65", nfeM("dest", map[string]any{"ISUF": "123456789"})},
		{"E02-20", "65", nfeM("dest", map[string]any{"CNPJ": nfeTCNPJ})},
		{"E05-20", "65", nfeM("ide/indPres", "4")},
		{"X03-10", "65", nfeM("transp/transporta", map[string]any{"CNPJ": nfeTCNPJ2})},
		{"X03-20", "65", nfeM("ide/indPres", "4")},
		{"ZA01-30", "65", nfeM("exporta", map[string]any{})},
		{"BB01-10", "65", nfeM("ide/gCompraGov", map[string]any{})},
		{"C02a-04", "65", nfeMMulti(nfeM("emit/CNPJ", nil), nfeM("emit/CPF", nfeTCPF))},
		{"W16-40", "65", nfeM("total/ICMSTot/vNF", "20000.00")},
		{"YA09-20", "55", nfeM("pag/vTroco", "400000.00")},
		{"I08-150", "65", nfeM("det/prod/CFOP", "5201")},
		{"N12-30", "65", nfeMICMS(cst("10", nil))},
		{"N12a-20", "65", nfeMMulti(nfeM("emit/CRT", "1"), nfeMICMS(sn("ICMSSN101", "101")))},
		{"N12-34", "65", nfeMICMS(cst("90", map[string]any{"modBCST": "4"}))},
		{"N12a-34", "65", nfeMMulti(nfeM("emit/CRT", "1"), nfeMICMS(sn("ICMSSN900", "900")), nfeM("det/imposto/ICMS/ICMSSN900/modBCST", "4"))},
		{"N12-40", "65", nfeM("det/prod/CFOP", "5933")},
		{"N12-44", "65", nfeMICMS(cst("60", nil))},
		{"N12a-40", "65", nfeMMulti(nfeM("emit/CRT", "1"), nfeMICMS(sn("ICMSSN102", "102")), nfeM("det/prod/CFOP", "5933"))},
		{"N12a-44", "65", nfeMMulti(nfeM("emit/CRT", "1"), nfeMICMS(sn("ICMSSN500", "500")))},
		{"N12a-81", "65", nfeMMulti(nfeM("emit/CRT", "4"), nfeMICMS(sn("ICMSSN500", "500")))},
		// participantes
		{"C02-10", "55", nfeM("emit/CNPJ", "11222333000182")},
		{"C02a-20", "55", nfeMMulti(nfeM("emit/CNPJ", nil), nfeM("emit/CPF", "11111111111"), nfeM("ide/serie", "900"))},
		{"E02-10", "55", nfeM("dest", map[string]any{"CNPJ": "11222333000182", "indIEDest": "9"})},
		{"E03-10", "55", nfeM("dest/CPF", "52998224726")},
		{"F02-10", "55", nfeM("retirada", map[string]any{"CNPJ": "11222333000182"})},
		{"F02a-10", "55", nfeM("retirada", map[string]any{"CPF": "00000000000"})},
		{"G02-10", "55", nfeM("entrega", map[string]any{"CNPJ": "11222333000182"})},
		{"G02a-10", "55", nfeM("entrega", map[string]any{"CPF": "00000000000"})},
		{"X05-10", "55", nfeM("transp/transporta", map[string]any{"CPF": "52998224726"})},
		{"YA05-20", "55", pag(map[string]any{"tPag": "03", "vPag": "20.00", "card": map[string]any{"CNPJ": "11222333000182"}})},
		{"YA03c-10", "55", pag(map[string]any{"tPag": "01", "vPag": "20.00", "CNPJPag": "11222333000182"})},
		{"E01-10", "55", nfeM("dest", nil)},
		{"E03a-10", "55", nfeM("ide/idDest", "3")},
		{"E03a-20", "55", nfeMMulti(nfeM("dest/idEstrangeiro", "AB123"), nfeM("ide/indFinal", "0"))},
		{"E03a-30", "55", nfeMMulti(nfeM("dest/idEstrangeiro", "AB123"), nfeM("dest/IE", "123456789012"))},
		{"E03a-60", "55", nfeMMulti(nfeM("dest/idEstrangeiro", "AB#123"))},
		{"E04-20", "55", nfeM("ide/tpAmb", "2")},
		{"E10-20", "55", nfeM("dest/enderDest/cMun", "4150308")},
		{"E10-30", "55", nfeMMulti(nfeM("dest/enderDest/UF", "EX"), nfeM("dest/enderDest/cMun", "1234567"))},
		{"F07-10", "55", nfeM("retirada", map[string]any{"UF": "EX", "cMun": "1234567"})},
		{"G07-10", "55", nfeM("entrega", map[string]any{"UF": "EX", "cMun": "1234567"})},
		{"F07-30", "55", nfeM("retirada", map[string]any{"UF": "SP", "cMun": "4150308"})},
		{"G07-30", "55", nfeM("entrega", map[string]any{"UF": "SP", "cMun": "4150308"})},
		{"E12-10", "55", nfeM("ide/idDest", "3")},
		{"E14-10", "55", nfeM("ide/idDest", "3")},
		{"E14-20", "55", nfeM("dest/enderDest/cPais", "0249")},
		{"E14-30", "55", nfeM("dest/enderDest/UF", "EX")},
		{"E12-30", "55", nfeMMulti(nfeM("ide/idDest", "2"), nfeM("dest", map[string]any{"CNPJ": nfeTCNPJ2, "indIEDest": "9", "enderDest": map[string]any{"UF": "SP"}}))},
		{"E12-50", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("ide/idDest", "2"), nfeM("dest", map[string]any{"CNPJ": nfeTCNPJ2, "indIEDest": "9", "enderDest": map[string]any{"UF": "SP"}}))},
		{"E12-40", "55", nfeMMulti(nfeM("ide/indFinal", "0"), nfeM("dest/enderDest/UF", "RJ"))},
		{"E12-60", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("ide/indFinal", "0"), nfeM("dest/enderDest/UF", "RJ"))},
		{"E16a-20", "55", nfeMMulti(nfeM("ide/idDest", "3"), nfeM("dest/indIEDest", "1"))},
		{"E16a-40", "55", nfeM("ide/indFinal", "0")},
		{"E17-20", "55", nfeM("dest/indIEDest", "1")},
		{"E17-30", "55", nfeMMulti(nfeM("dest/indIEDest", "2"), nfeM("dest/IE", "123456789012"))},
		{"E17-40", "55", nfeMMulti(nfeM("dest/enderDest/UF", "EX"), nfeM("dest/IE", "123"))},
		{"C17-30", "55", nfeM("emit/IE", "ISENTO")},
		{"C02-30", "55", nfeM("ide/serie", "950")},
		{"C02a-10", "55", nfeMMulti(nfeM("emit/CNPJ", nil), nfeM("emit/CPF", nfeTCPF), nfeM("ide/serie", "5"))},
		{"C18-40", "55", nfeM("emit/IEST", "123456789012")},
		{"C18-50", "55", nfeMMulti(nfeM("emit/IE", nil), nfeM("emit/IEST", "123"))},
		{"N12-20", "55", nfeMMulti(nfeM("emit/CRT", "1"), nfeMICMS(cst("10", nil)))},
		{"N12a-10", "55", nfeMICMS(sn("ICMSSN102", "102"))},
		{"N12a-80", "55", nfeMMulti(nfeM("emit/CRT", "4"), nfeMICMS(sn("ICMSSN500", "500")))},
		{"U01-10", "55", nfeM("det/imposto/ISSQN", map[string]any{})},
		// itens
		{"I08-10", "55", nfeM("det/prod/CFOP", "1102")},
		{"I08-20", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("det/prod/CFOP", "5102"))},
		{"I08-30", "55", nfeM("det/prod/CFOP", "7102")},
		{"I08-40", "55", nfeM("det/prod/CFOP", "6102")},
		{"I08-50", "55", nfeM("ide/idDest", "2")},
		{"I08-60", "55", nfeM("det/prod/CFOP", "7102")},
		{"I08-110", "55", nfeM("det/prod/CFOP", "3102")},
		{"I08-120", "55", nfeM("det/prod/CFOP", "3102")},
		{"I08-130", "55", nfeM("det/prod/CFOP", "3102")},
		{"I08-160", "55", nfeM("det/prod/CFOP", "5933")},
		{"I08-170", "55", nfeM("det/imposto/ISSQN", map[string]any{})},
		{"I08-184", "55", nfeM("det/prod/CFOP", "5929")},
		{"I08-190", "55", nfeM("det/prod/CFOP", "3503")},
		{"I08-186", "55", nfeMMulti(nfeM("det/prod/CFOP", "5929"), nfref(map[string]any{"refECF": map[string]any{"mod": "2D"}}))},
		{"I08-180", "55", nfeMMulti(nfeM("det/prod/CFOP", "5929"), nfeMRefs("refNFe", nfeTKey("35", "2610", nfeTCNPJ, "65", "001", "000000001", "1", "12345678")))},
		{"I11-10", "55", nfeM("det/prod/vProd", "25.00")},
		{"I11-20", "55", nfeM("det/prod/vUnTrib", "12.00")},
		{"I17-10", "55", nfeM("det/prod/vDesc", "30.00")},
		{"I03-10", "55", nfeM("det/prod/cEAN", "7891000100104")},
		{"I12-10", "55", nfeM("det/prod/cEANTrib", "7891000100104")},
		{"I12-30", "55", nfeM("det/prod/cEAN", "7891000100103")},
		{"I12-40", "55", nfeM("det/prod/cEANTrib", "7891000100103")},
		{"I05-24", "55", nfeM("det/prod/NCM", "00")},
		{"I05e-10", "55", nfeM("det/prod/indEscala", "N")},
		{"I05e-20", "55", nfeM("det/prod/CNPJFab", "11222333000182")},
		{"I84-10", "55", nfeM("det/prod/rastro", map[string]any{"dFab": "2026-01-02", "dVal": "2026-01-01"})},
		{"I86-10", "55", nfeM("det/prod/infProdNFF", map[string]any{})},
		{"I87-10", "55", nfeM("det/prod/infProdEmb", map[string]any{})},
		{"I50-10", "55", nfeM("det/prod/detExport", map[string]any{})},
		{"I23b-10", "55", nfeM("det/prod/DI", map[string]any{"tpViaTransp": "1"})},
		{"I23d-10", "55", nfeM("det/prod/DI", map[string]any{"tpIntermedio": "2", "UFTerceiro": "SP"})},
		{"I23e-10", "55", nfeM("det/prod/DI", map[string]any{"tpIntermedio": "2", "CNPJ": nfeTCNPJ})},
		{"I23d-20", "55", nfeM("det/prod/DI", map[string]any{"CNPJ": "11222333000182"})},
		{"I23d1-10", "55", nfeM("det/prod/DI", map[string]any{"CPF": "00000000000"})},
		{"Q01-20", "55", nfeM("det/imposto/PIS", nil)},
		{"S01-20", "55", nfeM("det/imposto/COFINS", nil)},
		{"O09-10", "55", nfeM("det/imposto/IPI", map[string]any{"cEnq": "150", "IPINT": map[string]any{"CST": "02"}})},
		{"LA03c-10", "55", nfeM("det/prod/comb", map[string]any{"cProdANP": "320101001", "pGLP": "10"})},
		{"LA03c-20", "55", nfeM("det/prod/comb", map[string]any{"cProdANP": "210203001", "pGLP": "60", "pGNn": "30", "vPart": "1.00"})},
		{"LA03d-10", "55", nfeM("det/prod/comb", map[string]any{"cProdANP": "210203001", "pGLP": "100"})},
		{"LA16-10", "65", nfeM("det/prod/comb", map[string]any{"encerrante": map[string]any{"vEncIni": "10", "vEncFin": "5"}})},
		{"LA11-20", "55", nfeM("det/prod/comb", map[string]any{"encerrante": map[string]any{"vEncIni": "1", "vEncFin": "5"}})},
		// ICMS
		{"N17-20", "55", nfeM("det/imposto/ICMS/ICMS00/vICMS", "9.99")},
		{"N16a-10", "55", nfeMICMS(cst("51", map[string]any{"vBC": "20.00", "pICMS": "18.00", "vICMSOp": "9.00"}))},
		{"N16c-10", "55", nfeMICMS(cst("51", map[string]any{"vICMSOp": "3.60", "pDif": "50.00", "vICMSDif": "3.00"}))},
		{"N17-10", "55", nfeMICMS(cst("51", map[string]any{"vICMSOp": "3.60", "vICMSDif": "1.80", "vICMS": "3.60"}))},
		{"N39-10", "55", nfeMICMS(cst("02", map[string]any{"qBCMono": "10", "adRemICMS": "1.00", "vICMSMono": "5.00"}))},
		{"N45-10", "55", nfeMICMS(cst("61", map[string]any{"qBCMonoRet": "10", "adRemICMSRet": "1.00", "vICMSMonoRet": "5.00"}))},
		{"N27d-10", "55", nfeMICMS(cst("60", map[string]any{"vBCFCPSTRet": "100", "pFCPSTRet": "2", "vFCPSTRet": "5.00"}))},
		{"N17c-10", "55", nfeMMulti(nfeM("det/imposto/ICMS/ICMS00/pFCP", "2.00"), nfeM("det/imposto/ICMS/ICMS00/vFCP", "5.00"))},
		{"N17b-10", "55", nfeM("det/imposto/ICMS/ICMS00/pFCP", "0")},
		{"N23b-10", "55", nfeMICMS(cst("10", map[string]any{"pFCPST": "0"}))},
		{"N27b-10", "55", nfeMICMS(cst("60", map[string]any{"pFCPSTRet": "0"}))},
		{"N28-30", "55", nfeMICMS(cst("40", map[string]any{"motDesICMS": "9"}))},
		{"N28-12", "55", nfeMICMS(cst("40", map[string]any{"motDesICMS": "7", "vICMSDeson": "1.00"}))},
		{"N28-10", "65", nfeMICMS(cst("40", map[string]any{"motDesICMS": "7", "vICMSDeson": "1.00", "indDeduzDeson": "1"}))},
		{"N18-10", "55", nfeMICMS(cst("10", map[string]any{"modBCST": "4"}))},
		{"N18-20", "55", nfeMICMS(cst("10", map[string]any{"modBCST": "3", "pMVAST": "10"}))},
		{"N37a-10", "55", nfeMICMS(cst("02", nil))},
		{"N39a-10", "55", nfeMICMS(cst("15", nil))},
		{"N43a-10", "55", nfeMICMS(cst("61", nil))},
		{"N23-10", "55", nfeMICMS(cst("60", nil))},
		{"NA13-10", "55", nfeM("det/imposto/ICMSUFDest", map[string]any{"vFCPUFDest": "9.00", "vBCFCPUFDest": "20.00", "pFCPUFDest": "2.00"})},
		{"NA01-30", "55", nfeM("det/imposto/ICMSUFDest", map[string]any{})},
		{"NA01-20", "55", nfeMMulti(nfeM("ide/idDest", "2"), nfeM("det/prod/CFOP", "6102"))},
		// totais
		{"W07-10", "55", tot("vProd", "25.00")},
		{"W08-10", "55", tot("vFrete", "5.00")},
		{"W09-10", "55", tot("vSeg", "5.00")},
		{"W10-10", "55", tot("vDesc", "5.00")},
		{"W11-10", "55", tot("vII", "5.00")},
		{"W12-10", "55", tot("vIPI", "5.00")},
		{"W12a-10", "55", tot("vIPIDevol", "5.00")},
		{"W15-10", "55", tot("vOutro", "5.00")},
		{"W03-10", "55", tot("vBC", "5.00")},
		{"W04-10", "55", tot("vICMS", "5.00")},
		{"W04a-10", "55", tot("vICMSDeson", "5.00")},
		{"W04b-10", "55", tot("vFCP", "5.00")},
		{"W05-10", "55", tot("vBCST", "5.00")},
		{"W06-10", "55", tot("vST", "5.00")},
		{"W06a-10", "55", tot("vFCPST", "5.00")},
		{"W06b-10", "55", tot("vFCPSTRet", "5.00")},
		{"W06c-10", "55", tot("vICMSMono", "5.00")},
		{"W06d-10", "55", tot("vICMSMonoReten", "5.00")},
		{"W06e-10", "55", tot("vICMSMonoRet", "5.00")},
		{"W04c-10", "55", tot("vFCPUFDest", "5.00")},
		{"W04e-10", "55", tot("vICMSUFDest", "5.00")},
		{"W04g-10", "55", tot("vICMSUFRemet", "5.00")},
		{"W13-10", "55", tot("vPIS", "5.00")},
		{"W14-10", "55", tot("vCOFINS", "5.00")},
		{"W18-10", "55", nfeMMulti(sq("vServ"), issqn)},
		{"W19-10", "55", sq("vBC")},
		{"W20-10", "55", sq("vISS")},
		{"W21-10", "55", sq("vPIS")},
		{"W22-10", "55", sq("vCOFINS")},
		{"W22b-10", "55", sq("vDeducao")},
		{"W22c-10", "55", sq("vOutro")},
		{"W22d-10", "55", sq("vDescIncond")},
		{"W22e-10", "55", sq("vDescCond")},
		{"W22f-10", "55", sq("vISSRet")},
		{"W33-10", "55", nfeM("total/ISTot/vIS", "5.00")},
		{"W31-10", "55", nfeM("total/ISTot/vIS", "0.00")},
		{"W31-20", "55", nfeM("det/imposto/IS", map[string]any{"vIS": "0.00"})},
		{"W34-10", "55", itot("vBCIBSCBS")},
		{"W34-20", "55", nfeM("det/imposto/IBSCBS", map[string]any{})},
		{"W35-10", "55", itot("vBCIBSCBS")},
		{"W38-10", "55", itot("gIBS/gIBSUF/vDif")},
		{"W39-10", "55", itot("gIBS/gIBSUF/vDevTrib")},
		{"W41-10", "55", itot("gIBS/gIBSUF/vIBSUF")},
		{"W43-10", "55", itot("gIBS/gIBSMun/vDif")},
		{"W44-10", "55", itot("gIBS/gIBSMun/vDevTrib")},
		{"W46-10", "55", itot("gIBS/gIBSMun/vIBSMun")},
		{"W47-10", "55", itot("gIBS/vIBS")},
		{"W48-10", "55", itot("gIBS/vCredPres")},
		{"W53-10", "55", itot("gCBS/vDif")},
		{"W54-10", "55", itot("gCBS/vDevTrib")},
		{"W56-10", "55", itot("gCBS/vCBS")},
		{"W56a-10", "55", itot("gCBS/vCredPres")},
		{"W58-10", "55", itot("gMono/vIBSMono")},
		{"W59-10", "55", itot("gMono/vCBSMono")},
		{"W59a-10", "55", itot("gMono/vIBSMonoReten")},
		{"W59b-10", "55", itot("gMono/vCBSMonoReten")},
		{"W59c-10", "55", itot("gMono/vIBSMonoRet")},
		{"W59d-10", "55", itot("gMono/vCBSMonoRet")},
		{"W59f-10", "55", itot("gEstornoCred/vIBSEstCred")},
		{"W59g-10", "55", itot("gEstornoCred/vCBSEstCred")},
		{"W16-10", "55", tot("vNF", "99.00")},
		// pagamento e cobrança
		{"YA03-10", "55", nfeM("pag/detPag", []any{map[string]any{"tPag": "01", "vPag": "10.00"}})},
		{"YA03-20", "55", nfeM("pag/detPag", []any{map[string]any{"tPag": "01", "vPag": "30.00"}})},
		{"YA09-10", "55", nfeMMulti(nfeM("pag/detPag", []any{map[string]any{"tPag": "01", "vPag": "30.00"}}), nfeM("pag/vTroco", "5.00"))},
		{"YA03-30", "55", pag(map[string]any{"tPag": "90", "vPag": "5.00"})},
		{"YA02-04", "55", nfeM("ide/finNFe", "3")},
		{"YA02-10", "65", pag(map[string]any{"tPag": "14", "vPag": "20.00"})},
		{"YA02-40", "65", pag(map[string]any{"tPag": "90", "vPag": "0.00"})},
		{"YA02a-10", "55", pag(map[string]any{"tPag": "99", "vPag": "20.00"})},
		{"YA02a-20", "55", pag(map[string]any{"tPag": "01", "vPag": "20.00", "xPag": "Outro"})},
		{"YA04-20", "55", pag(map[string]any{"tPag": "01", "vPag": "20.00", "card": map[string]any{"tpIntegra": "2"}})},
		{"YA05-10", "55", pag(map[string]any{"tPag": "03", "vPag": "20.00", "card": map[string]any{"tpIntegra": "1"}})},
		{"Y05-10", "55", nfeM("cobr/fat", map[string]any{"vOrig": "10.00", "vDesc": "15.00"})},
		{"Y06-10", "55", nfeM("cobr/fat", map[string]any{"vOrig": "10.00", "vDesc": "2.00", "vLiq": "9.00"})},
		{"Y08-10", "55", nfeM("cobr/dup", []any{map[string]any{"nDup": "002", "dVenc": "2026-12-05"}})},
		{"Y09-20", "55", nfeM("cobr/dup", []any{map[string]any{"nDup": "001", "dVenc": "2026-10-05"}})},
		{"Y09-30", "55", nfeM("cobr/dup", []any{map[string]any{"nDup": "001", "dVenc": "2026-12-05"}, map[string]any{"nDup": "002", "dVenc": "2026-11-20"}})},
		{"Y09-40", "55", nfeM("cobr/dup", []any{map[string]any{"nDup": "001", "dVenc": "2026-11-05"}})},
		{"Y10-10", "55", nfeMMulti(nfeM("cobr/fat", map[string]any{"vLiq": "20.00"}), nfeM("cobr/dup", []any{map[string]any{"nDup": "001", "dVenc": "2026-12-05", "vDup": "10.00"}}))},
		// chaves referenciadas
		{"BA02-10", "55", nfeMRefs("refNFe", bad)},
		{"BA02-14", "55", nfeMRefs("refNFe", nfeTKey("99", "2610", nfeTCNPJ, "55", "001", "000000001", "1", "12345678"))},
		{"BA02-24", "55", nfeMRefs("refNFe", nfeTKey("35", "2613", nfeTCNPJ, "55", "001", "000000001", "1", "12345678"))},
		{"BA02-30", "55", nfeMRefs("refNFe", nfeTKey("35", "2610", "11222333000182", "55", "001", "000000001", "1", "12345678"))},
		{"BA02-34", "55", nfeMRefs("refNFe", nfeTKey("35", "2610", nfeTCNPJ, "57", "001", "000000001", "1", "12345678"))},
		{"BA02-40", "55", nfeMRefs("refNFe", nfeTKey("35", "2610", nfeTCNPJ, "55", "001", "000000000", "1", "12345678"))},
		{"BA02-44", "55", nfeMRefs("refNFe", k55, k55)},
		{"BA02-50", "55", nfeMRefs("refNFe", own)},
		{"BA02-60", "55", nfeMRefs("refNFe", nfeTKey("35", "2610", nfeTCNPJ, "55", "001", "000000001", "8", "12345678"))},
		{"BA19-10", "55", nfeMRefs("refCTe", k57[:43]+string('0'+(k57[43]-'0'+1)%10))},
		{"BA19-14", "55", nfeMRefs("refCTe", nfeTKey("99", "2610", nfeTCNPJ, "57", "001", "000000001", "1", "12345678"))},
		{"BA19-24", "55", nfeMRefs("refCTe", nfeTKey("35", "2613", nfeTCNPJ, "57", "001", "000000001", "1", "12345678"))},
		{"BA19-30", "55", nfeMRefs("refCTe", nfeTKey("35", "2610", "11222333000182", "57", "001", "000000001", "1", "12345678"))},
		{"BA19-34", "55", nfeMRefs("refCTe", k55)},
		{"BA19-40", "55", nfeMRefs("refCTe", nfeTKey("35", "2610", nfeTCNPJ, "57", "001", "000000000", "1", "12345678"))},
		{"BA19-44", "55", nfeMRefs("refCTe", k57, k57)},
		{"BB05-70", "55", nfeM("ide/gCompraGov/refDFeAnt", bad)},
		{"BB05-80", "55", nfeM("ide/gCompraGov/refDFeAnt", nfeTKey("99", "2610", nfeTCNPJ, "55", "001", "000000001", "1", "12345678"))},
		{"BB05-100", "55", nfeM("ide/gCompraGov/refDFeAnt", nfeTKey("35", "2613", nfeTCNPJ, "55", "001", "000000001", "1", "12345678"))},
		{"BB05-110", "55", nfeM("ide/gCompraGov/refDFeAnt", nfeTKey("35", "2610", "11222333000182", "55", "001", "000000001", "1", "12345678"))},
		{"BB05-120", "55", nfeM("ide/gCompraGov/refDFeAnt", nfeTKey("35", "2610", nfeTCNPJ, "57", "001", "000000001", "1", "12345678"))},
		{"BB05-130", "55", nfeM("ide/gCompraGov/refDFeAnt", nfeTKey("35", "2610", nfeTCNPJ, "55", "001", "000000000", "1", "12345678"))},
		{"BB05-140", "55", nfeM("ide/gCompraGov/refDFeAnt", []any{k55, k55})},
		{"BB05-150", "55", nfeM("ide/gCompraGov/refDFeAnt", own)},
		{"BB05-160", "55", nfeM("ide/gCompraGov/refDFeAnt", nfeTKey("35", "2610", nfeTCNPJ, "55", "001", "000000001", "8", "12345678"))},
		// demais
		{"X04-30", "55", nfeMMulti(nfeM("transp/modFrete", "3"), nfeM("transp/transporta", map[string]any{"CNPJ": nfeTCNPJ2}))},
		{"X04-40", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("transp/modFrete", "3"), nfeM("transp/transporta", map[string]any{"CNPJ": nfeTCNPJ2}))},
		{"X04-50", "55", nfeMMulti(nfeM("transp/modFrete", "0"), nfeM("transp/transporta", map[string]any{"CNPJ": "11222333000262"}))},
		{"X04-60", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("transp/modFrete", "0"), nfeM("transp/transporta", map[string]any{"CPF": nfeTCPF}))},
		{"X04-70", "55", nfeMMulti(nfeM("transp/modFrete", "4"), nfeM("transp/transporta", map[string]any{"CNPJ": nfeTCNPJ2}))},
		{"X04-80", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("transp/modFrete", "4"), nfeM("transp/transporta", map[string]any{"CNPJ": nfeTCNPJ2}))},
		{"X04-90", "55", nfeMMulti(nfeM("transp/modFrete", "0"), nfeM("transp/transporta", map[string]any{"CPF": nfeTCPF}))},
		{"X04-100", "55", nfeMMulti(nfeM("ide/tpNF", "0"), nfeM("transp/modFrete", "0"), nfeM("transp/transporta", map[string]any{"CNPJ": "11222333000262"}))},
		{"X03-30", "55", nfeM("transp/transporta", map[string]any{"CNPJ": nfeTCNPJ2})},
		{"X02-20", "55", nfeMMulti(nfeM("ide/idDest", "2"), nfeM("transp/veicTransp", map[string]any{"placa": "AAA1B23"}))},
		{"BA10-10", "55", nfref(map[string]any{"refNFP": map[string]any{}})},
		{"BA06-10", "55", nfref(map[string]any{"refNF": map[string]any{"CNPJ": "11222333000182"}})},
		{"BA13-10", "55", nfref(map[string]any{"refNFP": map[string]any{"CNPJ": "11222333000182"}})},
		{"BA14-10", "55", nfref(map[string]any{"refNFP": map[string]any{"CPF": "00000000000"}})},
		{"BA20-10", "55", nfeM("ide/NFref", []any{map[string]any{"refECF": map[string]any{"mod": "2D", "nECF": "1", "nCOO": "1"}}, map[string]any{"refECF": map[string]any{"mod": "2D", "nECF": "1", "nCOO": "1"}}})},
		{"BA20-20", "55", nfeMMulti(nfeM("ide/idDest", "2"), nfref(map[string]any{"refECF": map[string]any{"mod": "2D"}}))},
		{"ZX02-10", "65", func(doc, inf map[string]any) { delete(doc, "infNFeSupl") }},
		{"ZX02-220", "55", qr("https://x.invalid/q?p=" + own + "|2|1")},
		{"ZX02-222", "65", nfeMMulti(nfeM("emit/CNPJ", nil), nfeM("emit/CPF", nfeTCPF))},
		{"ZX02-228", "65", qr("https://x.invalid/q?p=" + k55 + "|2|1")},
		{"ZX02-236", "65", qr("https://x.invalid/q?p=" + own + "|9|1")},
		{"ZX02-244", "65", qr("https://x.invalid/q?p=" + own + "|2|2")},
	}
}

func TestNFeRulesFailOnBadDoc(t *testing.T) {
	cases := nfeCases()
	byID := map[string]bool{}
	for _, c := range cases {
		byID[c.id+"/"+c.mod] = true
		var rule *Rule
		for i := range nfeRules {
			if nfeRules[i].ID == c.id {
				rule = &nfeRules[i]
			}
		}
		if rule == nil {
			t.Errorf("caso %s sem regra", c.id)
			continue
		}
		doc, inf := nfeTBase(c.mod)
		if got := rule.Check(doc); len(got) != 0 {
			t.Errorf("%s: dispara no documento válido: %v", c.id, got)
		}
		c.mut(doc, inf)
		if got := rule.Check(doc); len(got) == 0 {
			t.Errorf("%s: não disparou no documento inválido", c.id)
		}
	}
	for _, r := range nfeRules {
		found := false
		for _, c := range cases {
			if c.id == r.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("regra %s sem teste", r.ID)
		}
	}
}

// Regras só do modelo 65 não podem disparar na NF-e e vice-versa.
func TestNFeModelGuard(t *testing.T) {
	doc, inf := nfeTBase("55")
	nfeTSet(inf, "ide/tpImp", "1")
	for _, c := range nfeCases() {
		if c.mod != "65" {
			continue
		}
		d, i := nfeTBase("55")
		c.mut(d, i)
		for _, r := range nfeRules {
			if r.ID == c.id && r.Mode != "" && nfeOnly65(r.ID) {
				if got := r.Check(d); len(got) != 0 {
					t.Errorf("%s dispara em NF-e modelo 55: %v", r.ID, got)
				}
			}
		}
	}
	_ = doc
}

func nfeOnly65(id string) bool {
	return nfeIn(id, "B21-10", "B22-34", "B26-50", "B10a-10", "B25d-10", "B25b-60", "B25b-70", "C17-42", "E16a-10", "E17-10", "E18-10", "E02-20", "E05-20", "X03-10", "X03-20", "ZA01-30", "BB01-10", "C02a-04", "W16-40", "I08-150", "N12-40", "N12-44", "N12a-40", "N12a-44", "N12a-81", "YA02-10", "YA02-40", "ZX02-10", "ZX02-222")
}

func TestNFeApplyBlocks(t *testing.T) {
	doc, inf := nfeTBase("55")
	nfeTSet(inf, "emit/CNPJ", "11222333000182")
	found, blocking := Apply("nfe", doc, nil)
	if !blocking {
		t.Fatalf("CNPJ com DV inválido deveria bloquear: %v", found)
	}
}

func TestNFeDocumentChecks(t *testing.T) {
	if !nfeCNPJ(nfeTCNPJ) || nfeCNPJ("11222333000182") || nfeCNPJ("00000000000000") {
		t.Error("nfeCNPJ")
	}
	if !nfeCPF(nfeTCPF) || nfeCPF("52998224726") || nfeCPF("11111111111") {
		t.Error("nfeCPF")
	}
	if !nfeGTIN("7891000100103") || nfeGTIN("7891000100104") {
		t.Error("nfeGTIN")
	}
	k := nfeTKey("35", "2611", nfeTCNPJ, "55", "001", "000000123", "1", "87654321")
	if !nfeKeyOK(k) || nfeKeyOK(k[:43]+"9") && nfeKeyOK(k[:43]+"8") {
		t.Error("nfeKeyOK")
	}
}

// B25-80: as exceções liberam os grupos de imposto na nota de crédito/débito.
func TestNFeB2580Exceptions(t *testing.T) {
	var rule Rule
	for _, r := range nfeRules {
		if r.ID == "B25-80" {
			rule = r
		}
	}
	cases := []struct {
		name, credito, debito string
		fires                 bool
	}{
		{"crédito 01 com ICMS", "01", "", true},
		{"crédito 03 libera todos", "03", "", false},
		{"crédito 04 libera todos", "04", "", false},
		{"débito 07 libera todos", "", "07", false},
		{"débito 06 não libera ICMS", "", "06", true},
	}
	for _, c := range cases {
		doc, inf := nfeTBase("55")
		fin := "5"
		if c.debito != "" {
			fin = "6"
			nfeTSet(inf, "ide/tpNFDebito", c.debito)
		} else {
			nfeTSet(inf, "ide/tpNFCredito", c.credito)
		}
		nfeTSet(inf, "ide/finNFe", fin)
		if got := len(rule.Check(doc)) > 0; got != c.fires {
			t.Errorf("%s: disparou=%v, esperado %v", c.name, got, c.fires)
		}
	}
	// débito 06 em 2026: PIS e COFINS são aceitos; sem ICMS a regra não dispara.
	doc, inf := nfeTBase("55")
	nfeTSet(inf, "ide/finNFe", "6")
	nfeTSet(inf, "ide/tpNFDebito", "06")
	nfeTSet(inf, "det/imposto/ICMS", nil)
	if got := rule.Check(doc); len(got) != 0 {
		t.Errorf("débito 06 com PIS/COFINS em 2026 não deveria disparar: %v", got)
	}
}

// E16a-30: as exceções de ST e de operação isenta liberam o destinatário isento.
func TestNFeE16a30Exceptions(t *testing.T) {
	var rule Rule
	for _, r := range nfeRules {
		if r.ID == "E16a-30" {
			rule = r
		}
	}
	doc, inf := nfeTBase("55")
	nfeTSet(inf, "dest/indIEDest", "2")
	nfeTSet(inf, "det/imposto/ICMS", map[string]any{"ICMS40": map[string]any{"orig": "0", "CST": "40"}})
	if got := rule.Check(doc); len(got) != 0 {
		t.Errorf("operação isenta não deveria disparar: %v", got)
	}
	nfeTSet(inf, "dest/enderDest/UF", "PR")
	nfeTSet(inf, "det/imposto/ICMS", map[string]any{"ICMS00": map[string]any{"orig": "0", "CST": "00"}})
	if got := rule.Check(doc); len(got) != 0 {
		t.Errorf("UF que admite isento não deveria disparar: %v", got)
	}
}
