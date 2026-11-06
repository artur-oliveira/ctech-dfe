package validate

import (
	"fmt"
	"strings"
	"testing"
)

const (
	cteTCNPJ  = "11222333000181"
	cteTCNPJ2 = "11444777000161"
	cteTCPF   = "52998224725"
)

// cteTK monta uma chave de acesso válida (DV calculado).
func cteTK(uf, yymm, id, mod, serie, nnf, tp, cnf string) string {
	b := uf + yymm + id + mod + cteLeft(serie, 3) + cteLeft(nnf, 9) + tp + cnf
	return b + fmt.Sprint(cteDV(b))
}

func cteBadKey(k string) string {
	last := k[43]
	if last == '9' {
		return k[:43] + "0"
	}
	return k[:43] + string(last+1)
}

// cteSet grava val no caminho "a.b.c" criando os mapas intermediários; val nil remove a chave.
func cteSet(m map[string]any, path string, val any) {
	parts := strings.Split(path, ".")
	cur := m
	for _, p := range parts[:len(parts)-1] {
		next, _ := cur[p].(map[string]any)
		if next == nil {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	last := parts[len(parts)-1]
	if val == nil {
		delete(cur, last)
		return
	}
	cur[last] = val
}

func cteSeal(inf map[string]any) {
	body := cteKeyBody(inf)
	dv := fmt.Sprint(cteDV(body))
	cteSet(inf, "ide.cDV", dv)
	inf["@Id"] = "CTe" + body + dv
}

func cteMaps(n int, mk func(i int) map[string]any) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = mk(i)
	}
	return out
}

// cteBase é um CT-e normal rodoviário SP para RJ que não dispara nenhuma regra.
func cteBase() map[string]any {
	k1 := cteTK("35", "2503", cteTCNPJ2, "55", "1", "100", "1", "12345678")
	k2 := cteTK("35", "2503", cteTCNPJ2, "55", "1", "101", "1", "12345678")
	inf := map[string]any{
		"ide": map[string]any{
			"cUF": "35", "cCT": "12345678", "CFOP": "6353", "natOp": "Transporte", "mod": "57", "serie": "1", "nCT": "1000",
			"dhEmi": "2025-03-10T10:00:00-03:00", "tpImp": "1", "tpEmis": "1", "tpAmb": "1", "tpCTe": "0", "procEmi": "0",
			"verProc": "1", "cMunEnv": "3550308", "xMunEnv": "SAO PAULO", "UFEnv": "SP", "modal": "01", "tpServ": "0",
			"cMunIni": "3550308", "UFIni": "SP", "cMunFim": "3304557", "UFFim": "RJ", "retira": "1", "indIEToma": "1",
			"toma3": map[string]any{"toma": "0"},
		},
		"emit": map[string]any{"CNPJ": cteTCNPJ, "IE": "123456789012", "CRT": "3",
			"enderEmit": map[string]any{"cMun": "3550308", "UF": "SP"}},
		"rem": map[string]any{"CNPJ": cteTCNPJ2, "IE": "111111111", "xNome": "REMETENTE",
			"enderReme": map[string]any{"cMun": "3550308", "UF": "SP"}},
		"dest": map[string]any{"CPF": cteTCPF, "xNome": "DESTINATARIO",
			"enderDest": map[string]any{"cMun": "3304557", "UF": "RJ"}},
		"vPrest": map[string]any{"vTPrest": "100.00", "vRec": "100.00", "Comp": []any{map[string]any{"vComp": "100.00"}}},
		"imp": map[string]any{
			"ICMS": map[string]any{"ICMS00": map[string]any{"CST": "00", "vBC": "100.00", "pICMS": "12.00", "vICMS": "12.00"}},
			"IBSCBS": map[string]any{"CST": "000", "cClassTrib": "000001", "gIBSCBS": map[string]any{
				"vBC":     "100.00",
				"gIBSUF":  map[string]any{"pIBSUF": "0.10", "vIBSUF": "0.10"},
				"gIBSMun": map[string]any{"pIBSMun": "0", "vIBSMun": "0.00"},
				"vIBS":    "0.10",
				"gCBS":    map[string]any{"pCBS": "0.90", "vCBS": "0.90"},
			}},
			"vTotDFe": "100.00",
		},
		"infCTeNorm": map[string]any{
			"infCarga": map[string]any{"vCarga": "1000.00", "proPred": "X", "infQ": []any{map[string]any{"cUnid": "01", "tpMed": "KG", "qCarga": "1.0000"}}},
			"infDoc":   map[string]any{"infNFe": []any{map[string]any{"chave": k1}, map[string]any{"chave": k2}}},
			"infModal": map[string]any{"rodo": map[string]any{"RNTRC": "12345678"}},
		},
	}
	cteSeal(inf)
	return inf
}

