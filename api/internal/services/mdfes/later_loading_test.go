package mdfes

import (
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

func boolPtr(b bool) *bool { return &b }

var munSP = MdfeMun{IBGECode: "3550308", City: "Sao Paulo"}

func laterLoadingReq() MdfeEmitBody {
	return MdfeEmitBody{Loadings: []MdfeMun{munSP}, Unloadings: []MdfeMun{munSP}, CargoValue: strPtr("1500.00"), CargoWeight: strPtr("1000")}
}

func TestResolveLaterLoading(t *testing.T) {
	cases := []struct {
		req       *bool
		config    bool
		docs      int
		wantLater bool
	}{
		{nil, false, 0, false},
		{nil, true, 0, true},
		{nil, true, 2, false}, // configuração ligada, mas o pedido trouxe documentos: emissão normal
		{boolPtr(true), false, 0, true},
		{boolPtr(false), true, 0, false},
	}
	for i, c := range cases {
		if got := resolveLaterLoading(c.req, c.config, c.docs); got != c.wantLater {
			t.Errorf("caso %d: got %v want %v", i, got, c.wantLater)
		}
	}
}

func wantRule(t *testing.T, err error, rule string) {
	t.Helper()
	p, ok := err.(*problem.Problem)
	if !ok || p.Status != 422 || p.Code != dfecheck.CodeFiscalPrevalidation || len(p.Errors) != 1 || p.Errors[0].Params["rule"] != rule {
		t.Fatalf("want 422 regra %s, got %#v", rule, err)
	}
}

func TestLaterLoadingCargoRules(t *testing.T) {
	cases := []struct {
		rule  string
		modal string
		mut   func(r *MdfeEmitBody)
	}{
		{"F27", ModalRodoviario, func(r *MdfeEmitBody) { r.Documents = []MdfeDocRef{{Type: docTypeNFe}} }},
		{"F27", ModalRodoviario, func(r *MdfeEmitBody) { r.TransportedMdfes = []MdfeTransportedBody{{}} }},
		{"F23", ModalAereo, func(*MdfeEmitBody) {}},
		{"F24", ModalRodoviario, func(r *MdfeEmitBody) { r.EmitterType = strPtr(tpEmitPrestador) }},
		{"F21", ModalRodoviario, func(r *MdfeEmitBody) { r.Unloadings = []MdfeMun{{IBGECode: "3509502", City: "Campinas"}} }},
		{"F21", ModalRodoviario, func(r *MdfeEmitBody) { r.Loadings = nil }},
		{"F22", ModalRodoviario, func(r *MdfeEmitBody) { r.UFEnd = "RJ" }},
		{"F22", ModalRodoviario, func(r *MdfeEmitBody) { r.UFStart = ufExterior }},
	}
	for _, c := range cases {
		req := laterLoadingReq()
		c.mut(&req)
		_, err := laterLoadingCargo(c.modal, req)
		wantRule(t, err, c.rule)
	}
}

// O carregamento posterior montado de ponta a ponta passa nas regras F21 a F27 do catálogo (as mesmas que o
// portão bloqueia) e sai sem documentos, com indCarregaPosterior=1 e sem qNFe/qCTe.
func TestLaterLoadingBuildPassesTheGateRules(t *testing.T) {
	cargo, err := laterLoadingCargo(ModalRodoviario, laterLoadingReq())
	if err != nil {
		t.Fatal(err)
	}
	if cargo.ufIni != "SP" || cargo.ufFim != "SP" || cargo.totalValue.StringFixed(2) != "1500.00" || cargo.prodPred.XProd != defaultXProd {
		t.Fatalf("carga = %+v", cargo)
	}
	p := baseParams(nil)
	p.cargo = cargo
	p.indCarregaPosterior = true
	body := BuildMDFe(p)
	inf := body["MDFe"].(map[string]any)["infMDFe"].(map[string]any)
	if inf["ide"].(map[string]any)["indCarregaPosterior"] != indSim {
		t.Fatal("indCarregaPosterior ausente")
	}
	tot := inf["tot"].(map[string]any)
	if tot["qNFe"] != nil || tot["qCTe"] != nil {
		t.Fatalf("tot sem documentos não informa contagem: %v", tot)
	}
	for _, v := range dfecheck.GateMDFe(body, blockRules) {
		switch v.Rule {
		case "F21", "F22", "F23", "F24", "F26", "F27", "F51":
			t.Errorf("%s: %s", v.Rule, v.Message)
		}
	}
}

func TestBuildMDFeAutXML(t *testing.T) {
	p := baseParams(nil)
	p.autXML = []string{"52998224725", "12.ABC.345/01DE-35"}
	inf := BuildMDFe(p)["MDFe"].(map[string]any)["infMDFe"].(map[string]any)
	aut, ok := inf["autXML"].([]map[string]any)
	if !ok || len(aut) != 2 || aut[0]["CPF"] != "52998224725" || aut[1]["CNPJ"] != "12ABC34501DE35" {
		t.Fatalf("autXML = %#v", inf["autXML"])
	}
	if _, ok := BuildMDFe(baseParams(nil))["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["autXML"]; ok {
		t.Fatal("sem autorizados o grupo não sai")
	}
}

// O portão recusa por id de regra: autorizado ao XML com DV errado (F106) e repetido (F107), contagem de
// documentos (F51) e soma de componentes do pagamento (F58).
func TestGateMDFeBlocksPromotedRules(t *testing.T) {
	p := baseParams(nil)
	p.autXML = []string{"52998224726", "11144477735", "11144477735"}
	body := BuildMDFe(p)
	inf := body["MDFe"].(map[string]any)["infMDFe"].(map[string]any)
	inf["tot"].(map[string]any)["qNFe"] = "9"
	inf["infModal"].(map[string]any)["rodo"].(map[string]any)["infANTT"] = map[string]any{"infPag": []map[string]any{{
		"CNPJ": "11222333000181", "vContrato": "100.00", "indPag": "0",
		"Comp": []map[string]any{{"tpComp": "01", "vComp": "90.00"}},
	}}}
	inf["ide"].(map[string]any)["tpEmit"] = tpEmitPrestador
	inf["ide"].(map[string]any)["tpTransp"] = "1"
	err := gateMDFe(body)
	pr, ok := err.(*problem.Problem)
	if !ok || pr.Status != 422 || pr.Code != dfecheck.CodeFiscalPrevalidation {
		t.Fatalf("want 422 fiscal_prevalidation, got %#v", err)
	}
	got := map[string]bool{}
	for _, e := range pr.Errors {
		got[e.Field] = true
	}
	for _, id := range []string{"F106", "F107", "F51", "F58"} {
		if !got[id] {
			t.Errorf("%s não bloqueou: %v", id, got)
		}
	}
	for _, id := range []string{"F72", "F113", "F30a", "F89"} { // fora do conjunto bloqueante
		if got[id] {
			t.Errorf("%s bloqueou, mas é só shadow", id)
		}
	}
}
