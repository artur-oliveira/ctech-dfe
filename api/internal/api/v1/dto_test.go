package v1

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
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
		Cfop:          "5102",
		TaxFieldsBody: validTaxFields(),
	}
}

func validTaxFields() TaxFieldsBody {
	return TaxFieldsBody{
		Pis:             "01",
		Cofins:          "01",
		IbsCbsCst:       new("000"),
		IbsCbsClassTrib: new("000001"),
		IbsUfAliq:       new("8.0000"),
		IbsMunAliq:      new("1.0000"),
		CbsAliq:         new("9.0000"),
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
	prod.CfopConfig[0].IbsCbsCst = new("99") // invalid CST
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

// TestProductCfopConfig_OptionalWhenTaxProfileLinked reproduces a bug where a
// product covered entirely by tax_profiles could not be saved with an empty
// cfop_config: the DTO required min=1 unconditionally.
func TestProductCfopConfig_OptionalWhenTaxProfileLinked(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig = nil
	prod.TaxProfiles = []ProductTaxProfileRef{{TaxProfileID: "profile-1"}}
	if p := validation.Struct(prod); p != nil {
		t.Errorf("expected valid (cfop_config covered by tax_profiles), got errors %+v", p.Errors)
	}
}

// TestProductRequiresCfopConfigOrTaxProfile ensures a product can't end up
// with neither tax source configured.
func TestProductRequiresCfopConfigOrTaxProfile(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig = nil
	p := validation.Struct(prod)
	if p == nil {
		t.Fatal("expected error when both cfop_config and tax_profiles are empty")
	}
	fields := map[string]bool{}
	for _, fe := range p.Errors {
		fields[fe.Field] = true
	}
	if !fields["cfop_config"] || !fields["tax_profiles"] {
		t.Errorf("expected cfop_config and tax_profiles errors, got %+v", fields)
	}
}

// TestCfopConfig_UfOverrides_RequiresUfsWhenPresent ensures a uf_overrides
// entry that applies to no UF at all is rejected — an override that never
// matches any destination is dead configuration, not a "general override" in
// disguise.
func TestCfopConfig_UfOverrides_RequiresUfsWhenPresent(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].UfOverrides = []UfTaxOverride{{Ufs: nil, Overrides: map[string]any{"icms_aliq_override": "12.00"}}}
	p := validation.Struct(prod)
	if p == nil {
		t.Fatal("expected error when uf_overrides[].ufs is empty")
	}
}

func TestCfopConfig_UfOverrides_ValidUf(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].UfOverrides = []UfTaxOverride{{Ufs: []string{"SP", "RJ"}, Overrides: map[string]any{"icms_aliq_override": "12.00"}}}
	if p := validation.Struct(prod); p != nil {
		t.Errorf("expected valid, got %+v", p.Errors)
	}
}

// TestTaxFieldsBody_IbsCbs_OptionalWhenAllEmpty: a product with none of the
// IBS/CBS fields is valid now — the group is simply omitted at emission.
func TestTaxFieldsBody_IbsCbs_OptionalWhenAllEmpty(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].IbsCbsCst = nil
	prod.CfopConfig[0].IbsCbsClassTrib = nil
	prod.CfopConfig[0].IbsUfAliq = nil
	prod.CfopConfig[0].IbsMunAliq = nil
	prod.CfopConfig[0].CbsAliq = nil
	if p := validation.Struct(prod); p != nil {
		t.Errorf("expected valid with IBS/CBS group fully absent, got %+v", p.Errors)
	}
}

