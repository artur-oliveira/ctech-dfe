package validate

import "testing"

// tTAGood: BP-e TA de 2026 com CST 000 e cClassTrib 000001 (0,1% IBS UF, 0% município, 0,9% CBS) que não dispara regra.
func tTAGood() m {
	d := tBPeGood()
	inf := d["infBPe"].(m)
	inf["imp"] = m{
		"IBSCBS": m{
			"CST": "000", "cClassTrib": "000001",
			"gIBSCBS": m{
				"vBC":     "90.00",
				"gIBSUF":  m{"pIBSUF": "0.1", "vIBSUF": "0.09"},
				"gIBSMun": m{"pIBSMun": "0", "vIBSMun": "0.00"},
				"vIBS":    "0.09",
				"gCBS":    m{"pCBS": "0.9", "vCBS": "0.81"},
			},
		},
		"vTotDFe": "100.00",
	}
	return d
}

func taFinding(t *testing.T, doc m, id string) bool {
	t.Helper()
	for _, r := range Rules("bpeta") {
		if r.ID == id {
			return len(r.Check(doc)) > 0
		}
	}
	t.Fatalf("regra %s não registrada", id)
	return false
}

func TestIBSCBSTablesLoad(t *testing.T) {
	if c, ok := IBSCBSClassByCode("200050"); !ok || c.CST != "200" || c.PRedIBS != 40 || !c.BPeTA {
		t.Fatalf("200050 = %+v %v", c, ok)
	}
	if c, ok := IBSCBSCSTByCode("200"); !ok || !c.GIBSCBS || !c.GRed || c.GDif {
		t.Fatalf("CST 200 = %+v %v", c, ok)
	}
	if got := len(IBSCBSClassesTA()); got != 8 {
		t.Errorf("classes do TA = %d, esperado 8 (tabela v1.71)", got)
	}
	if _, ok := IBSCBSClassByCode("999999"); ok {
		t.Error("código inexistente não pode resolver")
	}
}

func TestIBSCBSGoodDocHasNoFindings(t *testing.T) {
	if found, blocking := Apply("bpeta", tTAGood(), nil); len(found) != 0 || blocking {
		t.Fatalf("achados inesperados: %+v", found)
	}
}

