package nfes

import (
	"reflect"
	"strings"
	"testing"
)

// legacyProduct é um produto como todos os que já existem hoje: tributação
// inteira em cfop_config[], nenhum perfil.
func legacyProduct() map[string]any {
	return map[string]any{
		"code": "PROD-01",
		"cfop_config": []any{
			map[string]any{
				"cfop": "5102", "pis": "01", "cofins": "01",
				"ibs_cbs_cst": "000", "cbs_aliq": "9.0000",
			},
			map[string]any{
				"cfop": "6102", "pis": "01", "cofins": "01",
				"ibs_cbs_cst": "000", "cbs_aliq": "9.0000",
			},
		},
	}
}

func profile(id string, cfops []string, fields map[string]any) map[string]any {
	list := make([]any, len(cfops))
	for i, c := range cfops {
		list[i] = c
	}
	m := map[string]any{
		"pk": "CNPJ_1", "sk": id, "name": "Perfil", "description": "…",
		"cfops": list, "created_at": "2026-01-01", "updated_at": "2026-01-01",
	}
	for k, v := range fields {
		m[k] = v
	}
	return m
}

// (a) Produto legado, sem perfil: o resultado tem que ser exatamente a entrada
// de cfop_config que já era usada — é o que garante zero regressão.
func TestResolveCfopTax_LegacyProductUnchanged(t *testing.T) {
	product := legacyProduct()
	got, err := resolveCfopTax(product, nil, "5102", "SP")
	if err != nil {
		t.Fatalf("resolveCfopTax: %v", err)
	}
	want := map[string]any{
		"cfop": "5102", "pis": "01", "cofins": "01",
		"ibs_cbs_cst": "000", "cbs_aliq": "9.0000",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// (b) Perfil puro: o produto não tem cfop_config para o CFOP, então tudo vem do
// perfil — e nenhum campo de cadastro do perfil vaza para o item.
func TestResolveCfopTax_ProfileOnly(t *testing.T) {
	product := map[string]any{
		"code":        "PROD-02",
		"cfop_config": []any{},
		"tax_profiles": []any{
			map[string]any{"tax_profile_id": "TAXPROFILE_1"},
		},
	}
	profiles := map[string]map[string]any{
		"TAXPROFILE_1": profile("TAXPROFILE_1", []string{"5102", "6102"}, map[string]any{
			"pis": "01", "cofins": "01", "ibs_cbs_cst": "000", "cbs_aliq": "9.0000",
		}),
	}

	got, err := resolveCfopTax(product, profiles, "6102", "SP")
	if err != nil {
		t.Fatalf("resolveCfopTax: %v", err)
	}
	want := map[string]any{
		"cfop": "6102", "pis": "01", "cofins": "01",
		"ibs_cbs_cst": "000", "cbs_aliq": "9.0000",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v — campos de cadastro do perfil não podem vazar", got, want)
	}
}

// (c) Override do produto vence o perfil, e só nas chaves que nomeia.
func TestResolveCfopTax_ProductOverrideBeatsProfile(t *testing.T) {
	product := map[string]any{
		"code":        "PROD-03",
		"cfop_config": []any{},
		"tax_profiles": []any{
			map[string]any{
				"tax_profile_id": "TAXPROFILE_1",
				"overrides":      map[string]any{"cbs_aliq": "5.0000"},
			},
		},
	}
	profiles := map[string]map[string]any{
		"TAXPROFILE_1": profile("TAXPROFILE_1", []string{"5102"}, map[string]any{
			"pis": "01", "cofins": "01", "ibs_cbs_cst": "000", "cbs_aliq": "9.0000",
		}),
	}

	got, err := resolveCfopTax(product, profiles, "5102", "SP")
	if err != nil {
		t.Fatalf("resolveCfopTax: %v", err)
	}
	if got["cbs_aliq"] != "5.0000" {
		t.Errorf("cbs_aliq = %v, want 5.0000 (override do produto vence)", got["cbs_aliq"])
	}
	if got["pis"] != "01" {
		t.Errorf("pis = %v, want 01 — override parcial não pode zerar o resto", got["pis"])
	}
}

// (d) cfop_config explícito no produto vence tudo, inclusive o override.
func TestResolveCfopTax_CfopConfigBeatsEverything(t *testing.T) {
	product := map[string]any{
		"code": "PROD-04",
		"cfop_config": []any{
			map[string]any{"cfop": "5102", "cbs_aliq": "1.0000"},
		},
		"tax_profiles": []any{
			map[string]any{
				"tax_profile_id": "TAXPROFILE_1",
				"overrides":      map[string]any{"cbs_aliq": "5.0000"},
			},
		},
	}
	profiles := map[string]map[string]any{
		"TAXPROFILE_1": profile("TAXPROFILE_1", []string{"5102"}, map[string]any{
			"pis": "01", "cofins": "01", "cbs_aliq": "9.0000",
		}),
	}

	got, err := resolveCfopTax(product, profiles, "5102", "SP")
	if err != nil {
		t.Fatalf("resolveCfopTax: %v", err)
	}
	if got["cbs_aliq"] != "1.0000" {
		t.Errorf("cbs_aliq = %v, want 1.0000 (cfop_config vence)", got["cbs_aliq"])
	}
	// O que o cfop_config não diz continua vindo do perfil.
	if got["pis"] != "01" {
		t.Errorf("pis = %v, want 01 — cfop_config parcial completa pelo perfil", got["pis"])
	}
}

// Só o perfil que cobre o CFOP pedido é aplicado.
func TestResolveCfopTax_PicksTheProfileCoveringTheCFOP(t *testing.T) {
	product := map[string]any{
		"code":        "PROD-05",
		"cfop_config": []any{},
		"tax_profiles": []any{
			map[string]any{"tax_profile_id": "TAXPROFILE_VENDA"},
			map[string]any{"tax_profile_id": "TAXPROFILE_DEVOLUCAO"},
		},
	}
	profiles := map[string]map[string]any{
		"TAXPROFILE_VENDA":     profile("TAXPROFILE_VENDA", []string{"5102"}, map[string]any{"pis": "01"}),
		"TAXPROFILE_DEVOLUCAO": profile("TAXPROFILE_DEVOLUCAO", []string{"1202"}, map[string]any{"pis": "49"}),
	}

	got, err := resolveCfopTax(product, profiles, "1202", "SP")
	if err != nil {
		t.Fatalf("resolveCfopTax: %v", err)
	}
	if got["pis"] != "49" {
		t.Errorf("pis = %v, want 49 — o perfil aplicado tem de ser o que cobre o CFOP", got["pis"])
	}
}

func TestResolveCfopTax_UnconfiguredCFOPIsAnError(t *testing.T) {
	if _, err := resolveCfopTax(legacyProduct(), nil, "5405", "SP"); err == nil {
		t.Fatal("esperado erro para CFOP sem tributação em lugar nenhum")
	}
}

// A validação de CFOP passa a considerar a união de cfop_config com os CFOPs
// dos perfis — antes só olhava cfop_config.
func TestProductCFOPs_UnionOfConfigAndProfiles(t *testing.T) {
	product := legacyProduct()
	product["tax_profiles"] = []any{map[string]any{"tax_profile_id": "TAXPROFILE_1"}}
	profiles := map[string]map[string]any{
		"TAXPROFILE_1": profile("TAXPROFILE_1", []string{"1202", "5102"}, nil),
	}

	got := productCFOPs(product, profiles)
	want := []string{"1202", "5102", "6102"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("productCFOPs = %v, want %v", got, want)
	}
}

func productWithUfOverride() map[string]any {
	return map[string]any{
		"cfop_config": []any{
			map[string]any{
				"cfop": "5102", "icms": "00",
				"icms_aliq_override": "18.00",
				"uf_overrides": []any{
					map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "20.00"}},
				},
			},
		},
	}
}

