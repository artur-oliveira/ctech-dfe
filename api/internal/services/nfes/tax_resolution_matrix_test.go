package nfes

import "testing"

// TestResolveCfopTax_ResolutionTierMatrix cobre cada um dos 6 níveis de
// resolução isoladamente, com todos os níveis presentes ao mesmo tempo, para
// garantir que o nível certo vence mesmo quando os outros também cobririam o
// CFOP (regressão do "produto vence tudo" após a introdução de UF).
func TestResolveCfopTax_ResolutionTierMatrix(t *testing.T) {
	cases := []struct {
		name         string
		product      map[string]any
		profiles     map[string]map[string]any
		cfop, destUF string
		wantAliq     string
	}{
		{
			name: "tier1_product_cfop_uf_wins_over_everything",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{
					"tax_profile_id": "p1",
					"overrides":      map[string]any{"icms_aliq_override": "40.00"},
				}},
				"cfop_config": []any{map[string]any{
					"cfop": "5102", "icms_aliq_override": "10.00",
					"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "99.00"}}},
				}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop:     "5102", destUF: "RJ", wantAliq: "99.00",
		},
		{
			name: "tier2_product_cfop_no_uf_match",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{
					"tax_profile_id": "p1",
					"overrides":      map[string]any{"icms_aliq_override": "40.00"},
				}},
				"cfop_config": []any{map[string]any{
					"cfop": "5102", "icms_aliq_override": "10.00",
					"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "99.00"}}},
				}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop:     "5102", destUF: "SP", wantAliq: "10.00",
		},
		{
			name: "tier3_link_override_plus_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{
					"tax_profile_id": "p1",
					"overrides": map[string]any{
						"icms_aliq_override": "30.00",
						"uf_overrides":       []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "50.00"}}},
					},
				}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop:     "5102", destUF: "RJ", wantAliq: "50.00",
		},
		{
			name: "tier4_link_override_no_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{
					"tax_profile_id": "p1",
					"overrides": map[string]any{
						"icms_aliq_override": "30.00",
						"uf_overrides":       []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "50.00"}}},
					},
				}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop:     "5102", destUF: "SP", wantAliq: "30.00",
		},
		{
			name: "tier5_profile_plus_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
			},
			profiles: map[string]map[string]any{"p1": {
				"cfops": []any{"5102"}, "icms_aliq_override": "1.00",
				"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "22.00"}}},
			}},
			cfop: "5102", destUF: "RJ", wantAliq: "22.00",
		},
		{
			name: "tier6_profile_no_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
			},
			profiles: map[string]map[string]any{"p1": {
				"cfops": []any{"5102"}, "icms_aliq_override": "1.00",
				"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "22.00"}}},
			}},
			cfop: "5102", destUF: "SP", wantAliq: "1.00",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveCfopTax(tc.product, tc.profiles, tc.cfop, tc.destUF)
			if err != nil {
				t.Fatal(err)
			}
			if resolved["icms_aliq_override"] != tc.wantAliq {
				t.Errorf("want %s, got %v", tc.wantAliq, resolved["icms_aliq_override"])
			}
		})
	}
}

// TestResolveCfopTax_Tier7_ErrorWhenUncovered é o 7º "nível" — nenhuma camada
// cobre o CFOP.
func TestResolveCfopTax_Tier7_ErrorWhenUncovered(t *testing.T) {
	if _, err := resolveCfopTax(map[string]any{"cfop_config": []any{}}, nil, "9999", "SP"); err == nil {
		t.Fatal("expected error for uncovered CFOP")
	}
}

// TestDifal_UnaffectedByUfOverrides confirma que uf_overrides não interfere no
// cálculo automático de DIFAL: buildICMSUFDest (builders_doc.go) usa
// resolveICMSIntraAliq/resolveICMSInterAliq/resolveFCPAliq diretamente por UF
// — nenhuma dessas funções lê uf_overrides, que só existe dentro do map
// resolvido por resolveCfopTax (usado para o ICMS "normal" do item, não para
// o grupo ICMSUFDest).
func TestDifal_UnaffectedByUfOverrides(t *testing.T) {
	product := map[string]any{
		"cfop_config": []any{map[string]any{
			"cfop": "6108", "icms": "00", "icms_aliq_override": "12.00",
			"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "99.00"}}},
		}},
	}
	resolved, err := resolveCfopTax(product, nil, "6108", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "99.00" {
		t.Fatalf("uf_overrides should still apply to the item's own ICMS resolution, got %v", resolved["icms_aliq_override"])
	}
	// DIFAL usa uma tabela intra/inter separada, alheia a uf_overrides — as
	// funções abaixo não recebem nada do resultado de resolveCfopTax.
	if got := resolveICMSIntraAliq("RJ"); got == "" {
		t.Fatal("resolveICMSIntraAliq should be unaffected and still return a value")
	}
	if got := resolveICMSInterAliq("SP", "RJ", nil); got == "" {
		t.Fatal("resolveICMSInterAliq should be unaffected and still return a value")
	}
}