func TestIBSCBSRules(t *testing.T) {
	imp := func(d m) m { return d["infBPe"].(m)["imp"].(m) }
	ibs := func(d m) m { return imp(d)["IBSCBS"].(m) }
	g := func(d m) m { return ibs(d)["gIBSCBS"].(m) }
	cases := []struct {
		id     string
		mutate func(d m)
	}{
		{"F76", func(d m) { ibs(d)["CST"] = "999" }},
		{"F77", func(d m) { ibs(d)["cClassTrib"] = "999999" }},
		{"F78", func(d m) { ibs(d)["cClassTrib"] = "200050" }},
		{"F79", func(d m) { ibs(d)["cClassTrib"] = "000002"; ibs(d)["CST"] = "000" }}, // classe de outro modelo
		{"F80", func(d m) { ibs(d)["CST"] = "410"; ibs(d)["cClassTrib"] = "410001" }},
		{"F81", func(d m) { delete(ibs(d), "gIBSCBS") }},
		{"F82", func(d m) { g(d)["gIBSUF"].(m)["pIBSUF"] = "0.5" }},
		{"F92", func(d m) { g(d)["gIBSMun"].(m)["pIBSMun"] = "0.05" }},
		{"F103", func(d m) { g(d)["gCBS"].(m)["pCBS"] = "1" }},
		{"F83", func(d m) { ibs(d)["CST"] = "200"; ibs(d)["cClassTrib"] = "200050" }},
		{"F107", func(d m) { g(d)["gCBS"].(m)["gRed"] = m{"pRedAliq": "40", "pAliqEfet": "0.54"} }},
		{"F87", func(d m) { ibs(d)["CST"] = "510" }},
		{"F109", func(d m) { g(d)["gCBS"].(m)["gDif"] = m{"pDif": "10", "vDif": "0.08"} }},
		{"F110", func(d m) {
			ibs(d)["CST"] = "510"
			g(d)["gCBS"].(m)["gDif"] = m{"pDif": "50", "vDif": "0.10"}
		}},
		{"F91", func(d m) { g(d)["gIBSUF"].(m)["vIBSUF"] = "0.50" }},
		{"F100", func(d m) { g(d)["gIBSMun"].(m)["vIBSMun"] = "0.50" }},
		{"F112", func(d m) { g(d)["gCBS"].(m)["vCBS"] = "5.00" }},
		{"F102", func(d m) { g(d)["vIBS"] = "1.00" }},
		{"F114", func(d m) { g(d)["gTribRegular"] = m{"CSTReg": "000", "cClassTribReg": "000001"} }},
		{"F115", func(d m) {
			g(d)["gTribRegular"] = m{"CSTReg": "999", "cClassTribReg": "000001", "pAliqEfetRegCBS": "1", "vTribRegCBS": "0.90"}
		}},
		{"F136", func(d m) {
			d["infBPe"].(m)["ide"].(m)["gCompraGov"] = m{"tpEnteGov": "1", "pRedutor": "0", "tpOperGov": "1"}
		}},
		{"F137", func(d m) {
			g(d)["gTribCompraGov"] = m{"pAliqIBSUF": "0.1", "vTribIBSUF": "0.09"}
		}},
		{"F138a", func(d m) { ibs(d)["gEstornoCred"] = m{"vIBSEstCred": "1.00", "vCBSEstCred": "1.00"} }},
		{"F138b", func(d m) { ibs(d)["CST"] = "410"; ibs(d)["cClassTrib"] = "410026"; delete(ibs(d), "gIBSCBS") }},
		{"F139", func(d m) {
			d["infBPe"].(m)["ide"].(m)["dhEmi"] = "2027-01-10T10:00:00-03:00"
			d["infBPe"].(m)["ide"].(m)["gCompraGov"] = m{"tpEnteGov": "1", "pRedutor": "0", "tpOperGov": "1"}
			g(d)["gIBSUF"].(m)["pIBSUF"] = "0.05"
		}},
		{"F139a-d", func(d m) {
			d["infBPe"].(m)["ide"].(m)["gCompraGov"] = m{"tpEnteGov": "5", "pRedutor": "0", "tpOperGov": "2"}
		}},
		{"F140", func(d m) { imp(d)["vTotDFe"] = "101.00" }},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			d := tTAGood()
			c.mutate(d)
			if !taFinding(t, d, c.id) {
				t.Fatalf("%s deveria disparar", c.id)
			}
		})
	}
}

// F140: em 2027 o total soma IBS e CBS.
func TestIBSCBSTotalDFeFrom2027(t *testing.T) {
	d := tTAGood()
	d["infBPe"].(m)["ide"].(m)["dhEmi"] = "2027-01-10T10:00:00-03:00"
	if !taFinding(t, d, "F140") {
		t.Fatal("em 2027 vTotDFe sem IBS e CBS deve disparar")
	}
	d["infBPe"].(m)["imp"].(m)["vTotDFe"] = "100.90"
	if taFinding(t, d, "F140") {
		t.Fatal("vBP 100,00 + IBS 0,09 + CBS 0,81 = 100,90")
	}
}

// F85: alíquota efetiva com redução de 40%.
func TestIBSCBSEffectiveRate(t *testing.T) {
	d := tTAGood()
	ibs := d["infBPe"].(m)["imp"].(m)["IBSCBS"].(m)
	ibs["CST"], ibs["cClassTrib"] = "200", "200050"
	cbs := ibs["gIBSCBS"].(m)["gCBS"].(m)
	cbs["gRed"] = m{"pRedAliq": "40", "pAliqEfet": "0.54"}
	cbs["vCBS"] = "0.49"
	uf := ibs["gIBSCBS"].(m)["gIBSUF"].(m)
	uf["gRed"] = m{"pRedAliq": "40", "pAliqEfet": "0.06"}
	uf["vIBSUF"] = "0.05"
	mun := ibs["gIBSCBS"].(m)["gIBSMun"].(m)
	mun["gRed"] = m{"pRedAliq": "40", "pAliqEfet": "0"}
	ibs["gIBSCBS"].(m)["vIBS"] = "0.05"
	for _, id := range []string{"F83", "F84", "F85", "F91", "F102", "F112", "F104", "F105", "F106"} {
		if taFinding(t, d, id) {
			t.Errorf("%s não deveria disparar", id)
		}
	}
	cbs["gRed"] = m{"pRedAliq": "40", "pAliqEfet": "0.90"}
	if !taFinding(t, d, "F106") {
		t.Error("F106 deveria acusar alíquota efetiva errada")
	}
}
