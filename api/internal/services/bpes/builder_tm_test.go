package bpes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

func testTMBody() BpeTMEmitBody {
	return BpeTMEmitBody{
		Modal: ModalFerroviario, CFOP: "5357", Competence: "2026-11-01",
		Groups: []BpeTMGroup{{
			OriginUF: "SP", Plate: strp("ABC1D23"), Prefix: strp("L100"),
			Items: []BpeTMItem{
				{OriginIBGE: "3550308", DestIBGE: strp("3509502"), Passengers: "120", Value: "480.00",
					Components: []BpeTMComponent{{Name: "Tarifa comum", Quantity: 100}, {Name: "Integrada", Quantity: 20}},
					ICMS:       BpeICMS{CST: "00", Rate: strp("12")}},
				{OriginIBGE: "3550308", Passengers: "30", Value: "120.00",
					Components: []BpeTMComponent{{Name: "Tarifa comum", Quantity: 30}},
					ICMS:       BpeICMS{CST: "40"}},
			},
		}},
	}
}

func testTMParams(t *testing.T) buildParams {
	p := testParams(t)
	p.variant = VariantTM
	p.tm = testTMBody()
	p.accessKey = GenerateAccessKey("SP", testCNPJ, 1, 10, p.now, tpEmisNormal)
	return p
}

func TestBuildBPeTMValidatesAgainstSchemaAndRules(t *testing.T) {
	body := BuildBPeTM(testTMParams(t))
	doc, ok := body["BPeTM"].(map[string]any)
	if !ok {
		t.Fatalf("raiz BPeTM ausente: %v", body)
	}
	s, err := validate.Load("bpetm")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range s.Validate(doc, validate.Options{Skip: []string{"Signature"}}) {
		t.Errorf("schema: %s %s %s", v.Path, v.Code, v.Message)
	}
	found, _ := validate.Apply("bpetm", doc, nil)
	for _, v := range found {
		t.Errorf("regra %s: %s", v.Rule, v.Message)
	}
	if _, has := doc["infBPeSupl"]; has {
		t.Error("o TM não tem infBPeSupl (sem QR Code)")
	}
}

func TestBuildBPeTMTotalsAndIdentity(t *testing.T) {
	p := testTMParams(t)
	inf := BuildBPeTM(p)["BPeTM"].(map[string]any)["infBPe"].(map[string]any)
	tot := inf["total"].(map[string]any)
	if tot["qPass"] != "150" || tot["vBP"] != "600.00" {
		t.Errorf("total = %v", tot)
	}
	icmsTot := tot["ICMSTot"].(map[string]any)
	if icmsTot["vBC"] != "480.00" || icmsTot["vICMS"] != "57.60" { // só o item com ICMS 00: 12% de 480
		t.Errorf("ICMSTot = %v", icmsTot)
	}
	ide := inf["ide"].(map[string]any)
	if ide["tpBPe"] != "4" || ide["CFOP"] != "5357" || ide["dCompet"] != "2026-11-01" || ide["mod"] != "63" || ide["modal"] != "4" {
		t.Errorf("ide = %v", ide)
	}
	if inf["@Id"] != "BPe"+p.accessKey || services.CalcMod11DV(p.accessKey[:43]) != p.accessKey[43:] {
		t.Errorf("Id/DV: %v %s", inf["@Id"], p.accessKey)
	}
	g := inf["detBPeTM"].([]any)[0].(map[string]any)
	comp := g["det"].([]any)[0].(map[string]any)["Comp"].([]any)[0].(map[string]any)
	if comp["qComp"] != "00100" { // 5 dígitos
		t.Errorf("Comp = %v", comp)
	}
}

func TestBuildBPeTMOfflineHasNoQR(t *testing.T) {
	p := testTMParams(t)
	p.tpEmis = tpEmisOffline
	p.contingencyReason = "Falha de comunicacao com a SEFAZ"
	doc := BuildBPeTM(p)["BPeTM"].(map[string]any)
	ide := doc["infBPe"].(map[string]any)["ide"].(map[string]any)
	if ide["tpEmis"] != "2" || ide["xJust"] != p.contingencyReason || ide["dhCont"] == nil {
		t.Errorf("ide = %v", ide)
	}
	if _, has := doc["infBPeSupl"]; has {
		t.Error("TM offline não leva QR Code")
	}
}

func TestValidateTMBody(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(b *BpeTMEmitBody)
		wantErr string
	}{
		{"corpo válido", func(b *BpeTMEmitBody) {}, ""},
		{"passageiros inválidos", func(b *BpeTMEmitBody) { b.Groups[0].Items[0].Passengers = "x" }, "passageiros"},
		{"valor inválido", func(b *BpeTMEmitBody) { b.Groups[0].Items[0].Value = "-5" }, "valor"},
		{"alíquota do ICMS 00 ausente", func(b *BpeTMEmitBody) { b.Groups[0].Items[0].ICMS = BpeICMS{CST: "00"} }, "alíquota"},
		{"competência inválida", func(b *BpeTMEmitBody) { b.Competence = "novembro" }, "competência"},
		{"contador final menor que o inicial", func(b *BpeTMEmitBody) {
			b.Groups[0].Items[0].StartCounter, b.Groups[0].Items[0].EndCounter = strp("200"), strp("100")
		}, "contador"},
		{"CFOP inválido", func(b *BpeTMEmitBody) { b.CFOP = "5300" }, "CFOP"},
	}
	for _, c := range cases {
		b := testTMBody()
		c.mutate(&b)
		err := validateTMBody(b)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: erro inesperado: %v", c.name, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("%s: erro = %v, quer conter %q", c.name, err, c.wantErr)
		}
	}
}
