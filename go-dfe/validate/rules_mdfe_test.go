package validate

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	tCNPJ1 = "11222333000181"
	tCNPJ2 = "11444777000161"
	tCNPJ3 = "33000167000101"
	tCPF1  = "52998224725"
	tCPF2  = "11144477735"
)

type m = map[string]any

func tKey(cuf, aamm, doc, mod, serie, n, tpEmis, cod string) string {
	k := cuf + aamm + doc + mod + serie + n + tpEmis + cod
	return k + fmt.Sprint(mdfeKeyDV(k))
}

func tCTeKey() string {
	return tKey("35", "2610", tCNPJ2, "57", "001", "000000123", "1", "12345678")
}

func tMDFeKey() string {
	return tKey("35", "2611", tCNPJ1, "58", "001", "000000010", "1", "12345678")
}

func tGood() m {
	key := tMDFeKey()
	return m{
		"infMDFe": m{
			"@Id": "MDFe" + key,
			"ide": m{
				"cUF": "35", "tpAmb": "2", "tpEmit": "1", "mod": "58", "serie": "1", "nMDF": "10", "cMDF": "12345678",
				"cDV": key[43:], "modal": "1", "dhEmi": "2026-11-05T10:00:00-03:00", "tpEmis": "1",
				"UFIni": "SP", "UFFim": "RJ",
				"infMunCarrega": []m{{"cMunCarrega": "3550308"}},
				"infPercurso":   []m{{"UFPer": "MG"}},
			},
			"emit": m{"CNPJ": tCNPJ1, "IE": "123456789", "enderEmit": m{"cMun": "3550308", "UF": "SP"}},
			"infModal": m{"@versaoModal": "3.00", "rodo": m{
				"veicTracao":  m{"placa": "ABC1D23", "tpRod": "03", "condutor": []m{{"CPF": tCPF1}}},
				"veicReboque": []m{{"placa": "ABC1234"}},
				"infANTT": m{
					"RNTRC":          "12345678",
					"infCIOT":        []m{{"CIOT": "123456789012", "CNPJ": tCNPJ1}},
					"valePed":        m{"disp": []m{{"CNPJForn": tCNPJ3, "CNPJPg": tCNPJ3}}, "categCombVeic": "02"},
					"infContratante": []m{{"CNPJ": tCNPJ2}},
					"infPag": []m{{
						"CNPJ": tCNPJ2, "vContrato": "1000.00", "indPag": "1", "vAdiant": "200.00",
						"Comp": []m{{"tpComp": "01", "vComp": "900.00"}, {"tpComp": "02", "vComp": "100.00"}},
						"infPrazo": []m{{"nParcela": "001", "dVenc": "2026-12-05", "vParcela": "400.00"},
							{"nParcela": "002", "dVenc": "2027-01-05", "vParcela": "400.00"}},
						"infBanc": m{"CNPJIPEF": tCNPJ3},
					}},
				},
			}},
			"infDoc": m{"infMunDescarga": []m{{
				"cMunDescarga": "3304557",
				"infCTe":       []m{{"chCTe": tCTeKey()}},
			}}},
			"seg": []m{{
				"infResp": m{"respSeg": "1", "CNPJ": tCNPJ1},
				"infSeg":  m{"xSeg": "Seguradora", "CNPJ": tCNPJ3}, "nApol": "A1", "nAver": "V1",
			}},
			"prodPred":   m{"tpCarga": "01", "xProd": "x", "NCM": "12345678", "infLotacao": m{"x": "y"}},
			"tot":        m{"qCTe": "1", "vCarga": "10.00", "cUnid": "01", "qCarga": "1.0000"},
			"autXML":     []m{{"CNPJ": tCNPJ2}},
			"infRespTec": m{"CNPJ": tCNPJ3},
		},
		"infMDFeSupl": m{"qrCodMDFe": "https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode?chMDFe=" + key + "&tpAmb=2"},
	}
}