func cteRun(id string, inf map[string]any) []string {
	for _, r := range Rules("cte") {
		if r.ID == id {
			return r.Check(inf)
		}
	}
	panic("regra inexistente: " + id)
}

func cteTestKey57(nnf string) string {
	return cteTK("35", "2503", cteTCNPJ, "57", "1", nnf, "1", "12345678")
}

// cteFails: uma mutação por regra; a regra deve disparar no documento mutado e ficar muda no base.
func cteFails() map[string]func(inf map[string]any) {
	k57 := cteTestKey57("500")
	other := cteTK("35", "2503", cteTCNPJ, "55", "1", "102", "1", "12345678")
	nfes := func(keys ...string) []any {
		return cteMaps(len(keys), func(i int) map[string]any { return map[string]any{"chave": keys[i]} })
	}
	set := func(path string, v any) func(map[string]any) {
		return func(inf map[string]any) { cteSet(inf, path, v) }
	}
	multi := func(fs ...func(map[string]any)) func(map[string]any) {
		return func(inf map[string]any) {
			for _, f := range fs {
				f(inf)
			}
		}
	}
	glob := set("ide.indGlobalizado", "1")
	m := map[string]func(map[string]any){
		"G002":  set("ide.tpAmb", "2"),
		"G003":  multi(set("ide.tpAmb", "2"), set("exped", map[string]any{"xNome": "X"})),
		"G004":  multi(set("ide.tpAmb", "2"), set("receb", map[string]any{"xNome": "X"})),
		"G005":  multi(set("ide.tpAmb", "2"), set("rem.xNome", cteHomolXNome)),
		"G006":  set("ide.serie", "890"),
		"G007":  set("emit.enderEmit.UF", "RJ"),
		"G008":  set("ide.dhCont", "2025-03-10T09:00:00-03:00"),
		"G008b": set("ide.xJust", "justificativa de contingencia"),
		"G009":  set("ide.tpEmis", "2"),
		"G010":  multi(set("ide.tpEmis", "2"), set("ide.dhCont", "2025-03-10T11:00:00-03:00")),
		"G012":  multi(set("ide.tpEmis", "7"), set("ide.tpCTe", "1")),
		"G013":  set("ide.mod", "58"),
		"G014":  set("ide.nCT", "1001"),
		"G015": func(inf map[string]any) {
			cteSet(inf, "ide.cDV", fmt.Sprint((cteDV(cteKeyBody(inf))+1)%10))
		},
		"G016":  set("infCTeNorm", nil),
		"G017":  set("ide.tpCTe", "1"),
		"G018":  set("rem", nil),
		"G019":  set("ide.toma3.toma", "1"),
		"G020":  set("ide.toma3.toma", "2"),
		"G021":  multi(set("ide.toma3.toma", "3"), set("dest", nil)),
		"G022":  set("rem.IE", nil),
		"G023":  set("ide.indIEToma", "2"),
		"G024":  multi(set("ide.indIEToma", "2"), set("rem.IE", "ISENTO")),
		"G027":  set("infCTeNorm.infCarga.vCarga", nil),
		"G036":  set("infCTeNorm.infDoc", nil),
		"G037":  set("ide.tpServ", "3"),
		"G039":  set("infCTeNorm.infDoc.infNFe", nfes(cteTK("35", "2503", cteTCNPJ2, "55", "1", "100", "1", "12345678"), cteTK("35", "2503", cteTCNPJ2, "55", "1", "100", "1", "12345678"))),
		"G040":  set("infCTeNorm.infDoc.infNFe", nfes(cteBadKey(other))),
		"G045a": set("infCTeNorm.infDoc.infNFe", cteMaps(2001, func(i int) map[string]any { return map[string]any{"chave": "x"} })),
		"G045b": set("infCTeNorm.infDoc.infDCe", nfes("a", "a")),
		"G045c": set("infCTeNorm.infDoc.infDCe", nfes("123")),
		"G046":  set("vPrest.vTPrest", "10000000.00"),
		"G047":  set("imp.ICMS.ICMS00.vICMS", "13.00"),
		"G048":  set("vPrest.vRec", "200.00"),
		"G050":  set("ide.CFOP", "5353"),
		"G051":  set("emit.enderEmit.UF", "RJ"),
		"G052":  set("ide.CFOP", "6932"),
		"G053":  multi(set("rem", nil)),
		"G054":  set("dest", nil),
		"G055":  set("ide.tpServ", "3"),
		"G056":  set("ide.tpServ", "3"),
		"G057":  set("ide.tpServ", "4"),
		"G058":  set("infCTeNorm.infServVinc", map[string]any{"infCTeMultimodal": nfes()}),
		"G059":  set("ide.tpServ", "1"),
		"G060":  set("infCTeNorm.docAnt", map[string]any{}),
		"G061":  set("infCTeNorm.docAnt", cteDocAnt(k57, k57)),
		"G065":  set("infCTeNorm.docAnt", cteDocAnt(cteBadKey(k57))),
		"G070":  set("infCTeNorm.infServVinc", cteMultimodal(k57, k57)),
		"G071":  set("infCTeNorm.infServVinc", cteMultimodal(cteBadKey(k57))),
		"G076":  multi(set("ide.tpCTe", "3"), set("ide.tpEmis", "2")),
		"G077":  set("ide.tpCTe", "3"),
		"G078":  multi(set("ide.tpCTe", "3"), set("infCTeNorm.infCteSub", map[string]any{"chCte": cteBadKey(k57)})),
		"G104":  set("emit.CNPJ", "11222333000182"),
		"G105":  set("emit.IE", ""),
		"G112":  set("emit.enderEmit.cMun", "3304557"),
		"G161":  multi(set("ide.tpCTe", "1"), set("infCteComp", cteMaps(1, func(int) map[string]any { return map[string]any{"chCTe": cteBadKey(k57)} }))),
		"G177":  set("infCteComp", cteMaps(2, func(int) map[string]any { return map[string]any{"chCTe": k57} })),
		"G178":  set("ide.cMunEnv", "3304557"),
		"G180":  set("ide.cMunIni", "3304557"),
		"G182":  set("ide.cMunFim", "3550308"),
		"G193":  set("autXML", cteMaps(1, func(int) map[string]any { return map[string]any{"CNPJ": "11222333000182"} })),
		"G194":  set("autXML", cteMaps(1, func(int) map[string]any { return map[string]any{"CPF": "11111111111"} })),
		"G195":  set("autXML", cteMaps(2, func(int) map[string]any { return map[string]any{"CNPJ": "99999999000191"} })),
		"G196":  set("autXML", cteMaps(1, func(int) map[string]any { return map[string]any{"CNPJ": cteTCNPJ2} })),
		"G197":  glob,
		"G198":  multi(glob, set("ide.toma3.toma", "1")),
		"G199":  multi(glob, set("infCTeNorm.infDoc.infNFe", nfes())),
		"G200":  multi(glob, set("ide.toma3.toma", "3")),
		"G201":  multi(glob, set("infCTeNorm.infDoc.infNFe", nfes(cteTK("35", "2503", cteTCNPJ2, "55", "1", "100", "1", "12345678"), other))),
		"G202":  multi(glob, set("ide.toma3.toma", "3")),
		"G203":  glob,
		"G204":  multi(glob, set("ide.toma3.toma", "3")),
		"G205":  glob,
		"G206":  glob,
		"G207":  set("infCTeNorm.infDoc.infNFe", nfes(cteTK("35", "2503", cteTCNPJ2, "55", "1", "100", "1", "12345678"), other)),
		"G208":  set("rem.xNome", "DIVERSOS"),
		"G210":  set("infRespTec", map[string]any{"CNPJ": "1"}),
		"G218":  set("infSolicNFF", map[string]any{"xSolic": "x"}),

		"PAA-910": set("infPAA", map[string]any{}),
		"PAA-911": set("ide.procEmi", "4"),
		"PAA-912": multi(set("infPAA", map[string]any{}), set("ide.procEmi", "4")),
		"PAA-913": set("infPAA", map[string]any{}),
		"PAA01":   set("infPAA", map[string]any{"CNPJPAA": "123"}),

		"CGOV-001": multi(set("ide.gCompraGov", map[string]any{"tpEnteGov": "1"}), set("imp.IBSCBS.gIBSCBS.gIBSUF.pIBSUF", "1")),
		"CGOV-002": set("ide.gCompraGov", map[string]any{"tpOperGov": "1", "refDFeAnt": []any{k57}}),
		"CGOV-003": set("ide.gCompraGov", map[string]any{"tpOperGov": "2"}),
		"CGOV-004": set("ide.gCompraGov", map[string]any{"tpOperGov": "3"}),
		"CGOV-005": set("ide.gCompraGov", map[string]any{"tpOperGov": "4", "refDFeAnt": []any{k57}}),
		"CGOV-006": set("ide.gCompraGov", map[string]any{"tpOperGov": "2", "refDFeAnt": []any{"123"}}),

		"IBSCBS-049": set("ide.gCompraGov", map[string]any{"tpEnteGov": "5", "tpOperGov": "4"}),
		"IBSCBS-050": set("imp.IBSCBS.gIBSCBS.gTribCompraGov", map[string]any{}),
		"IBSCBS-051": multi(set("ide.gCompraGov", map[string]any{"tpEnteGov": "5", "tpOperGov": "4"}),
			set("imp.IBSCBS.gIBSCBS.gTribCompraGov", map[string]any{"vTribIBSUF": "5", "vTribIBSMun": "0", "vTribCBS": "0"})),
		"IBSCBS-022b": set("imp.IBSCBS.gIBSCBS.vIBS", "1.00"),
		"IBSCBS-360":  set("imp.vTotDFe", nil),
		"IBSCBS-365":  set("imp.vTotDFe", "50.00"),
		"IBSCBS-1021": set("imp.IBSCBS.gIBSCBS.gCBS.gALCZFMCBS", map[string]any{}),

		"IBSCBS-009b": set("imp.IBSCBS.gIBSCBS.gIBSUF.gRed", map[string]any{"pRedAliq": "40", "pAliqEfet": "0.10"}),
		"IBSCBS-017b": multi(set("imp.IBSCBS.gIBSCBS.gIBSMun.pIBSMun", "10"), set("imp.IBSCBS.gIBSCBS.gIBSMun.gRed", map[string]any{"pRedAliq": "40", "pAliqEfet": "1"})),
		"IBSCBS-024b": set("imp.IBSCBS.gIBSCBS.gCBS.gRed", map[string]any{"pRedAliq": "40", "pAliqEfet": "0.90"}),
		"IBSCBS-013":  set("imp.IBSCBS.gIBSCBS.gIBSUF.gDif", map[string]any{"pDif": "50", "vDif": "0.50"}),
		"IBSCBS-021":  multi(set("imp.IBSCBS.gIBSCBS.gIBSMun.pIBSMun", "10"), set("imp.IBSCBS.gIBSCBS.gIBSMun.gDif", map[string]any{"pDif": "50", "vDif": "1.00"})),
		"IBSCBS-028":  set("imp.IBSCBS.gIBSCBS.gCBS.gDif", map[string]any{"pDif": "50", "vDif": "0.10"}),
		"IBSCBS-014":  set("imp.IBSCBS.gIBSCBS.gIBSUF.vIBSUF", "0.50"),
		"IBSCBS-022":  set("imp.IBSCBS.gIBSCBS.gIBSMun.vIBSMun", "0.50"),
		"IBSCBS-029":  set("imp.IBSCBS.gIBSCBS.gCBS.vCBS", "5.00"),
		"IBSCBS-013a": set("imp.IBSCBS.gIBSCBS.gIBSUF.gDif", map[string]any{"pDif": "100", "vDif": "0.50"}),
		"IBSCBS-022a": set("imp.IBSCBS.gIBSCBS.gIBSMun.gDif", map[string]any{"pDif": "100", "vDif": "1.00"}),
		"IBSCBS-028a": set("imp.IBSCBS.gIBSCBS.gCBS.gDif", map[string]any{"pDif": "100", "vDif": "5.00"}),
		"IBSCBS-033":  set("imp.IBSCBS.gIBSCBS.gTribRegular", map[string]any{"pAliqEfetRegIBSUF": "10", "vTribRegIBSUF": "1"}),
		"IBSCBS-034":  set("imp.IBSCBS.gIBSCBS.gTribRegular", map[string]any{"pAliqEfetRegIBSMun": "10", "vTribRegIBSMun": "1"}),
		"IBSCBS-035":  set("imp.IBSCBS.gIBSCBS.gTribRegular", map[string]any{"pAliqEfetRegCBS": "10", "vTribRegCBS": "1"}),

		"SN-1090": multi(set("emit.CRT", "1"), set("emit.tpRegimeIBSCBS", "2")),
		"SN-1091": multi(set("emit.CRT", "3"), set("emit.tpRegimeIBSCBS", "1")),
		"SN-1092": multi(set("emit.CRT", "4"), set("emit.tpRegimeIBSCBS", "1")),
		"SN-1100": set("emit.CRT", "1"),
		"SN-1101": multi(set("emit.CRT", "1"), set("imp.IBSCBS.gIBSCBS", nil)),
		"SN-1102": multi(set("emit.CRT", "1"), set("emit.tpRegimeIBSCBS", "1"), set("imp.IBSCBS.gTribSN", map[string]any{})),
		"SN-1103": set("emit.CRT", "2"),
		"SN-1104": multi(set("emit.CRT", "2"), set("imp.IBSCBS.gTribSN", map[string]any{"gIBSSN": map[string]any{}, "gCBSSN": map[string]any{}})),
		"SN-1107": set("emit.CRT", "4"),

		"NT2026.004-1055": multi(set("imp.IBSCBS.indDoacao", "1"), set("rem.CNPJ", "11222333000199")),
		"NT2026.004-1072": set("ide.gMultaJuros", map[string]any{"chCTeRef": k57}),
		"NT2026.004-1051": set("imp.ICMS.gICMSPrevistoPagtoAntecip", map[string]any{"vICMSPrevisto": "1"}),
		"NT2026.004-1052": multi(set("imp.ICMS.gICMSPrevistoPagtoAntecip", map[string]any{"vICMSPrevisto": "1"}), set("ide.tpPagAnt", "3")),
		"ANT-1026":        set("ide.gPagAntecipado", map[string]any{"chDFePagAnt": k57}),
		"ANT-1027":        set("ide.tpPagAnt", "3"),
	}
	for _, a := range []struct{ tag, cnpj, cpf, mun, path, ender string }{
		{"rem", "G114", "G115", "G116", "rem", "enderReme"},
		{"dest", "G122", "G123", "G124", "dest", "enderDest"},
		{"exped", "G132", "G133", "G134", "exped", "enderExped"},
		{"receb", "G140", "G141", "G142", "receb", "enderReceb"},
		{"toma4", "G148", "G149", "G150", "ide.toma4", "enderToma"},
	} {
		m[a.cnpj] = set(a.path+".CNPJ", "11222333000182")
		m[a.cpf] = set(a.path+".CPF", "11111111111")
		m[a.mun] = set(a.path+"."+a.ender, map[string]any{"cMun": "3304557", "UF": "SP"})
	}
	return m
}

