package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func minimalEnviNFeArgs() (org, receiver map[string]any, productItems, payments []map[string]any) {
	org = map[string]any{
		"name": "Emit Ltda",
		"person": map[string]any{
			"crt": float64(1),
			"addresses": []any{
				map[string]any{"street": "Rua Emit", "city": "Sao Paulo", "state_federation": "SP", "city_ibge_code": "3550308"},
			},
		},
	}
	receiver = map[string]any{
		"name": "Dest Ltda",
		"person": map[string]any{
			"addresses": []any{
				map[string]any{"street": "Rua Dest", "city": "Sao Paulo", "state_federation": "SP", "city_ibge_code": "3550308"},
			},
		},
	}
	return org, receiver, nil, nil
}

func TestBuildEnviNFe_EntregaPresent(t *testing.T) {
	org, receiver, productItems, payments := minimalEnviNFeArgs()
	result := BuildEnviNFe(
		org, receiver, "CNPJ_11222333000181",
		productItems, payments,
		1, 1, 2,
		"35260711222333000181550010000000011000000010", decimal.Zero, decimal.Zero,
		nil, time.Now(),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{}, nfModel55, nil,
		nil, &NfeLocalBody{XLgr: "Rua Entrega", Nro: "1", XBairro: "B", CMun: "3550308", XMun: "SP", UF: "SP"},
	)
	infNFe := result["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)
	entrega, ok := infNFe["entrega"].(map[string]any)
	if !ok {
		t.Fatalf("expected entrega key in infNFe, got %+v", infNFe)
	}
	if entrega["xLgr"] != "Rua Entrega" {
		t.Errorf("entrega.xLgr = %v, want Rua Entrega", entrega["xLgr"])
	}
	if _, hasRetirada := infNFe["retirada"]; hasRetirada {
		t.Error("expected no retirada key when retirada arg is nil")
	}
}

func TestBuildEnviNFe_AutXMLPresentWhenOrgHasAuthorizedViewers(t *testing.T) {
	org, receiver, productItems, payments := minimalEnviNFeArgs()
	org["authorized_xml_viewers"] = []any{
		map[string]any{"cpf_cnpj": "11122233344", "name": "Contador"},
	}
	result := BuildEnviNFe(
		org, receiver, "CNPJ_11222333000181",
		productItems, payments,
		1, 1, 2,
		"35260711222333000181550010000000011000000010", decimal.Zero, decimal.Zero,
		nil, time.Now(),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{}, nfModel55, nil,
		nil, nil,
	)
	infNFe := result["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)
	autXML, ok := infNFe["autXML"].([]map[string]any)
	if !ok || len(autXML) != 1 {
		t.Fatalf("expected autXML with 1 entry, got %+v", infNFe["autXML"])
	}
	if autXML[0]["CPF"] != "11122233344" {
		t.Errorf("autXML[0] = %+v, want CPF 11122233344", autXML[0])
	}
}

func TestBuildEnviNFe_NoAutXMLWhenOrgHasNoAuthorizedViewers(t *testing.T) {
	org, receiver, productItems, payments := minimalEnviNFeArgs()
	result := BuildEnviNFe(
		org, receiver, "CNPJ_11222333000181",
		productItems, payments,
		1, 1, 2,
		"35260711222333000181550010000000011000000010", decimal.Zero, decimal.Zero,
		nil, time.Now(),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{}, nfModel55, nil,
		nil, nil,
	)
	infNFe := result["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)
	if _, hasAutXML := infNFe["autXML"]; hasAutXML {
		t.Error("expected no autXML key when org has no authorized viewers")
	}
}

func TestBuildLocal_FullFields(t *testing.T) {
	cnpj := "11222333000181"
	xNome := "Depósito Sul"
	fone := "11988887777"
	email := "deposito@example.com"
	xCpl := "Galpão 3"
	l := &NfeLocalBody{
		CNPJ: &cnpj, XNome: &xNome, Fone: &fone, Email: &email, XCpl: &xCpl,
		XLgr: "Rua das Flores", Nro: "100", XBairro: "Centro",
		CMun: "3550308", XMun: "São Paulo", UF: "SP",
	}
	got := buildLocal(l)
	if got["CNPJ"] != cnpj || got["xNome"] != xNome || got["xLgr"] != "Rua das Flores" {
		t.Fatalf("unexpected build: %+v", got)
	}
	if _, hasCEP := got["CEP"]; hasCEP {
		t.Fatal("TLocal must not include CEP — that's a TEndereco-only field")
	}
	if got["cPais"] != cPaisBrasil || got["xPais"] != xPaisBrasil {
		t.Fatalf("expected default Brazil country, got %+v", got)
	}
}

func TestBuildAutXML_CNPJRoutedCorrectly(t *testing.T) {
	org := map[string]any{
		"authorized_xml_viewers": []any{
			map[string]any{"cpf_cnpj": "11222333000181", "name": "Contador"},
			map[string]any{"cpf_cnpj": "11122233344", "name": "Auditor"},
		},
	}
	got := buildAutXML(org)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0]["CNPJ"] != "11222333000181" {
		t.Fatalf("expected CNPJ routing for 14-digit doc: %+v", got[0])
	}
	if got[1]["CPF"] != "11122233344" {
		t.Fatalf("expected CPF routing for 11-digit doc: %+v", got[1])
	}
}

func TestBuildAutXML_EmptyReturnsNil(t *testing.T) {
	if buildAutXML(map[string]any{}) != nil {
		t.Fatal("expected nil for organization with no authorized viewers")
	}
}

func TestBuildLocal_NilReturnsNil(t *testing.T) {
	if buildLocal(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestBuildLocal_OmitsEmptyOptionalFields(t *testing.T) {
	l := &NfeLocalBody{XLgr: "Rua X", Nro: "1", XBairro: "B", CMun: "3550308", XMun: "SP", UF: "SP"}
	got := buildLocal(l)
	for _, k := range []string{"CNPJ", "CPF", "xNome", "xCpl", "fone", "email"} {
		if _, ok := got[k]; ok {
			t.Fatalf("expected %s omitted when empty, got %+v", k, got)
		}
	}
}