// helpers de navegação para as mutações
func tInf(d m) m       { return d["infMDFe"].(m) }
func tIde(d m) m       { return tInf(d)["ide"].(m) }
func tRodo(d m) m      { return tInf(d)["infModal"].(m)["rodo"].(m) }
func tANTT(d m) m      { return tRodo(d)["infANTT"].(m) }
func tPag(d m) m       { return tANTT(d)["infPag"].([]m)[0] }
func tMun(d m) m       { return tInf(d)["infDoc"].(m)["infMunDescarga"].([]m)[0] }
func tSeg(d m) m       { return tInf(d)["seg"].([]m)[0] }
func tCTe(d m) m       { return tMun(d)["infCTe"].([]m)[0] }
func tProp(d m, doc m) { tRodo(d)["veicTracao"].(m)["prop"] = doc }

func setNow(t *testing.T) {
	old := mdfeNow
	mdfeNow = func() time.Time { return time.Date(2026, 11, 5, 10, 1, 0, 0, time.FixedZone("", -3*3600)) }
	t.Cleanup(func() { mdfeNow = old })
}

func TestMDFeValidators(t *testing.T) {
	for _, c := range []string{tCNPJ1, tCNPJ2, tCNPJ3, "00000000000191"} {
		if !mdfeValidCNPJ(c) {
			t.Errorf("CNPJ valido recusado: %s", c)
		}
	}
	for _, c := range []string{"11222333000182", "00000000000000", "123"} {
		if mdfeValidCNPJ(c) {
			t.Errorf("CNPJ invalido aceito: %s", c)
		}
	}
	for _, c := range []string{tCPF1, tCPF2, "12345678909"} {
		if !mdfeValidCPF(c) {
			t.Errorf("CPF valido recusado: %s", c)
		}
	}
	for _, c := range []string{"52998224726", "11111111111", "00000000000"} {
		if mdfeValidCPF(c) {
			t.Errorf("CPF invalido aceito: %s", c)
		}
	}
	// DV de chave: exemplo clássico do manual da NF-e
	if !mdfeKeyDVOK("35080599999090910270550010000000015180051273") {
		t.Error("DV da chave do manual deveria conferir")
	}
	if mdfeKeyDVOK("35080599999090910270550010000000015180051274") {
		t.Error("DV errado aceito")
	}
}

func TestMDFeGoodDocHasNoFindings(t *testing.T) {
	setNow(t)
	for _, r := range mdfeRules {
		if got := r.Check(tGood()); len(got) > 0 {
			t.Errorf("%s: documento valido gerou achado: %v", r.ID, got)
		}
	}
}

func TestMDFeCatalogShape(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range mdfeRules {
		if seen[r.ID] {
			t.Errorf("id repetido: %s", r.ID)
		}
		seen[r.ID] = true
		if r.Doc != "mdfe" || r.CStat == 0 || (r.Mode != ModeWarn && r.Mode != ModeBlock) {
			t.Errorf("%s: Doc/CStat/Mode invalidos", r.ID)
		}
	}
	if len(mdfeRules) < 50 {
		t.Errorf("catalogo com %d regras", len(mdfeRules))
	}
	if len(Rules("mdfe")) < len(mdfeRules) {
		t.Error("regras nao registradas no catalogo")
	}
	if got := Rules("mdfe"); len(got) == 0 {
		t.Fatal("sem regras mdfe")
	}
	if tGood()["infMDFe"] == nil {
		t.Fatal("doc de teste sem infMDFe")
	}
	if blocking := func() bool { _, b := Apply("mdfe", tGood(), nil); return b }(); blocking {
		t.Error("documento valido bloqueou") // relógio real: F79/F80 são Warn, não bloqueiam
	}
}

