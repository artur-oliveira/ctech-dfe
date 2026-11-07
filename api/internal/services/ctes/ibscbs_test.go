package ctes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	dfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// CNPJ válido que não é ator do CT-e de teste.
const cnpjAut = "34028316000103"

func sampleIBSCBS() *CteIBSCBS {
	return &CteIBSCBS{CST: "000", ClassTrib: "000001", BaseValue: "1850.00", IBSUFRate: "0.1", IBSMunRate: "0", CBSRate: "0.9"}
}

func TestIBSCBSCalculationAndRegime(t *testing.T) {
	r, err := resolveIBSCBS(sampleIBSCBS(), 3)
	if err != nil {
		t.Fatal(err)
	}
	g := r.node["gIBSCBS"].(map[string]any)
	if g["gIBSUF"].(map[string]any)["vIBSUF"] != "1.85" || g["gIBSUF"].(map[string]any)["pIBSUF"] != "0.10" ||
		g["gCBS"].(map[string]any)["vCBS"] != "16.65" || g["vIBS"] != "1.85" {
		t.Fatalf("grupo = %+v", g)
	}
	// vTotDFe: só vTPrest em 2026; soma IBS e CBS a partir de 2027 (IBSCBS-365).
	v := baseParams(t, RoleRemetente).vTPrest
	if r.totalDFe(v, 2026) != "1850.00" || r.totalDFe(v, 2027) != "1868.50" {
		t.Fatalf("vTotDFe = %s / %s", r.totalDFe(v, 2026), r.totalDFe(v, 2027))
	}
	for _, crt := range []int{1, 2, 4} {
		if _, err := resolveIBSCBS(sampleIBSCBS(), crt); code(err) != CodeIBSCBSRegime {
			t.Fatalf("CRT %d: %v", crt, err)
		}
	}
	if r, err := resolveIBSCBS(nil, 1); r != nil || err != nil {
		t.Fatal("sem IBS/CBS informado nada muda")
	}
}

// O documento com IBSCBS, vTotDFe e autXML passa no schema, não dispara regra do MOC e sai na ordem do XSD.
func TestBuildCTe_IBSCBSEAutXML(t *testing.T) {
	p := baseParams(t, RoleRemetente)
	p.crt = 3
	var err error
	if p.ibscbs, err = resolveIBSCBS(sampleIBSCBS(), 3); err != nil {
		t.Fatal(err)
	}
	if p.autXML, err = resolveAuthorizedXML([]CteAuthorizedXML{{Doc: cnpjAut}, {Doc: cpfC}}, []string{cnpjA, cnpjB}); err != nil {
		t.Fatal(err)
	}
	body := BuildCTe(p)
	cte := body["CTe"].(map[string]any)
	for _, v := range dfecheck.Shadow(dfecheck.SpecCTe, "cte", p.accessKey, cte) {
		if v.Code == "rule" && v.Severity != validate.SevError {
			continue
		}
		t.Errorf("%s %s %s: %s", v.Path, v.Code, v.Rule, v.Message)
	}
	if err := gateCTe(body); err != nil {
		t.Fatal(err)
	}
	raw, err := dfe.BuildXMLFragment(cte, "CTe", cteXMLNS)
	if err != nil {
		t.Fatal(err)
	}
	last, x := -1, string(raw)
	for _, tag := range []string{"<imp>", "<ICMS>", "<IBSCBS>", "<CST>", "<cClassTrib>", "<gIBSCBS>", "<vBC>", "<gIBSUF>", "<gIBSMun>", "<vIBS>",
		"<gCBS>", "</IBSCBS>", "<vTotDFe>1850.00</vTotDFe>", "</imp>", "<autXML>", "<CNPJ>" + cnpjAut, "<CPF>" + cpfC, "<infRespTec>"} {
		i := strings.Index(x[last+1:], tag)
		if i < 0 {
			t.Fatalf("%s fora de ordem ou ausente em\n%s", tag, x)
		}
		last += i + 1
	}
}

func TestAuthorizedXMLRefusals(t *testing.T) {
	cases := map[string][]CteAuthorizedXML{
		"DV inválido": {{Doc: "11222333000199"}},
		"repetido":    {{Doc: cnpjAut}, {Doc: cnpjAut}},
		"é ator":      {{Doc: cnpjA}},
	}
	for name, in := range cases {
		if _, err := resolveAuthorizedXML(in, []string{cnpjA}); code(err) != CodeAuthXMLInvalid {
			t.Errorf("%s: %v", name, err)
		}
	}
	many := make([]CteAuthorizedXML, MaxAuthorizedXML+1)
	if _, err := resolveAuthorizedXML(many, nil); code(err) != CodeAuthXMLInvalid {
		t.Errorf("mais de 10: %v", err)
	}
}
