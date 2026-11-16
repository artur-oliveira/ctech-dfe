package nfcoms

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

const testCNPJ = "11222333000181"

func testOrg(t *testing.T, crt int) map[string]types.AttributeValue {
	t.Helper()
	org, err := attributevalue.MarshalMap(map[string]any{
		"name":   "Provedor Teste Ltda",
		"tax_id": testCNPJ,
		"person": map[string]any{
			"fantasy_name": "Provedor Teste",
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

func testBody() NfcomEmitBody {
	return NfcomEmitBody{
		Customer: NfcomCustomer{
			Name: "Cliente Exemplo", CNPJ: strp("33000167000101"), IEIndicator: "9",
			Address: NfcomAddress{Street: "Rua B", Number: "20", Neighborhood: "Bairro", IBGECode: "3550308",
				City: "Sao Paulo", PostalCode: "01002000", UF: "SP"},
		},
		Subscriber: NfcomSubscriber{Code: "ASS-001", Type: "1", ServiceUsed: "6", ContractNumber: strp("C-1"), ContractStart: strp("2026-01-01")},
		Items: []NfcomItem{{
			Code: "PLANO100", Description: "Internet 100 Mbps", ClassCode: "0100301", CFOP: strp("5307"), Unit: "4",
			Quantity: "1", UnitValue: "100.00", Discount: strp("10.00"), OtherCharges: strp("2.50"),
			ICMS: NfcomICMS{CST: "00", Rate: strp("18")}, PIS: &NfcomPIS{CST: "01", Rate: "0.65"}, COFINS: &NfcomCOFINS{CST: "01", Rate: "3"},
		}},
		Invoice: &NfcomInvoice{Competence: "202611", DueDate: "2026-12-10", BarCode: "12345678901234567890"},
	}
}

func strp(s string) *string { return &s }

func testParams(t *testing.T) buildParams {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	key := GenerateAccessKey("SP", testCNPJ, 1, 10, now, tpEmisNormal)
	return buildParams{
		org: testOrg(t, 3), orgPK: "ORG1", accessKey: key, serie: 1, number: 10, environment: 2, now: now,
		req: testBody(), tpEmis: tpEmisNormal, tech: TechData{Version: "ctech-dfe-test"},
	}
}

func TestBuildNFComValidatesAgainstSchemaAndRules(t *testing.T) {
	body := BuildNFCom(testParams(t))
	nfcom, ok := body["NFCom"].(map[string]any)
	if !ok {
		t.Fatalf("raiz NFCom ausente: %v", body)
	}
	s, err := validate.Load("nfcom")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range s.Validate(nfcom, validate.Options{Skip: []string{"Signature"}}) {
		t.Errorf("schema: %s %s %s", v.Path, v.Code, v.Message)
	}
	found, _ := validate.Apply("nfcom", nfcom, nil)
	for _, v := range found {
		t.Errorf("regra %s: %s", v.Rule, v.Message)
	}
}

func TestBuildNFComTotalsAndItem(t *testing.T) {
	nfcom := BuildNFCom(testParams(t))["NFCom"].(map[string]any)
	inf := nfcom["infNFCom"].(map[string]any)
	prod := inf["det"].([]any)[0].(map[string]any)["prod"].(map[string]any)
	if prod["vProd"] != "92.50" { // 1 x 100,00 - 10,00 + 2,50
		t.Errorf("vProd = %v", prod["vProd"])
	}
	tot := inf["total"].(map[string]any)
	if tot["vProd"] != "92.50" || tot["vNF"] != "92.50" || tot["vDesc"] != "10.00" || tot["vOutro"] != "2.50" {
		t.Errorf("total = %v", tot)
	}
	icms := inf["det"].([]any)[0].(map[string]any)["imposto"].(map[string]any)["ICMS00"].(map[string]any)
	if icms["vBC"] != "92.50" || icms["vICMS"] != "16.65" { // 92,50 x 18%
		t.Errorf("ICMS00 = %v", icms)
	}
	if tot["ICMSTot"].(map[string]any)["vICMS"] != "16.65" || tot["vPIS"] != "0.60" || tot["vCOFINS"] != "2.78" {
		t.Errorf("tributos do total = %v", tot)
	}
}

func TestBuildNFComAccessKeyAndID(t *testing.T) {
	p := testParams(t)
	nfcom := BuildNFCom(p)["NFCom"].(map[string]any)
	inf := nfcom["infNFCom"].(map[string]any)
	if inf["@Id"] != "NFCom"+p.accessKey || len(p.accessKey) != 44 || p.accessKey[20:22] != "62" || p.accessKey[34] != '1' || p.accessKey[35] != '0' {
		t.Errorf("chave/Id: %q %v", p.accessKey, inf["@Id"])
	}
	if got := services.CalcMod11DV(p.accessKey[:43]); got != p.accessKey[43:] {
		t.Errorf("DV = %s, want %s", p.accessKey[43:], got)
	}
	supl := nfcom["infNFComSupl"].(map[string]any)
	if q := supl["qrCodNFCom"].(string); !strings.Contains(q, "chNFCom="+p.accessKey) || !strings.Contains(q, "tpAmb=2") {
		t.Errorf("QR Code = %s", q)
	}
}

func TestBuildNFComSimplesNacionalUsesICMSSN(t *testing.T) {
	p := testParams(t)
	p.org = testOrg(t, 1)
	p.req.Items[0].ICMS = NfcomICMS{CST: "sn"}
	inf := BuildNFCom(p)["NFCom"].(map[string]any)["infNFCom"].(map[string]any)
	imp := inf["det"].([]any)[0].(map[string]any)["imposto"].(map[string]any)
	sn, ok := imp["ICMSSN"].(map[string]any)
	if !ok || sn["CST"] != "90" || sn["indSN"] != "1" {
		t.Errorf("imposto = %v", imp)
	}
}

func TestBuildNFComOfflineCarriesContingency(t *testing.T) {
	p := testParams(t)
	p.tpEmis = tpEmisOffline
	p.accessKey = GenerateAccessKey("SP", testCNPJ, 1, 10, p.now, tpEmisOffline)
	p.qrSign = "ASSINATURA"
	p.contingencyReason = "Falha de comunicacao com a SEFAZ"
	ide := BuildNFCom(p)["NFCom"].(map[string]any)["infNFCom"].(map[string]any)["ide"].(map[string]any)
	if ide["tpEmis"] != "2" || ide["xJust"] != p.contingencyReason || ide["dhCont"] == nil {
		t.Errorf("ide = %v", ide)
	}
}
