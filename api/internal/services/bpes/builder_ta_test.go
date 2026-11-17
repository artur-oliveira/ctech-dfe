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
		Tax:      BpeTATax{ClassTrib: "000001"},
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

// inYear2027 leva a emissão e as datas do bilhete para 2027, regenerando a chave de acesso.
func inYear2027(p *buildParams) {
	p.now = p.now.AddDate(1, 0, 0)
	p.accessKey = GenerateAccessKey("SP", testCNPJ, 1, 10, p.now, tpEmisNormal)
	p.ta.Passage.Boarding = "2027-11-20T10:00:00-03:00"
	p.ta.Passage.Validity = "2027-12-20T10:00:00-03:00"
	p.ta.Trips[0].Departure = "2027-11-20T10:00:00-03:00"
}

// taSchemaAndRules monta o TA com o imposto dado e exige zero achado de schema e de regra.
func taSchemaAndRules(t *testing.T, mutate func(p *buildParams)) map[string]any {
	t.Helper()
	p := testTAParams(t)
	mutate(&p)
	doc := BuildBPeTA(p)["BPeTA"].(map[string]any)
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
	return doc["infBPe"].(map[string]any)
}

// 2026: IBS UF 0,1%, município 0% e CBS 0,9% sobre o valor pago (300,00); vTotDFe é só o vBP (F140).
func TestBuildBPeTATax2026(t *testing.T) {
	inf := taSchemaAndRules(t, func(p *buildParams) {})
	imp := inf["imp"].(map[string]any)
	if imp["vTotDFe"] != "350.00" {
		t.Errorf("vTotDFe = %v (2026 não soma IBS e CBS: vBP 350,00)", imp["vTotDFe"])
	}
	g := imp["IBSCBS"].(map[string]any)["gIBSCBS"].(map[string]any)
	if g["vBC"] != "300.00" || g["vIBS"] != "0.30" || g["gCBS"].(map[string]any)["vCBS"] != "2.70" {
		t.Errorf("gIBSCBS = %v", g)
	}
	if imp["IBSCBS"].(map[string]any)["CST"] != "000" {
		t.Error("o CST sai da tabela a partir da classificação")
	}
}

// 2027: IBS 0,05% UF e município; a CBS de 2027 não tem alíquota em lei e vem do corpo; vTotDFe soma IBS e CBS.
func TestBuildBPeTATax2027(t *testing.T) {
	inf := taSchemaAndRules(t, func(p *buildParams) {
		inYear2027(p)
		p.ta.Tax.CBSRate = strp("8.8")
	})
	imp := inf["imp"].(map[string]any)
	if imp["vTotDFe"] != "376.70" { // 350,00 + IBS 0,30 (0,05% + 0,05% de 300) + CBS 26,40
		t.Errorf("vTotDFe = %v", imp["vTotDFe"])
	}
}

// 200050: transporte aéreo regional, redução de 40% em IBS e CBS (gRed com alíquota efetiva).
func TestBuildBPeTATaxReducedClass(t *testing.T) {
	inf := taSchemaAndRules(t, func(p *buildParams) { p.ta.Tax = BpeTATax{ClassTrib: "200050"} })
	g := inf["imp"].(map[string]any)["IBSCBS"].(map[string]any)["gIBSCBS"].(map[string]any)
	red := g["gCBS"].(map[string]any)["gRed"].(map[string]any)
	if red["pRedAliq"] != "40" || red["pAliqEfet"] != "0.54" {
		t.Errorf("gRed da CBS = %v", red)
	}
	if g["gCBS"].(map[string]any)["vCBS"] != "1.62" { // 300 x 0,54%
		t.Errorf("vCBS = %v", g["gCBS"].(map[string]any)["vCBS"])
	}
}

// 410001: imunidade, o grupo gIBSCBS não existe (CST veda). 410026: doação com estorno de crédito, exige o grupo de estorno.
func TestBuildBPeTATaxNoGroupAndReversal(t *testing.T) {
	inf := taSchemaAndRules(t, func(p *buildParams) { p.ta.Tax = BpeTATax{ClassTrib: "410001"} })
	if _, has := inf["imp"].(map[string]any)["IBSCBS"].(map[string]any)["gIBSCBS"]; has {
		t.Error("CST 410 veda gIBSCBS")
	}
	inf = taSchemaAndRules(t, func(p *buildParams) {
		p.ta.Tax = BpeTATax{ClassTrib: "410026", Donation: true, CreditReversal: &BpeTAReversal{IBS: "1.00", CBS: "2.00"}}
	})
	ibscbs := inf["imp"].(map[string]any)["IBSCBS"].(map[string]any)
	if ibscbs["indDoacao"] != "1" || ibscbs["gEstornoCred"].(map[string]any)["vCBSEstCred"] != "2.00" {
		t.Errorf("IBSCBS = %v", ibscbs)
	}
}

