package v1

import (
	"strings"
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

func TestPersonObjectBody_NfseGroup(t *testing.T) {
	base := func() PersonObjectBody {
		return PersonObjectBody{
			Addresses: []AddressBody{{
				CityIBGECode: "2211001", Street: "Rua A", Neighborhood: "Centro",
				Number: "10", City: "Teresina", StateFederation: "PI", PostalCode: "64000000",
			}},
		}
	}

	t.Run("grupo ausente é válido", func(t *testing.T) {
		if p := validation.Struct(base()); p != nil {
			t.Fatalf("person sem grupo nfse rejeitado: %+v", p)
		}
	})

	t.Run("grupo completo é válido", func(t *testing.T) {
		b := base()
		b.Nfse = &NfseInfoBody{
			IM:      new("987654"),
			RegTrib: &NfseRegTribBody{OpSimpNac: 3, RegApTribSN: ptrInt(1), RegEspTrib: 0},
		}
		if p := validation.Struct(b); p != nil {
			t.Fatalf("grupo nfse válido rejeitado: %+v", p)
		}
	})

	t.Run("op_simp_nac fora do domínio é rejeitado", func(t *testing.T) {
		b := base()
		b.Nfse = &NfseInfoBody{RegTrib: &NfseRegTribBody{OpSimpNac: 9, RegEspTrib: 0}}
		if p := validation.Struct(b); p == nil {
			t.Fatal("op_simp_nac=9 aceito, esperado erro")
		}
	})

	t.Run("im não numérica é rejeitada", func(t *testing.T) {
		b := base()
		b.Nfse = &NfseInfoBody{IM: new("ABC123")}
		if p := validation.Struct(b); p == nil {
			t.Fatal("im alfanumérica aceita, esperado erro")
		}
	})
}

//go:fix inline
func ptrInt(v int) *int { return new(v) }

func TestServiceBody_Validation(t *testing.T) {
	valid := ServiceBody{
		Code:             "SRV001",
		Description:      "Análise e desenvolvimento de sistemas",
		TribNacionalCode: "010101",
		Unit:             "UN",
		Value:            "1500.00",
		Iss:              ServiceIssBody{TribISSQN: 1, TaxRate: "5.00"},
	}
	if p := validation.Struct(valid); p != nil {
		t.Fatalf("ServiceBody válido rejeitado: %+v", p)
	}

	t.Run("trib_nacional_code inexistente é rejeitado", func(t *testing.T) {
		b := valid
		b.TribNacionalCode = "999999"
		if p := validation.Struct(b); p == nil {
			t.Fatal("código inexistente aceito")
		}
	})

	t.Run("descrição acima de 2000 chars é rejeitada", func(t *testing.T) {
		b := valid
		b.Description = strings.Repeat("x", 2001)
		if p := validation.Struct(b); p == nil {
			t.Fatal("descrição longa demais aceita")
		}
	})
}

func TestNfseConfigBody_Validation(t *testing.T) {
	valid := NfseConfigBody{
		Provider:    "nacional",
		Environment: 2,
		Timezone:    "America/Fortaleza",
		CLocEmi:     "2211001",
		Serie:       "00001",
	}
	if p := validation.Struct(valid); p != nil {
		t.Fatalf("NfseConfigBody válido rejeitado: %+v", p)
	}

	t.Run("provider desconhecido é rejeitado", func(t *testing.T) {
		b := valid
		b.Provider = "gissonline"
		if p := validation.Struct(b); p == nil {
			t.Fatal("provider desconhecido aceito")
		}
	})

	t.Run("c_loc_emi fora do formato IBGE é rejeitado", func(t *testing.T) {
		b := valid
		b.CLocEmi = "2211"
		if p := validation.Struct(b); p == nil {
			t.Fatal("código IBGE curto aceito")
		}
	})

	t.Run("timezone desconhecido é rejeitado", func(t *testing.T) {
		b := valid
		b.Timezone = "America/Invalid"
		if p := validation.Struct(b); p == nil {
			t.Fatal("timezone desconhecido aceito")
		}
	})
}
