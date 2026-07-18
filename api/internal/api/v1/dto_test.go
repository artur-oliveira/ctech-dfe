package v1

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/validation"
)

func validAddress() AddressBody {
	return AddressBody{
		CityIBGECode:    "3550308",
		Street:          "Av. Paulista",
		Neighborhood:    "Bela Vista",
		Number:          "1000",
		City:            "São Paulo",
		StateFederation: "SP",
		PostalCode:      "01310100",
	}
}

func validPerson() PersonObjectBody {
	return PersonObjectBody{
		Crt:       new(3),
		Addresses: []AddressBody{validAddress()},
		Contacts:  &ContactsBody{Emails: []string{"a@b.com"}, Phones: []string{"11999998888"}},
	}
}

func validCfopConfig() CfopConfigBody {
	return CfopConfigBody{
		Cfop:            "5102",
		Pis:             "01",
		Cofins:          "01",
		IbsCbsCst:       "000",
		IbsCbsClassTrib: "000001",
		IbsUfAliq:       "8.0000",
		IbsMunAliq:      "1.0000",
		CbsAliq:         "9.0000",
	}
}

func validProduct() ProductBody {
	return ProductBody{
		Code:        "PROD-01",
		Description: "Caneta azul",
		Ncm:         "96081000",
		Origin:      "0",
		Unit:        "UN",
		Value:       "9.90",
		IndTot:      "1",
		CfopNfce:    "5102",
		CfopConfig:  []CfopConfigBody{validCfopConfig()},
	}
}

func validVehicle() VehicleCreateBody {
	return VehicleCreateBody{
		Plate:   "ABC1D23",
		PlateUf: "SP",
		Role:    "tractor",
	}
}

func validVehicleWithAllFields() VehicleCreateBody {
	return VehicleCreateBody{
		Plate:    "ABC1D23",
		PlateUf:  "SP",
		Role:     "tractor",
		Wheelset: "02",
		Bodywork: "01",
		Renavam:  "123456789",
		Weight:   8000,
		CapKG:    12000,
		CapM3:    40,
		Cint:     "INT-01",
		Owner: &VehicleOwnerBody{
			CpfCnpj: "11222333000181",
			Rntrc:   "12345678",
			Name:    "Transportadora X",
			Type:    "ETC",
		},
	}
}

// TestValidDTOsPass ensures the happy-path payloads validate cleanly.
func TestValidDTOsPass(t *testing.T) {
	cases := map[string]any{
		"person":             PersonCreateBody{CpfOrCnpj: "52998224725", Name: "Fulano", Person: validPerson()},
		"org":                OrganizationCreateBody{CpfOrCnpj: "11222333000181", Name: "Empresa", Person: validPerson()},
		"product":            validProduct(),
		"vehicle":            validVehicle(),
		"vehicle-all-fields": validVehicleWithAllFields(),
		"nfe-config":         FiscalConfigBody{fiscalConfigBase{Timezone: "America/Sao_Paulo", Environment: 2}},
		"nfce-config":        NfceConfigBody{fiscalConfigBase: fiscalConfigBase{Timezone: "America/Sao_Paulo", Environment: 2}, ProdCsc: "CSC", ProdCscID: 1, HomCsc: "CSC", HomCscID: 1},
	}
	for name, dto := range cases {
		if p := validation.Struct(dto); p != nil {
			t.Errorf("%s: expected valid, got errors %+v", name, p.Errors)
		}
	}
}

// TestInvalidDTOsFail checks that bad inputs surface field-level errors.
func TestInvalidDTOsFail(t *testing.T) {
	// Bad CPF + a present-but-incomplete person (no addresses).
	bad := PersonCreateBody{CpfOrCnpj: "00000000000", Name: "X", Person: PersonObjectBody{Crt: new(3)}}
	p := validation.Struct(bad)
	if p == nil || p.Status != 422 {
		t.Fatalf("expected 422 validation problem, got %+v", p)
	}
	fields := map[string]bool{}
	for _, fe := range p.Errors {
		fields[fe.Field] = true
	}
	if !fields["cpf_or_cnpj"] {
		t.Errorf("expected cpf_or_cnpj error, got %+v", fields)
	}
	if !fields["name"] {
		t.Errorf("expected name (min) error, got %+v", fields)
	}
	if !fields["person.addresses"] {
		t.Errorf("expected person.addresses error, got %+v", fields)
	}
}

// TestProductNestedFieldPath verifies array index paths in nested cfop_config.
func TestProductNestedFieldPath(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].Cfop = "51"      // invalid CFOP
	prod.CfopConfig[0].IbsCbsCst = "99" // invalid CST
	p := validation.Struct(prod)
	if p == nil {
		t.Fatal("expected errors")
	}
	want := map[string]bool{"cfop_config[0].cfop": false, "cfop_config[0].ibs_cbs_cst": false}
	for _, fe := range p.Errors {
		if _, ok := want[fe.Field]; ok {
			want[fe.Field] = true
		}
	}
	for field, found := range want {
		if !found {
			t.Errorf("expected error at %q, got %+v", field, p.Errors)
		}
	}
}