func cteDocAnt(keys ...string) map[string]any {
	els := cteMaps(len(keys), func(i int) map[string]any { return map[string]any{"chCTe": keys[i]} })
	return map[string]any{"emiDocAnt": []any{map[string]any{"idDocAnt": []any{map[string]any{"idDocAntEle": els}}}}}
}

func cteMultimodal(keys ...string) map[string]any {
	return map[string]any{"infCTeMultimodal": cteMaps(len(keys), func(i int) map[string]any { return map[string]any{"chCTeMultimodal": keys[i]} })}
}

func TestCTeBaselineClean(t *testing.T) {
	inf := cteBase()
	for _, r := range Rules("cte") {
		if got := r.Check(inf); len(got) > 0 {
			t.Errorf("%s dispara no documento válido: %v", r.ID, got)
		}
	}
}

func TestCTeRulesFail(t *testing.T) {
	cases := cteFails()
	for _, r := range Rules("cte") {
		mut, ok := cases[r.ID]
		if !ok {
			t.Errorf("regra %s sem caso de falha", r.ID)
			continue
		}
		inf := cteBase()
		mut(inf)
		if got := r.Check(inf); len(got) == 0 {
			t.Errorf("%s não disparou no documento inválido", r.ID)
		}
	}
	ids := map[string]bool{}
	for _, r := range Rules("cte") {
		if ids[r.ID] {
			t.Errorf("id duplicado %s", r.ID)
		}
		ids[r.ID] = true
		if r.CStat == 0 || r.Doc != "cte" || r.Mode == "" {
			t.Errorf("regra %s mal formada", r.ID)
		}
	}
	for id := range cases {
		if !ids[id] {
			t.Errorf("caso de falha para regra inexistente %s", id)
		}
	}
	if len(ids) < 50 {
		t.Errorf("catálogo com %d regras, esperado ao menos 50", len(ids))
	}
}