func TestTaxFieldsBody_IbsCbs_PartialIsError(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].IbsCbsClassTrib = nil
	prod.CfopConfig[0].IbsUfAliq = nil
	prod.CfopConfig[0].IbsMunAliq = nil
	prod.CfopConfig[0].CbsAliq = nil
	p := validation.Struct(prod)
	if p == nil {
		t.Fatal("expected error: ibs_cbs group is all-or-nothing")
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
	zero := 0
	cIndOp, cst, cClassTrib := "100301", "000", "000001"
	valid := ServiceBody{
		Code:             "SRV001",
		Description:      "Análise e desenvolvimento de sistemas",
		TribNacionalCode: "010101",
		Unit:             "UN",
		Value:            "1500.00",
		Iss:              ServiceIssBody{TribISSQN: 1, TaxRate: "5.00"},
		IbsCbs: &ServiceIbsCbsBody{
			CIndOp: &cIndOp, Cst: &cst, CClassTrib: &cClassTrib,
			IndDest: &zero, FinNFSe: &zero,
		},
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

	t.Run("IBS/CBS ausente é rejeitado", func(t *testing.T) {
		b := valid
		b.IbsCbs = nil
		if p := validation.Struct(b); p == nil {
			t.Fatal("serviço sem IBS/CBS aceito")
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

// ── Person roles ─────────────────────────────────────────────────────────────

func validPersonCreate() PersonCreateBody {
	return PersonCreateBody{
		CpfOrCnpj: "11222333000181",
		Name:      "Transportes Acme",
		Person:    validPerson(),
	}
}

func TestPersonRoles_Accepted(t *testing.T) {
	cases := map[string][]string{
		"absent":     nil,
		"empty":      {},
		"single":     {services.RoleCarrier},
		"multi":      {services.RoleCustomer, services.RoleCarrier},
		"every role": services.AllPersonRoles,
	}
	for name, roles := range cases {
		t.Run(name, func(t *testing.T) {
			dto := validPersonCreate()
			dto.Roles = roles
			if p := validation.Struct(dto); p != nil {
				t.Fatalf("expected valid, got %v", p)
			}
		})
	}
}

func TestPersonRoles_RejectsUnknownRole(t *testing.T) {
	dto := validPersonCreate()
	dto.Roles = []string{services.RoleCustomer, "shareholder"}
	if p := validation.Struct(dto); p == nil {
		t.Fatal("expected unknown role to be rejected")
	}
}

func TestPersonUpdateRoles_RejectsUnknownRole(t *testing.T) {
	dto := PersonUpdateBody{Roles: &[]string{"shareholder"}}
	if p := validation.Struct(dto); p == nil {
		t.Fatal("expected unknown role to be rejected")
	}
}

// Corpo sem `roles` não pode virar `"roles": null` no mapa de update: null vira
// REMOVE, e a pessoa perderia os papéis numa edição que nem falou deles.
func TestPersonUpdateRoles_AbsentIsNotAnUpdate(t *testing.T) {
	m, err := structToMap(PersonUpdateBody{Name: new("Fulano")})
	if err != nil {
		t.Fatalf("structToMap: %v", err)
	}
	if _, ok := m["roles"]; ok {
		t.Fatalf("roles presente num update que não o enviou: %v", m["roles"])
	}

	m, err = structToMap(PersonUpdateBody{Roles: &[]string{}})
	if err != nil {
		t.Fatalf("structToMap: %v", err)
	}
	roles, ok := m["roles"].([]any)
	if !ok || len(roles) != 0 {
		t.Fatalf("roles = %v, esperada lista vazia (limpar papéis)", m["roles"])
	}
}

// The `oneof=` list is baked into a struct tag (tags must be literals), so it
// cannot reference services.AllPersonRoles directly. This asserts the two never
// drift apart.
func TestPersonRolesTagMatchesAllPersonRoles(t *testing.T) {
	want := "omitempty,dive,oneof=" + strings.Join(services.AllPersonRoles, " ")
	if personRolesValidation != want {
		t.Fatalf("personRolesValidation = %q, want %q", personRolesValidation, want)
	}
	for _, f := range []string{
		reflect.TypeOf(PersonCreateBody{}).Field(2).Tag.Get("validate"),
		reflect.TypeOf(PersonUpdateBody{}).Field(1).Tag.Get("validate"),
	} {
		if f != personRolesValidation {
			t.Errorf("struct tag = %q, want %q", f, personRolesValidation)
		}
	}
}

// ── Tax profiles ─────────────────────────────────────────────────────────────

func validTaxProfile() TaxProfileBody {
	return TaxProfileBody{
		Name:          "Venda de mercadoria — Simples Nacional",
		Cfops:         []string{"5102", "6102"},
		TaxFieldsBody: validTaxFields(),
	}
}

func TestTaxProfile_Valid(t *testing.T) {
	if p := validation.Struct(validTaxProfile()); p != nil {
		t.Fatalf("expected valid tax profile, got %v", p)
	}
}

func TestTaxProfile_RequiresAtLeastOneCfop(t *testing.T) {
	dto := validTaxProfile()
	dto.Cfops = nil
	if p := validation.Struct(dto); p == nil {
		t.Fatal("expected a profile with no CFOP to be rejected")
	}
}

func TestTaxProfile_RejectsMalformedCfop(t *testing.T) {
	dto := validTaxProfile()
	dto.Cfops = []string{"5102", "51"}
	if p := validation.Struct(dto); p == nil {
		t.Fatal("expected a malformed CFOP to be rejected")
	}
}

// The IBS/CBS block on a profile follows the same all-or-nothing rule as a
// product's cfop_config (validateIbsCbsGroup) — filling only some of the 5
// key fields is rejected, but leaving the whole group empty is valid.
func TestTaxProfile_IbsCbsBlock_PartialIsRejected(t *testing.T) {
	dto := validTaxProfile()
	dto.IbsCbsCst = nil
	p := validation.Struct(dto)
	if p == nil {
		t.Fatal("expected partial ibs_cbs group to be rejected")
	}
	if len(p.Errors) == 0 || p.Errors[0].Field != "ibs_cbs_cst" {
		t.Errorf("error path = %+v, want ibs_cbs_cst (embedded struct must be inlined)", p.Errors)
	}
}

// ── Naturezas de operação ────────────────────────────────────────────────────

func validOperation() OperationBody {
	natOp := "Venda de mercadoria"
	suffix := "102"
	return OperationBody{
		Name:       "Venda para revenda",
		DocTypes:   []string{"nfe", "nfce"},
		NatOp:      &natOp,
		CfopSuffix: &suffix,
	}
}

func TestOperation_Valid(t *testing.T) {
	if p := validation.Struct(validOperation()); p != nil {
		t.Fatalf("esperado válido, obtido %v", p)
	}
}

func TestOperation_RejectsUnknownDocType(t *testing.T) {
	dto := validOperation()
	dto.DocTypes = []string{"nfe", "nfsE"}
	if p := validation.Struct(dto); p == nil {
		t.Fatal("esperada recusa de doc_type desconhecido")
	}
}

// O sufixo é a natureza fiscal: 3 dígitos, sem o escopo — o escopo é resolvido
// na emissão a partir das UFs.
func TestOperation_CfopSuffixIsThreeDigits(t *testing.T) {
	for _, bad := range []string{"5102", "10", "10a"} {
		dto := validOperation()
		dto.CfopSuffix = &bad
		if p := validation.Struct(dto); p == nil {
			t.Errorf("esperada recusa de cfop_suffix %q", bad)
		}
	}
}

func TestOperation_MinimalIsValid(t *testing.T) {
	if p := validation.Struct(OperationBody{Name: "Só o nome"}); p != nil {
		t.Fatalf("uma operação só com nome tem que ser válida: %v", p)
	}
}
