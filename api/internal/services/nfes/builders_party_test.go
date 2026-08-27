package nfes

import "testing"

func TestBuildEmitIncluiIESTIMCNAE(t *testing.T) {
	org := map[string]any{"name": "ACME", "person": map[string]any{
		"crt":       3,
		"cnae":      "4712100",
		"isuf_emit": "123456789",
		"nfse":      map[string]any{"im": "998877"},
		"state_registrations": []any{
			map[string]any{"uf": "PI", "state_registration": "194000000", "ie_st": "070000000"},
		},
		"addresses": []any{map[string]any{"state_federation": "PI", "city": "Teresina"}},
	}}
	got := buildEmit(org, getPersonMap(org), "CNPJ_11647612000197", "PI", "PI", 3)
	for k, want := range map[string]string{"IEST": "070000000", "IM": "998877", "CNAE": "4712100", "ISUFEmit": "123456789"} {
		if got[k] != want {
			t.Fatalf("%s: want %q, got %v", k, want, got[k])
		}
	}
}

// Sem os campos, as tags têm que estar ausentes — tag vazia é rejeição.
func TestBuildEmitOmiteCamposAusentes(t *testing.T) {
	org := map[string]any{"name": "ACME", "person": map[string]any{
		"crt":                 1,
		"state_registrations": []any{map[string]any{"uf": "PI", "state_registration": "194000000"}},
		"addresses":           []any{map[string]any{"state_federation": "PI"}},
	}}
	got := buildEmit(org, getPersonMap(org), "CNPJ_11647612000197", "PI", "PI", 1)
	for _, k := range []string{"IEST", "IM", "CNAE", "ISUFEmit"} {
		if _, ok := got[k]; ok {
			t.Fatalf("%s não deveria estar presente", k)
		}
	}
}

// IEST é da UF de destino, não da UF do emitente: numa venda interna sem
// inscrição de ST na própria UF, a tag não sai.
func TestBuildEmitIESTUsaUFDeDestino(t *testing.T) {
	org := map[string]any{"name": "ACME", "person": map[string]any{
		"state_registrations": []any{
			map[string]any{"uf": "PI", "state_registration": "194000000"},
			map[string]any{"uf": "SP", "state_registration": "111", "ie_st": "070000000"},
		},
		"addresses": []any{map[string]any{"state_federation": "PI"}},
	}}
	if got := buildEmit(org, getPersonMap(org), "CNPJ_11647612000197", "PI", "PI", 3); got["IEST"] != nil {
		t.Fatalf("IEST não deveria sair na operação interna: %v", got["IEST"])
	}
	if got := buildEmit(org, getPersonMap(org), "CNPJ_11647612000197", "PI", "SP", 3); got["IEST"] != "070000000" {
		t.Fatalf("IEST na UF de destino: %v", got["IEST"])
	}
}

func TestBuildDestEstrangeiro(t *testing.T) {
	receiver := map[string]any{"name": "John Doe", "sk": "IDEST_A1234567", "person": map[string]any{
		"id_estrangeiro": "A1234567",
		"addresses":      []any{map[string]any{"state_federation": "EX", "city": "Exterior"}},
	}}
	got := buildDest(receiver, getPersonMap(receiver), "IDEST_A1234567", "EX", false, 1, "")
	if got["idEstrangeiro"] != "A1234567" {
		t.Fatalf("idEstrangeiro ausente: %v", got)
	}
	if _, ok := got["CPF"]; ok {
		t.Fatal("CPF não pode coexistir com idEstrangeiro (choice do XSD)")
	}
	if _, ok := got["CNPJ"]; ok {
		t.Fatal("CNPJ não pode coexistir com idEstrangeiro")
	}
	if got["indIEDest"] != indIEDestNaoContrib {
		t.Fatalf("estrangeiro é sempre não contribuinte: %v", got["indIEDest"])
	}
}

func TestBuildDestNilQuandoSemDestinatario(t *testing.T) {
	if got := buildDest(nil, nil, "", "SP", true, 1, ""); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}