func TestMDFeRulesFail(t *testing.T) {
	setNow(t)
	badKey := func(k string) string { return k[:43] + fmt.Sprint((int(k[43]-'0')+1)%10) }
	cases := []struct {
		id  string
		bad func(d m)
	}{
		{"F02", func(d m) { tInf(d)["emit"].(m)["enderEmit"].(m)["UF"] = "RJ" }},
		{"F03", func(d m) { tIde(d)["nMDF"] = "11" }},
		{"F04", func(d m) {
			tInf(d)["@Id"] = "MDFe" + tKey("35", "1101", tCNPJ1, "58", "001", "000000010", "1", "12345678")
		}},
		{"F05", func(d m) { tInf(d)["@Id"] = "MDFe" + badKey(tMDFeKey()) }},
		{"F06", func(d m) { tInf(d)["infModal"].(m)["@versaoModal"] = "9.99" }},
		{"F08", func(d m) { tIde(d)["infMunCarrega"] = []m{{"cMunCarrega": "3304557"}} }},
		{"F10", func(d m) { tIde(d)["infMunCarrega"] = []m{{"cMunCarrega": "3550308"}, {"cMunCarrega": "3550308"}} }},
		{"F11", func(d m) { tMun(d)["cMunDescarga"] = "4106902" }},
		{"F13", func(d m) { tInf(d)["infDoc"].(m)["infMunDescarga"] = []m{tMun(d), {"cMunDescarga": "3304557"}} }},
		{"F14", func(d m) { tMun(d)["infNFe"] = []m{{"chNFe": "x"}} }},
		{"F15", func(d m) { tIde(d)["tpEmit"] = "2" }},
		{"F16", func(d m) {
			tIde(d)["tpEmit"] = "3"
			tIde(d)["UFFim"] = "SP"
			tIde(d)["infPercurso"] = nil
			tIde(d)["tpEmit"] = "3"
		}},
		{"F17", func(d m) { tIde(d)["tpEmit"] = "3" }},
		{"F18", func(d m) { tProp(d, m{"CPF": tCPF2}); tIde(d)["tpTransp"] = "1" }},
		{"F19", func(d m) { tProp(d, m{"CNPJ": tCNPJ2}); tIde(d)["tpTransp"] = "2" }},
		{"F20", func(d m) { tIde(d)["tpTransp"] = "1" }},
		{"F21", func(d m) { tIde(d)["indCarregaPosterior"] = "1" }},
		{"F22", func(d m) { tIde(d)["indCarregaPosterior"] = "1" }},
		{"F23", func(d m) { tIde(d)["indCarregaPosterior"] = "1"; tIde(d)["modal"] = "2" }},
		{"F24", func(d m) { tIde(d)["indCarregaPosterior"] = "1" }},
		{"F26", func(d m) { delete(tMun(d), "infCTe") }},
		{"F27", func(d m) { tIde(d)["indCarregaPosterior"] = "1" }},
		{"F28", func(d m) { tMun(d)["infCTe"] = []m{tCTe(d), tCTe(d)} }},
		{"F29", func(d m) { tMun(d)["infNFe"] = []m{{"chNFe": "a"}, {"chNFe": "a"}} }},
		{"F30", func(d m) { tCTe(d)["chCTe"] = badKey(tCTeKey()) }},
		{"F30a", func(d m) { tCTe(d)["chCTe"] = tKey("35", "2501", tCNPJ2, "57", "001", "000000123", "1", "12345678") }},
		{"F34", func(d m) {
			tIde(d)["modal"] = "3"
			tCTe(d)["infEntregaParcial"] = m{"qtdTotal": "1", "qtdParcial": "1"}
		}},
		{"F35", func(d m) { tCTe(d)["chCTe"] = tKey("35", "2610", tCNPJ2, "57", "001", "000000123", "5", "12345678") }},
		{"F36", func(d m) { tCTe(d)["SegCodBarra"] = "123456789012345678901234567890123456" }},
		{"F36b", func(d m) {
			tCTe(d)["infNFePrestParcial"] = []m{{"chNFe": tKey("35", "2401", tCNPJ2, "55", "001", "000000123", "1", "12345678")}}
		}},
		{"F36c", func(d m) { tCTe(d)["infNFePrestParcial"] = []m{{"chNFe": "a"}, {"chNFe": "a"}} }},
		{"F36d", func(d m) { tCTe(d)["infNFePrestParcial"] = []m{{"chNFe": "123"}} }},
		{"F37", func(d m) { tMun(d)["infNFe"] = []m{{"chNFe": "123"}} }},
		{"F37a", func(d m) {
			tMun(d)["infNFe"] = []m{{"chNFe": tKey("35", "2401", tCNPJ2, "55", "001", "000000123", "1", "12345678")}}
		}},
		{"F41", func(d m) {
			tMun(d)["infNFe"] = []m{{"chNFe": tKey("35", "2610", tCNPJ2, "55", "001", "000000123", "5", "12345678")}}
		}},
		{"F42", func(d m) {
			tMun(d)["infNFe"] = []m{{"chNFe": tKey("35", "2610", tCNPJ2, "55", "001", "000000123", "1", "12345678"), "SegCodBarra": "1"}}
		}},
		{"F43", func(d m) { tMun(d)["infMDFeTransp"] = []m{{"chMDFe": tMDFeKey()}} }},
		{"F45", func(d m) { tMun(d)["infMDFeTransp"] = []m{{"chMDFe": "123"}} }},
		{"F45a", func(d m) {
			tMun(d)["infMDFeTransp"] = []m{{"chMDFe": tKey("35", "2501", tCNPJ2, "58", "001", "000000123", "1", "12345678")}}
		}},
		{"F51", func(d m) { tInf(d)["tot"].(m)["qCTe"] = "2" }},
		{"F52", func(d m) { delete(tPag(d), "infPrazo") }},
		{"F53", func(d m) { tPag(d)["indPag"] = "0" }},
		{"F54", func(d m) { delete(tInf(d), "prodPred") }},
		{"F55", func(d m) { delete(tInf(d)["prodPred"].(m), "infLotacao") }},
		{"F55a", func(d m) { delete(tInf(d)["prodPred"].(m), "NCM") }},
		{"F55b", func(d m) { delete(tANTT(d), "infPag") }},
		{"F56", func(d m) { tPag(d)["CNPJ"] = "11222333000182" }},
		{"F57", func(d m) { tPag(d)["infBanc"] = m{"CNPJIPEF": "11222333000182"} }},
		{"F58", func(d m) { tPag(d)["vContrato"] = "999.00" }},
		{"F59", func(d m) { tPag(d)["infPrazo"].([]m)[0]["nParcela"] = "1" }},
		{"F60", func(d m) { tPag(d)["infPrazo"].([]m)[0]["dVenc"] = "2026-11-01" }},
		{"F61", func(d m) { tPag(d)["infPrazo"].([]m)[1]["dVenc"] = "2026-12-01" }},
		{"F62", func(d m) { tPag(d)["vAdiant"] = "100.00" }},
		{"F63", func(d m) { tPag(d)["indPag"] = "0" }},
		{"F64", func(d m) { tProp(d, m{"CNPJ": tCNPJ1}); tIde(d)["tpTransp"] = "1" }},
		{"F65", func(d m) { tProp(d, m{"CNPJ": tCNPJ3}); tIde(d)["tpTransp"] = "1" }},
		{"F66", func(d m) { tANTT(d)["infContratante"] = []m{{"CNPJ": tCNPJ2}, {"CNPJ": tCNPJ2}} }},
		{"F67", func(d m) { tInf(d)["emit"].(m)["CNPJ"] = "11222333000182" }},
		{"F68", func(d m) { e := tInf(d)["emit"].(m); delete(e, "CNPJ"); e["CPF"] = "52998224726" }},
		{"F69", func(d m) { tIde(d)["serie"] = "950" }},
		{"F70", func(d m) { e := tInf(d)["emit"].(m); delete(e, "CNPJ"); e["CPF"] = tCPF1 }},
		{"F71", func(d m) { e := tInf(d)["emit"].(m); delete(e, "CNPJ"); e["CPF"] = tCPF1; tIde(d)["serie"] = "950" }},
		{"F72", func(d m) { tInf(d)["emit"].(m)["IE"] = "" }},
		{"F77", func(d m) { tInf(d)["emit"].(m)["enderEmit"].(m)["cMun"] = "3304557" }},
		{"F79", func(d m) { tIde(d)["dhEmi"] = "2026-11-05T11:00:00-03:00" }},
		{"F80", func(d m) { tIde(d)["dhEmi"] = "2026-11-03T10:00:00-03:00" }},
		{"F89", func(d m) { tRodo(d)["veicTracao"].(m)["placa"] = "AB1234" }},
		{"F89c", func(d m) { delete(tRodo(d), "veicReboque") }},
		{"F90", func(d m) { tIde(d)["infPercurso"] = []m{{"UFPer": "AM"}} }},
		{"F91", func(d m) { delete(tInf(d), "seg") }},
		{"F92", func(d m) { delete(tSeg(d), "nAver") }},
		{"F93", func(d m) { tSeg(d)["infResp"] = m{"respSeg": "2"} }},
		{"F94", func(d m) {
			delete(tANTT(d), "infCIOT")
			delete(tANTT(d), "infContratante")
			tANTT(d)["valePed"] = m{"disp": []m{{"CNPJForn": tCNPJ3}}, "categCombVeic": "02"}
		}},
		{"F95", func(d m) { delete(tANTT(d)["valePed"].(m), "categCombVeic") }},
		{"F96", func(d m) { tANTT(d)["valePed"].(m)["disp"] = []m{{"CNPJForn": "11222333000182"}} }},
		{"F98", func(d m) { tANTT(d)["valePed"].(m)["disp"] = []m{{"CNPJForn": tCNPJ3, "CPFPg": "52998224726"}} }},
		{"F99", func(d m) { tRodo(d)["veicTracao"].(m)["condutor"] = []m{{"CPF": tCPF1}, {"CPF": tCPF1}} }},
		{"F100", func(d m) { tRodo(d)["veicTracao"].(m)["condutor"] = []m{{"CPF": "52998224726"}} }},
		{"F101", func(d m) { tANTT(d)["infCIOT"] = []m{{"CIOT": "1", "CNPJ": "11222333000182"}} }},
		{"F102", func(d m) { tANTT(d)["infContratante"] = []m{{"CNPJ": "11222333000182"}} }},
		{"F103", func(d m) { tProp(d, m{"CNPJ": "11222333000182"}) }},
		{"F104", func(d m) { tRodo(d)["veicReboque"] = []m{{"placa": "ABC1234", "prop": m{"CPF": "52998224726"}}} }},
		{"F105", func(d m) { tInf(d)["autXML"] = []m{{"CNPJ": "11222333000182"}} }},
		{"F106", func(d m) { tInf(d)["autXML"] = []m{{"CPF": "11111111111"}} }},
		{"F107", func(d m) { tInf(d)["autXML"] = []m{{"CNPJ": tCNPJ2}, {"CNPJ": tCNPJ2}} }},
		{"F108", func(d m) { delete(tANTT(d), "RNTRC") }},
		{"F113", func(d m) { delete(tANTT(d), "infCIOT") }},
		{"F114", func(d m) { delete(d, "infMDFeSupl") }},
		{"F115", func(d m) { d["infMDFeSupl"].(m)["qrCodMDFe"] = "https://example.com/mdfe/qrCode?chMDFe=1" }},
		{"F116", func(d m) {
			d["infMDFeSupl"].(m)["qrCodMDFe"] = "https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode?chMDFe=123&tpAmb=2"
		}},
		{"F117", func(d m) { tIde(d)["tpEmis"] = "2" }},
		{"F118", func(d m) {
			d["infMDFeSupl"].(m)["qrCodMDFe"] = d["infMDFeSupl"].(m)["qrCodMDFe"].(string) + "&sign=abc"
		}},
		{"F121", func(d m) { tInf(d)["infRespTec"] = m{"CNPJ": "11222333000182"} }},
		{"F123", func(d m) { tInf(d)["infSolicNFF"] = m{"xSolic": "x"} }},
		{"PAA01", func(d m) { tInf(d)["infPAA"] = m{"CNPJPAA": "11222333000182"} }},
	}
	byID := map[string]Rule{}
	for _, r := range mdfeRules {
		byID[r.ID] = r
	}
	covered := map[string]bool{}
	for _, c := range cases {
		r, ok := byID[c.id]
		if !ok {
			t.Errorf("%s: regra inexistente", c.id)
			continue
		}
		covered[c.id] = true
		d := tGood()
		c.bad(d)
		got := r.Check(d)
		if len(got) == 0 {
			t.Errorf("%s: documento invalido nao gerou achado", c.id)
		}
		for _, msg := range got {
			if strings.Contains(msg, tCNPJ1) || strings.Contains(msg, tCPF1) {
				t.Errorf("%s: mensagem vaza documento: %s", c.id, msg)
			}
		}
	}
	for id := range byID {
		if !covered[id] {
			t.Errorf("%s: sem caso de falha", id)
		}
	}
}