func TestCTeBlockRulesAreDeterministic(t *testing.T) {
	want := map[string]bool{"G015": true, "G104": true, "G193": true, "G194": true, "G210": true,
		"PAA01": true}
	for _, a := range []string{"G114", "G115", "G122", "G123", "G132", "G133", "G140", "G141", "G148", "G149"} {
		want[a] = true
	}
	for _, r := range Rules("cte") {
		if (r.Mode == ModeBlock) != want[r.ID] {
			t.Errorf("%s: modo %s inesperado", r.ID, r.Mode)
		}
	}
}

func TestCTeHelpers(t *testing.T) {
	if !cteCNPJOK(cteTCNPJ) || cteCNPJOK("11222333000182") || cteCNPJOK("00000000000000") {
		t.Error("CNPJ")
	}
	if !cteCPFOK(cteTCPF) || cteCPFOK("52998224726") || cteCPFOK("11111111111") {
		t.Error("CPF")
	}
	// CNPJ alfanumérico: DV com ord-48 (exemplo oficial 12.ABC.345/01DE-35).
	if !cteCNPJOK("12ABC34501DE35") {
		t.Error("CNPJ alfanumérico")
	}
	k := cteTestKey57("7")
	if r := cteKeyReason(k, "57", 2009); r != "" {
		t.Errorf("chave válida recusada: %s", r)
	}
	if r := cteKeyReason(cteBadKey(k), "57", 2009); r != "DV inválido" {
		t.Errorf("DV: %q", r)
	}
	if r := cteKeyReason(k, "55", 2009); r == "" {
		t.Error("modelo errado aceito")
	}
}

// As regras do CT-e recebem o conteúdo de <CTe> como as de NF-e e MDF-e (o dfecheck passa o CTe inteiro).
func TestCTeRulesAcceptCTeRoot(t *testing.T) {
	inf := map[string]any{"emit": map[string]any{"CNPJ": "00000000000000"}}
	direct, _ := Apply("cte", inf, Modes{})
	wrapped, _ := Apply("cte", map[string]any{"infCte": inf}, Modes{})
	if len(direct) == 0 || len(direct) != len(wrapped) {
		t.Fatalf("achados direto %d, dentro de CTe %d", len(direct), len(wrapped))
	}
}
