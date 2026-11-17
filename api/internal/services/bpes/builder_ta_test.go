package bpes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

func testTABody() BpeTAEmitBody {
	return BpeTAEmitBody{
		Variant: VariantTA, Presence: "2",
		Origin: BpePlace{UF: "SP", IBGECode: "3550308"}, Destination: BpePlace{UF: "RJ", IBGECode: "3304557"},
		Passage: BpeTAPassage{
			Boarding: "2026-11-20T10:00:00-03:00", Validity: "2026-12-20T10:00:00-03:00",
			Passenger: &BpePassenger{Name: "Maria Passageira", CPF: strp("52998224725"), DocType: "1", DocNumber: "123456789"},
		},
		Trips: []BpeTATrip{{
			FlightNumber: "1234", Carrier: "ABC", TripType: "00", FromAirport: "GRU", ToAirport: "SDU",
			ServiceType: "12", Accommodation: "6", LegType: "1", Departure: "2026-11-20T10:00:00-03:00",
		}},
		Components:   []BpeTAComponent{{Type: "01", Value: "300.00"}, {Type: "03", Value: "50.00"}},
		Discount:     strp("50.00"),
		DiscountType: strp("03"), DiscountDesc: strp("Promocao"),
		Payments: []BpePayment{{Type: "06", Value: "300.00"}},
		Tax:      BpeTATax{CST: "000", ClassTrib: "000001", IBSUFRate: strp("0.1"), IBSMunRate: strp("0.05"), CBSRate: strp("0.9")},
	}
}

func testTAParams(t *testing.T) buildParams {
	p := testParams(t)
	p.variant = VariantTA
	p.ta = testTABody()
	return p
}

func TestBuildBPeTAValidatesAgainstSchemaAndRules(t *testing.T) {
	body := BuildBPeTA(testTAParams(t))
	doc, ok := body["BPeTA"].(map[string]any)
	if !ok {
		t.Fatalf("raiz BPeTA ausente: %v", body)
	}
	s, err := validate.Load("bpeta")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range s.Validate(doc, validate.Options{Skip: []string{"Signature"}}) {
		t.Errorf("schema: %s %s %s", v.Path, v.Code, v.Message)
	}
	found, _ := validate.Apply("bpeta", doc, nil)
	for _, v := range found {
		t.Errorf("regra %s: %s", v.Rule, v.Message)
	}
	if _, has := doc["infBPeSupl"]; !has {
		t.Error("o TA tem QR Code (infBPeSupl)")
	}
}

func TestBuildBPeTAIdentityAndTotals(t *testing.T) {
	p := testTAParams(t)
	inf := BuildBPeTA(p)["BPeTA"].(map[string]any)["infBPe"].(map[string]any)
	ide := inf["ide"].(map[string]any)
	if ide["modal"] != "2" || ide["tpCompra"] != "0" || ide["mod"] != "63" || ide["tpBPe"] != "0" {
		t.Errorf("ide = %v", ide)
	}
	v := inf["infValorBPe"].(map[string]any)
	if v["vBP"] != "350.00" || v["vPgto"] != "300.00" || v["vTroco"] != "0.00" {
		t.Errorf("infValorBPe = %v", v)
	}
	trip := inf["infViagem"].([]any)[0].(map[string]any)
	if trip["nroVoo"] != "1234" || trip["SiglaCiaOperVoo"] != "ABC" || trip["cAeroOrig"] != "GRU" || trip["cAeroDest"] != "SDU" || trip["tpServ"] != "12" {
		t.Errorf("infViagem = %v", trip)
	}
	if _, has := inf["infPassagem"].(map[string]any)["cLocOrig"]; has {
		t.Error("o TA não tem cLocOrig")
	}
}

func TestBuildBPeTATax(t *testing.T) {
	imp := BuildBPeTA(testTAParams(t))["BPeTA"].(map[string]any)["infBPe"].(map[string]any)["imp"].(map[string]any)
	if imp["vTotDFe"] != "300.00" {
		t.Errorf("vTotDFe = %v", imp["vTotDFe"])
	}
	g := imp["IBSCBS"].(map[string]any)["gIBSCBS"].(map[string]any)
	if g["vBC"] != "300.00" || g["vIBS"] != "0.45" || g["gCBS"].(map[string]any)["vCBS"] != "2.70" { // 0,10% + 0,05% de 300; 0,90% de 300
		t.Errorf("gIBSCBS = %v", g)
	}
	p := testTAParams(t)
	p.ta.Tax = BpeTATax{CST: "410", ClassTrib: "410001"}
	imp = BuildBPeTA(p)["BPeTA"].(map[string]any)["infBPe"].(map[string]any)["imp"].(map[string]any)
	if _, has := imp["IBSCBS"].(map[string]any)["gIBSCBS"]; has {
		t.Error("sem alíquotas não há gIBSCBS")
	}
}

func TestBuildBPeTAIATANumber(t *testing.T) {
	p := testTAParams(t)
	p.ta.IATANumber = strp("957-1234567890")
	ide := BuildBPeTA(p)["BPeTA"].(map[string]any)["infBPe"].(map[string]any)["ide"].(map[string]any)
	if ide["nroBPIATA"] != "957-1234567890" {
		t.Errorf("nroBPIATA = %v", ide["nroBPIATA"])
	}
}

func TestValidateTABody(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(b *BpeTAEmitBody)
		wantErr string
	}{
		{"corpo válido", func(b *BpeTAEmitBody) {}, ""},
		{"pagamento insuficiente", func(b *BpeTAEmitBody) { b.Payments[0].Value = "10.00" }, "pagamentos"},
		{"conexão exige a data-hora", func(b *BpeTAEmitBody) { b.Trips[0].LegType = "3" }, "conexão"},
		{"embarque depois da validade", func(b *BpeTAEmitBody) { b.Passage.Validity = "2026-11-01T10:00:00-03:00" }, "validade"},
		{"sem voo", func(b *BpeTAEmitBody) { b.Trips = nil }, "trecho"},
	}
	for _, c := range cases {
		b := testTABody()
		c.mutate(&b)
		err := validateTABody(b)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: erro inesperado: %v", c.name, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("%s: erro = %v, quer conter %q", c.name, err, c.wantErr)
		}
	}
}

// 112140: fornecimento não realizado, só TA; IBS e CBS com duas casas.
func TestSupplyNotPerformedEnvelope(t *testing.T) {
	p := testTAParams(t)
	ec := &eventContext{environment: 2, cnpj: testCNPJ, emitUF: "SP"}
	body := buildEventEnvelope(ec, p.accessKey, TpEventoFornecimentoNaoRealizado, 1, supplyNotPerformedDet("135260000000001", "10.00", "5.00"))
	if found := dfecheck.EventFindings("bpe", body); len(found) != 0 {
		t.Fatalf("achados: %+v", found)
	}
	for variant, want := range map[string]bool{VariantNormal: false, VariantTM: false, VariantTA: true} {
		if got := eventAllowed(variant, TpEventoFornecimentoNaoRealizado); got != want {
			t.Errorf("eventAllowed(%s, 112140) = %v, quer %v", variant, got, want)
		}
	}
}