func TestMDFeBlockSet(t *testing.T) {
	want := []string{"F05", "F67", "F68", "F96", "F98", "F100", "F101", "F102", "F103", "F104", "F105", "F106"}
	got := map[string]bool{}
	for _, r := range mdfeRules {
		if r.Mode == ModeBlock {
			got[r.ID] = true
		}
	}
	for _, id := range want {
		if !got[id] {
			t.Errorf("%s deveria ser Block", id)
		}
	}
	if len(got) != len(want) {
		t.Errorf("Block inesperado: %v", got)
	}
}

func TestMDFeToleranceBoundary(t *testing.T) {
	setNow(t)
	d := tGood()
	tPag(d)["vContrato"] = "1000.01" // diferença de R$ 0,01 é tolerada em F58
	for _, r := range mdfeRules {
		if r.ID == "F58" && len(r.Check(d)) > 0 {
			t.Error("F58 deve tolerar R$ 0,01")
		}
	}
}

// CNPJ alfanumérico (IN RFB 2229/2024, NT conjunta 2025.001): exemplo oficial da RFB e chave com ele no meio.
func TestMDFeAlphanumericCNPJ(t *testing.T) {
	setNow(t)
	const alfa = "12ABC34501DE35"
	if !mdfeValidCNPJ(alfa) || mdfeValidCNPJ("12ABC34501DE34") {
		t.Fatal("DV do CNPJ alfanumerico (ord-48) nao confere com o exemplo da RFB")
	}
	key := tKey("35", "2610", alfa, "55", "001", "000000123", "1", "12345678")
	if r := mdfeKeyReason(key, "55", mdfeNFeMinYear, mdfeTpEmisNFe); r != "" {
		t.Fatalf("chave com CNPJ alfanumerico recusada: %s", r)
	}
	bad := key[:43] + fmt.Sprint((int(key[43]-'0')+1)%10)
	if r := mdfeKeyReason(bad, "55", mdfeNFeMinYear, mdfeTpEmisNFe); r != "DV invalido" {
		t.Fatalf("DV errado da chave alfanumerica: motivo %q", r)
	}
	d := tGood()
	tInf(d)["autXML"] = []m{{"CNPJ": alfa}}
	for _, r := range mdfeRules {
		if (r.ID == "F105" || r.ID == "F107") && len(r.Check(d)) > 0 {
			t.Errorf("%s recusou autXML com CNPJ alfanumerico valido", r.ID)
		}
	}
}