// Compra governamental da União em 2027: IBS da UF e do município zeram, gTribCompraGov guarda o que seria devido.
func TestBuildBPeTATaxGovernment(t *testing.T) {
	inf := taSchemaAndRules(t, func(p *buildParams) {
		inYear2027(p)
		p.ta.Tax.CBSRate = strp("8.8")
		p.ta.Tax.Government = &BpeTAGovernment{Entity: "1", Reducer: "5", Operation: "1"}
	})
	ide := inf["ide"].(map[string]any)["gCompraGov"].(map[string]any)
	if ide["tpEnteGov"] != "1" || ide["pRedutor"] != "5" {
		t.Errorf("gCompraGov = %v", ide)
	}
	g := inf["imp"].(map[string]any)["IBSCBS"].(map[string]any)["gIBSCBS"].(map[string]any)
	if g["gIBSUF"].(map[string]any)["pIBSUF"] != "0" || g["gIBSMun"].(map[string]any)["vIBSMun"] != "0.00" {
		t.Errorf("União: IBS UF e município devem zerar: %v", g)
	}
	if g["gTribCompraGov"].(map[string]any)["vTribIBSUF"] != "0.15" { // 0,05% de 300 sem o Art. 473
		t.Errorf("gTribCompraGov = %v", g["gTribCompraGov"])
	}
}

func TestValidateTATax(t *testing.T) {
	cases := []struct {
		name    string
		tax     BpeTATax
		year    int
		wantErr string
	}{
		{"válido 2026", BpeTATax{ClassTrib: "000001"}, 2026, ""},
		{"classe inexistente", BpeTATax{ClassTrib: "999999"}, 2026, "inexistente"},
		{"classe de outro modelo", BpeTATax{ClassTrib: "000002"}, 2026, "não permitida"},
		{"doação vedada", BpeTATax{ClassTrib: "410001", Donation: true}, 2026, "doação"},
		{"doação exigida", BpeTATax{ClassTrib: "410003"}, 2026, "doação"},
		{"alíquota fixada em lei", BpeTATax{ClassTrib: "000001", IBSUFRate: strp("1")}, 2026, "fixada em lei"},
		{"CBS 2027 sem alíquota", BpeTATax{ClassTrib: "000001"}, 2027, "informe a alíquota"},
		{"alíquota inválida", BpeTATax{ClassTrib: "000001", CBSRate: strp("abc")}, 2027, "inválida"},
		{"alíquota negativa", BpeTATax{ClassTrib: "000001", CBSRate: strp("-1")}, 2027, "inválida"},
		{"estorno sem classe", BpeTATax{ClassTrib: "000001", CreditReversal: &BpeTAReversal{IBS: "1", CBS: "1"}}, 2026, "estorno"},
		{"classe exige estorno", BpeTATax{ClassTrib: "410026", Donation: true}, 2026, "estorno"},
		{"regular sem classe", BpeTATax{ClassTrib: "000001", Regular: &BpeTARegular{CST: "000", ClassTrib: "000001", IBSUFRate: "0", IBSMunRate: "0", CBSRate: "0"}}, 2026, "regular"},
		{"diferimento sem CST", BpeTATax{ClassTrib: "000001", Deferral: &BpeTAEntes{CBS: strp("10")}}, 2026, "diferimento"},
		{"grupo em CST sem IBS", BpeTATax{ClassTrib: "410001", Devolution: &BpeTAEntes{CBS: strp("1")}}, 2026, "não admite os grupos"},
		{"compra gov com chave indevida", BpeTATax{ClassTrib: "000001", Government: &BpeTAGovernment{Entity: "1", Reducer: "0", Operation: "1", References: []string{"35261112345678000190630010000000101012345671"}}}, 2026, "não leva chave"},
		{"compra gov exige uma chave", BpeTATax{ClassTrib: "000001", Government: &BpeTAGovernment{Entity: "1", Reducer: "0", Operation: "2"}}, 2026, "exige uma chave"},
		{"chave malformada", BpeTATax{ClassTrib: "000001", Government: &BpeTAGovernment{Entity: "1", Reducer: "0", Operation: "3", References: []string{"abc"}}}, 2026, "inválida"},
	}
	for _, c := range cases {
		err := validateTATax(c.tax, c.year)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: erro inesperado: %v", c.name, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("%s: erro = %v, quer conter %q", c.name, err, c.wantErr)
		}
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
		err := validateTABody(b, 2026)
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
