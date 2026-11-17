package bpes

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

const testCNPJ = "11222333000181"

func strp(s string) *string { return &s }

func testOrg(t *testing.T, crt int) map[string]types.AttributeValue {
	t.Helper()
	org, err := attributevalue.MarshalMap(map[string]any{
		"name":   "Viacao Teste Ltda",
		"tax_id": testCNPJ,
		"person": map[string]any{
			"fantasy_name": "Viacao Teste",
			"crt":          crt,
			"addresses": []any{map[string]any{
				"street": "Rua A", "number": "10", "neighborhood": "Centro", "city": "Sao Paulo",
				"city_ibge_code": "3550308", "postal_code": "01001-000", "state_federation": "SP",
			}},
			"state_registrations": []any{map[string]any{"uf": "SP", "state_registration": "110042490114"}},
			"contacts":            map[string]any{"emails": []any{"fiscal@exemplo.com.br"}, "phones": []any{"1130000000"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return org
}

func testBody() BpeEmitBody {
	return BpeEmitBody{
		Modal: ModalRodoviario, Presence: "1",
		Origin: BpePlace{UF: "SP", IBGECode: "3550308"}, Destination: BpePlace{UF: "RJ", IBGECode: "3304557"},
		Passage: BpePassage{
			FromCode: "SPO01", FromName: "Rodoviaria Tiete", ToCode: "RIO01", ToName: "Rodoviaria Novo Rio",
			Boarding: "2026-11-20T10:00:00-03:00", Validity: "2026-12-20T10:00:00-03:00",
			Passenger: &BpePassenger{Name: "Maria Passageira", CPF: strp("52998224725"), DocType: "1", DocNumber: "123456789"},
		},
		Trips: []BpeTrip{{
			RouteCode: "R100", RouteName: "Sao Paulo x Rio", TripType: "00", ServiceType: "1", Accommodation: "1",
			LegType: "1", Departure: "2026-11-20T10:00:00-03:00", Seat: strp("12"),
		}},
		Components:   []BpeComponent{{Type: "01", Value: "80.00"}, {Type: "02", Value: "20.00"}},
		Discount:     strp("10.00"),
		DiscountType: strp("01"), DiscountDesc: strp("Idoso"),
		Payments: []BpePayment{{Type: "01", Value: "90.00"}},
		ICMS:     BpeICMS{CST: "00", Rate: strp("12")},
	}
}

func testParams(t *testing.T) buildParams {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	return buildParams{
		org: testOrg(t, 3), orgPK: "ORG1", accessKey: GenerateAccessKey("SP", testCNPJ, 1, 10, now, tpEmisNormal),
		serie: 1, number: 10, environment: 2, now: now, req: testBody(), tpEmis: tpEmisNormal,
		tech: TechData{Version: "ctech-dfe-test"},
	}
}

func TestBuildBPeValidatesAgainstSchemaAndRules(t *testing.T) {
	body := BuildBPe(testParams(t))
	doc, ok := body["BPe"].(map[string]any)
	if !ok {
		t.Fatalf("raiz BPe ausente: %v", body)
	}
	s, err := validate.Load("bpe")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range s.Validate(doc, validate.Options{Skip: []string{"Signature"}}) {
		t.Errorf("schema: %s %s %s", v.Path, v.Code, v.Message)
	}
	found, _ := validate.Apply("bpe", doc, nil)
	for _, v := range found {
		t.Errorf("regra %s: %s", v.Rule, v.Message)
	}
}

func TestBuildBPeTotalsAndTaxes(t *testing.T) {
	inf := BuildBPe(testParams(t))["BPe"].(map[string]any)["infBPe"].(map[string]any)
	v := inf["infValorBPe"].(map[string]any)
	// vBP = soma dos componentes; vPgto = vBP - desconto; sem troco.
	if v["vBP"] != "100.00" || v["vDesconto"] != "10.00" || v["vPgto"] != "90.00" || v["vTroco"] != "0.00" {
		t.Errorf("infValorBPe = %v", v)
	}
	icms := inf["imp"].(map[string]any)["ICMS"].(map[string]any)["ICMS00"].(map[string]any)
	if icms["vBC"] != "90.00" || icms["vICMS"] != "10.80" { // base = vPgto, 12%
		t.Errorf("ICMS00 = %v", icms)
	}
}

func TestBuildBPeChange(t *testing.T) {
	p := testParams(t)
	p.req.Payments = []BpePayment{{Type: "01", Value: "100.00"}}
	v := BuildBPe(p)["BPe"].(map[string]any)["infBPe"].(map[string]any)["infValorBPe"].(map[string]any)
	if v["vPgto"] != "90.00" || v["vTroco"] != "10.00" {
		t.Errorf("troco = %v", v)
	}
}

func TestBuildBPeAccessKeyAndQR(t *testing.T) {
	p := testParams(t)
	doc := BuildBPe(p)["BPe"].(map[string]any)
	inf := doc["infBPe"].(map[string]any)
	if inf["@Id"] != "BPe"+p.accessKey || len(p.accessKey) != 44 || p.accessKey[20:22] != "63" || p.accessKey[34] != '1' {
		t.Errorf("chave/Id: %q %v", p.accessKey, inf["@Id"])
	}
	if got := services.CalcMod11DV(p.accessKey[:43]); got != p.accessKey[43:] {
		t.Errorf("DV = %s, want %s", p.accessKey[43:], got)
	}
	if q := doc["infBPeSupl"].(map[string]any)["qrCodBPe"].(string); q == "" {
		t.Error("QR Code ausente")
	}
}

func TestBuildBPeSimplesNacional(t *testing.T) {
	p := testParams(t)
	p.req.ICMS = BpeICMS{CST: "sn"}
	imp := BuildBPe(p)["BPe"].(map[string]any)["infBPe"].(map[string]any)["imp"].(map[string]any)["ICMS"].(map[string]any)
	if sn, ok := imp["ICMSSN"].(map[string]any); !ok || sn["CST"] != "90" || sn["indSN"] != "1" {
		t.Errorf("ICMS = %v", imp)
	}
}

func TestBuildBPeOfflineCarriesContingency(t *testing.T) {
	p := testParams(t)
	p.tpEmis = tpEmisOffline
	p.accessKey = GenerateAccessKey("SP", testCNPJ, 1, 10, p.now, tpEmisOffline)
	p.qrSign = "ASSINATURA"
	p.contingencyReason = "Falha de comunicacao com a SEFAZ"
	ide := BuildBPe(p)["BPe"].(map[string]any)["infBPe"].(map[string]any)["ide"].(map[string]any)
	if ide["tpEmis"] != "2" || ide["xJust"] != p.contingencyReason || ide["dhCont"] == nil {
		t.Errorf("ide = %v", ide)
	}
}