func TestResolveCfopTax_ProductUfOverride_BeatsProductBase(t *testing.T) {
	resolved, err := resolveCfopTax(productWithUfOverride(), nil, "5102", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "20.00" {
		t.Errorf("expected RJ override to win, got %v", resolved["icms_aliq_override"])
	}
}

func TestResolveCfopTax_ProductUfOverride_FallsBackForOtherUf(t *testing.T) {
	resolved, err := resolveCfopTax(productWithUfOverride(), nil, "5102", "SP")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "18.00" {
		t.Errorf("expected product base (no UF match), got %v", resolved["icms_aliq_override"])
	}
	if _, ok := resolved["uf_overrides"]; ok {
		t.Errorf("uf_overrides metadata must not leak into the resolved tax map, got %v", resolved)
	}
}

func TestResolveCfopTax_ProfileUfOverride_LowerThanProductLevel(t *testing.T) {
	product := map[string]any{
		"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
	}
	profiles := map[string]map[string]any{
		"p1": {
			"cfops": []any{"5102"}, "icms_aliq_override": "12.00",
			"uf_overrides": []any{
				map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "22.00"}},
			},
		},
	}
	resolved, err := resolveCfopTax(product, profiles, "5102", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "22.00" {
		t.Errorf("expected profile+UF override, got %v", resolved["icms_aliq_override"])
	}
}

func TestResolveCfopTax_LinkOverride_UfBeatsLinkOverrideBase(t *testing.T) {
	product := map[string]any{
		"tax_profiles": []any{map[string]any{
			"tax_profile_id": "p1",
			"overrides": map[string]any{
				"icms_aliq_override": "30.00",
				"uf_overrides": []any{
					map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "50.00"}},
				},
			},
		}},
	}
	profiles := map[string]map[string]any{
		"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"},
	}
	resolved, err := resolveCfopTax(product, profiles, "5102", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "50.00" {
		t.Errorf("expected link-override+UF to win, got %v", resolved["icms_aliq_override"])
	}
	resolvedSP, err := resolveCfopTax(product, profiles, "5102", "SP")
	if err != nil {
		t.Fatal(err)
	}
	if resolvedSP["icms_aliq_override"] != "30.00" {
		t.Errorf("expected link-override base (no UF match), got %v", resolvedSP["icms_aliq_override"])
	}
}

func TestResolveCfopTax_NoLayerCovers_ReturnsError(t *testing.T) {
	if _, err := resolveCfopTax(map[string]any{}, nil, "5102", "SP"); err == nil {
		t.Fatal("expected error when no layer covers the CFOP")
	}
}

// A mensagem de erro tem que dizer onde configurar, não só que faltou.
func TestCfopNotConfiguredError_SaysWhereToConfigure(t *testing.T) {
	msg := cfopNotConfiguredError("5405", "PROD-01", []string{"5102", "6102"})
	for _, want := range []string{"5405", "PROD-01", "5102", "perfil fiscal"} {
		if !strings.Contains(msg, want) {
			t.Errorf("mensagem %q não menciona %q", msg, want)
		}
	}
}
