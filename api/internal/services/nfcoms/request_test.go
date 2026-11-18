package nfcoms

import (
	"strings"
	"testing"
)

func fullPerson() map[string]any {
	return map[string]any{
		"name": "  Cliente   Exemplo\tLtda ",
		"addresses": []any{map[string]any{
			"street": "Rua A", "number": "", "neighborhood": "Centro", "city_ibge_code": "3550308",
			"city": "Sao Paulo", "postal_code": "01001-000", "state_federation": "SP",
		}},
		"contacts":            map[string]any{"phones": []any{"(11) 3000-0000"}},
		"state_registrations": []any{map[string]any{"uf": "SP", "state_registration": "110.042.490.114"}},
	}
}

func TestCustomerFromPersonMapsRegistry(t *testing.T) {
	c, err := customerFromPerson("CNPJ_33000167000101", fullPerson())
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Cliente Exemplo Ltda" || c.CNPJ == nil || *c.CNPJ != "33000167000101" || c.CPF != nil {
		t.Errorf("customer = %+v", c)
	}
	if c.IEIndicator != "1" || c.IE == nil || *c.IE != "110042490114" {
		t.Errorf("IE = %v %v", c.IEIndicator, c.IE)
	}
	a := c.Address
	if a.Number != "S/N" || a.PostalCode != "01001000" || a.UF != "SP" || a.Phone == nil || *a.Phone != "1130000000" {
		t.Errorf("address = %+v", a)
	}
}

func TestCustomerFromPersonIEIndicator(t *testing.T) {
	p := fullPerson()
	p["state_registrations"] = []any{map[string]any{"uf": "SP", "state_registration": "isento"}}
	c, err := customerFromPerson("CPF_52998224725", p)
	if err != nil || c.IEIndicator != "2" || c.IE == nil || *c.IE != "ISENTO" || c.CPF == nil {
		t.Errorf("isento: %+v %v", c, err)
	}
	delete(p, "state_registrations")
	if c, err = customerFromPerson("CPF_52998224725", p); err != nil || c.IEIndicator != "9" || c.IE != nil {
		t.Errorf("não contribuinte: %+v %v", c, err)
	}
}

func TestCustomerFromPersonRefusesIncompleteAddress(t *testing.T) {
	p := fullPerson()
	delete(p["addresses"].([]any)[0].(map[string]any), "city_ibge_code")
	if _, err := customerFromPerson("CNPJ_33000167000101", p); err == nil {
		t.Error("endereço sem município não vale")
	}
	p = fullPerson()
	p["addresses"].([]any)[0].(map[string]any)["state_federation"] = "EX"
	if _, err := customerFromPerson("CNPJ_33000167000101", p); err == nil {
		t.Error("a NFCom não aceita endereço no exterior")
	}
	if _, err := customerFromPerson("XYZ_1", fullPerson()); err == nil {
		t.Error("chave de pessoa desconhecida")
	}
}

func samplePlan() map[string]any {
	return map[string]any{
		"code": "PLANO100", "description": "Internet 100 Mbps", "class_code": "0100301", "cfop": "5307", "unit": "4",
		"unit_value": "99.90",
		"icms":       map[string]any{"cst": "00", "rate": "18"},
		"pis":        map[string]any{"cst": "01", "rate": "0.65"},
		"cofins":     map[string]any{"cst": "01", "rate": "3"},
	}
}

func TestItemFromPlanUsesPlanValueAndTaxes(t *testing.T) {
	it, err := itemFromPlan(samplePlan(), NfcomItemRequest{PlanID: "p", Quantity: "2", Discount: ptr("5.00")})
	if err != nil {
		t.Fatal(err)
	}
	if it.UnitValue != "99.90" || it.Quantity != "2" || it.ICMS.CST != "00" || *it.ICMS.Rate != "18" || it.PIS.Rate != "0.65" || it.COFINS.CST != "01" {
		t.Errorf("item = %+v", it)
	}
	if err := validateItem(1, it); err != nil {
		t.Errorf("item resolvido deveria validar: %v", err)
	}
}

func TestItemFromPlanUnitValueRules(t *testing.T) {
	if _, err := itemFromPlan(samplePlan(), NfcomItemRequest{Quantity: "1", UnitValue: ptr("1.00")}); err == nil {
		t.Error("plano com valor fixo recusa troca do valor")
	}
	open := samplePlan()
	delete(open, "unit_value")
	if _, err := itemFromPlan(open, NfcomItemRequest{Quantity: "1"}); err == nil {
		t.Error("plano sem valor exige o valor unitário")
	}
	it, err := itemFromPlan(open, NfcomItemRequest{Quantity: "1", UnitValue: ptr("10.00")})
	if err != nil || it.UnitValue != "10.00" {
		t.Errorf("valor informado: %+v %v", it, err)
	}
}

func TestValidatePlanAppliesItemRules(t *testing.T) {
	ok := NfcomItem{Code: "A", Description: "d", ClassCode: "0100301", CFOP: ptr("5307"), Unit: "4", UnitValue: "1", ICMS: NfcomICMS{CST: "00", Rate: ptr("18")}}
	if err := ValidatePlan(ok); err != nil {
		t.Fatalf("plano válido: %v", err)
	}
	noRate := ok
	noRate.ICMS.Rate = nil
	if err := ValidatePlan(noRate); err == nil || !strings.Contains(err.Error(), "alíquota") {
		t.Errorf("ICMS 00 sem alíquota: %v", err)
	}
	noCFOP := ok
	noCFOP.CFOP = nil
	if err := ValidatePlan(noCFOP); err == nil {
		t.Error("sem CFOP e com CST")
	}
}
