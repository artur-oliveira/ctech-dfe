package services

import "testing"

func TestUnwrapResponseNode_Default(t *testing.T) {
	raw := map[string]any{
		"nfeResultMsg": map[string]any{"cStat": "107", "xMotivo": "Servico em Operacao"},
	}
	result, err := unwrapResponseNode("nfe", "SP", "NfeStatusServico", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["cStat"] != "107" {
		t.Errorf("result = %+v", result)
	}
}

// TestUnwrapResponseNode_ANDistribution is the regression test for the bug
// this file fixes: NFeDistribuicaoDFe via the AN authorizer must unwrap
// TWO levels (nfeDistDFeInteresseResponse -> nfeDistDFeInteresseResult), an
// override that entirely REPLACES the default single-level "nfeResultMsg"
// unwrap — not layer on top of it.
func TestUnwrapResponseNode_ANDistribution(t *testing.T) {
	raw := map[string]any{
		"nfeDistDFeInteresseResponse": map[string]any{
			"nfeDistDFeInteresseResult": map[string]any{
				"retDistDFeInt": map[string]any{"cStat": "138"},
			},
		},
	}
	result, err := unwrapResponseNode("nfe", "AN", "NFeDistribuicaoDFe", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	retDistDFeInt, ok := result["retDistDFeInt"].(map[string]any)
	if !ok || retDistDFeInt["cStat"] != "138" {
		t.Errorf("result = %+v, want retDistDFeInt.cStat=138", result)
	}
}

func TestUnwrapResponseNode_MTTwoLevelOverride(t *testing.T) {
	raw := map[string]any{
		"nfeResultMsg": map[string]any{
			"consultaCadastroResult": map[string]any{"cStat": "111"},
		},
	}
	result, err := unwrapResponseNode("nfe", "MT", "NfeConsultaCadastro", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["cStat"] != "111" {
		t.Errorf("result = %+v", result)
	}
}

func TestUnwrapResponseNode_CTeANDistribution(t *testing.T) {
	raw := map[string]any{
		"cteDistDFeInteresseResponse": map[string]any{
			"cteDistDFeInteresseResult": map[string]any{"cStat": "138"},
		},
	}
	result, err := unwrapResponseNode("cte", "AN", "CTeDistribuicaoDFe", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["cStat"] != "138" {
		t.Errorf("result = %+v", result)
	}
}

// TestUnwrapResponseNode_MDFeSVRSDistribution: MDF-e has no per-UF direct
// authorizer (every UF -> SVRS), so a real-UF caller (not "AN") must still
// resolve through endpoints.Authorizer to "SVRS" and hit the SVRS override
// — this is what worker's distribution.go actually does (mdfe's
// docTypeConfig.uf is "", falling back to the org's real UF).
func TestUnwrapResponseNode_MDFeSVRSDistribution(t *testing.T) {
	// The SVRS override path is a single element that REPLACES the default
	// entirely (like the AN overrides above) — MDFe's distribution response
	// has no "mdfeResultMsg" wrapper to unwrap first.
	raw := map[string]any{
		"mdfeDistDFeInteresseResult": map[string]any{"cStat": "138"},
	}
	result, err := unwrapResponseNode("mdfe", "SP", "MDFeDistribuicaoDFe", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["cStat"] != "138" {
		t.Errorf("result = %+v", result)
	}
}

func TestUnwrapResponseNode_MissingNodeErrors(t *testing.T) {
	raw := map[string]any{"nfeResultMsg": map[string]any{"somethingElse": "x"}}
	if _, err := unwrapResponseNode("nfe", "AN", "NFeDistribuicaoDFe", raw); err == nil {
		t.Error("expected error when the expected override node is missing, got nil")
	}
}

func TestEnsureList(t *testing.T) {
	d := map[string]any{
		"retEnviNFe": map[string]any{
			"protNFe": map[string]any{"infProt": map[string]any{"cStat": "100"}},
		},
	}
	ensureList(d, "retEnviNFe/protNFe")

	retEnviNFe := d["retEnviNFe"].(map[string]any)
	list, ok := retEnviNFe["protNFe"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("protNFe = %#v, want a 1-element list", retEnviNFe["protNFe"])
	}

	// Already a list: left untouched.
	d2 := map[string]any{"a": map[string]any{"b": []any{map[string]any{"x": 1}, map[string]any{"x": 2}}}}
	ensureList(d2, "a/b")
	if list2, ok := d2["a"].(map[string]any)["b"].([]any); !ok || len(list2) != 2 {
		t.Errorf("existing list should be left alone: %#v", d2)
	}

	// Missing path: no panic, no-op.
	d3 := map[string]any{"a": map[string]any{}}
	ensureList(d3, "a/b/c")
}
