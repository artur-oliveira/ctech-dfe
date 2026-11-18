package bpes

import (
	"strings"
	"testing"
)

func sp(s string) *string { return &s }

func normalLine() BpeLine {
	return BpeLine{
		Name: "SP x RJ", Variant: VariantNormal, Modal: sp("rodoviario"),
		Origin:      &BpeLinePlace{UF: "SP", IBGECode: "3550308", City: "Sao Paulo"},
		Destination: &BpeLinePlace{UF: "RJ", IBGECode: "3304557", City: "Rio de Janeiro"},
		RouteCode:   sp("L100"), RouteName: sp("SAO PAULO X RIO"), TripType: sp("00"), ServiceType: sp("6"), Accommodation: sp("1"),
		ICMS: &BpeICMS{CST: "00", Rate: sp("12")},
	}
}

func TestBpeLineValidateByVariant(t *testing.T) {
	if err := normalLine().Validate(); err != nil {
		t.Fatalf("linha comum válida: %v", err)
	}
	bad := normalLine()
	bad.ICMS = &BpeICMS{CST: "00"}
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "alíquota") {
		t.Errorf("ICMS 00 sem alíquota: %v", err)
	}
	bad = normalLine()
	bad.ServiceType = sp("12") // serviço do TA
	if err := bad.Validate(); err == nil {
		t.Error("serviço de outra variante")
	}
	bad = normalLine()
	bad.RouteCode = nil
	if err := bad.Validate(); err == nil {
		t.Error("sem código da linha")
	}

	ta := BpeLine{
		Name: "GRU x GIG", Variant: VariantTA,
		Origin:      &BpeLinePlace{UF: "SP", IBGECode: "3550308", City: "Sao Paulo"},
		Destination: &BpeLinePlace{UF: "RJ", IBGECode: "3304557", City: "Rio de Janeiro"},
		Carrier:     sp("LA"), FlightNumber: sp("LA3000"), FromAirport: sp("GRU"), ToAirport: sp("GIG"),
		TripType: sp("00"), ServiceType: sp("12"), Accommodation: sp("6"), Tax: &BpeTATax{ClassTrib: "000001"},
	}
	if err := ta.Validate(); err != nil {
		t.Fatalf("linha TA válida: %v", err)
	}
	ta.Tax = nil
	if err := ta.Validate(); err == nil {
		t.Error("TA sem classificação tributária")
	}

	tm := BpeLine{
		Name: "Metro", Variant: VariantTM, Modal: sp("ferroviario"), CFOP: sp("5353"), OriginUF: sp("SP"),
		Legs:           []BpeLineLeg{{OriginIBGE: "3550308", ICMS: BpeICMS{CST: "40"}}},
		PassengerTypes: []string{"Inteira", "Meia"},
	}
	if err := tm.Validate(); err != nil {
		t.Fatalf("linha TM válida: %v", err)
	}
	tm.Legs[0].ICMS = BpeICMS{CST: "90"}
	if err := tm.Validate(); err == nil || !strings.Contains(err.Error(), "trecho 1") {
		t.Errorf("trecho com ICMS 90 sem alíquota: %v", err)
	}
	tm.Legs = nil
	if err := tm.Validate(); err == nil {
		t.Error("TM sem trecho")
	}
}

func TestPartiesFromPeople(t *testing.T) {
	person := map[string]any{
		"name": "  Maria   da Silva ",
		"addresses": []any{map[string]any{
			"street": "Rua A", "number": "", "neighborhood": "Centro", "city_ibge_code": "3550308",
			"city": "Sao Paulo", "postal_code": "01001-000", "state_federation": "SP",
		}},
		"state_registrations": []any{map[string]any{"uf": "SP", "state_registration": "110.042.490.114"}},
	}
	b, err := buyerOf("CNPJ_33000167000101", person)
	if err != nil || b.Name != "Maria da Silva" || b.CNPJ == nil || b.IE == nil || *b.IE != "110042490114" || b.Address.Number != "S/N" {
		t.Errorf("comprador = %+v %v", b, err)
	}
	if _, err := buyerOf("XYZ_1", person); err == nil {
		t.Error("documento desconhecido")
	}
	a, err := agencyOf("CNPJ_33000167000101", person)
	if err != nil || a.CNPJ != "33000167000101" {
		t.Errorf("agência = %+v %v", a, err)
	}
	if _, err := agencyOf("CPF_52998224725", person); err == nil {
		t.Error("agência exige CNPJ")
	}
	p, err := passengerOf("CPF_52998224725", person)
	if err != nil || p.CPF == nil || p.DocType != "5" || p.DocNumber != "52998224725" {
		t.Errorf("passageiro = %+v %v", p, err)
	}
	if _, err := passengerOf("CNPJ_33000167000101", person); err == nil {
		t.Error("passageiro exige CPF")
	}
	delete(person["addresses"].([]any)[0].(map[string]any), "city_ibge_code")
	if _, err := buyerOf("CNPJ_33000167000101", person); err == nil {
		t.Error("endereço sem município")
	}
}

func TestCleanPassenger(t *testing.T) {
	p := cleanPassenger(&BpePassenger{Name: "  João   Souza ", DocNumber: " 123\t456 ", DocType: "1"})
	if p.Name != "João Souza" || p.DocNumber != "123 456" {
		t.Errorf("passageiro limpo = %+v", p)
	}
	if cleanPassenger(nil) != nil {
		t.Error("nil permanece nil")
	}
}
