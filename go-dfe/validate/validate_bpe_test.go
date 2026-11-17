package validate

import "testing"

func TestBPeSpecsLoad(t *testing.T) {
	for _, n := range []string{
		"bpe", "bpetm", "bpeta",
		"evento_bpe_eventoBPe", "evento_bpe_evCancBPe", "evento_bpe_evNaoEmbBPe", "evento_bpe_evAlteracaoPoltrona",
		"evento_bpe_evExcessoBagagem", "evento_bpe_evVincPgto", "evento_bpe_evCancVincPgto", "evento_bpe_evFornecNaoRealizBPe",
	} {
		if _, err := Load(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}

// Documento quase vazio: acusa o grupo obrigatório e o modelo fora de 63, nas três variantes.
func TestBPeSchemaFindings(t *testing.T) {
	for _, c := range []struct{ spec, root string }{{"bpe", "BPe"}, {"bpetm", "BPeTM"}, {"bpeta", "BPeTA"}} {
		s, err := Load(c.spec)
		if err != nil {
			t.Fatal(err)
		}
		doc := map[string]any{"infBPe": map[string]any{
			"@versao": "1.00", "@Id": "x", "ide": map[string]any{"cUF": "35", "mod": "99", "bogus": "x"},
		}}
		got := codes(s.Validate(doc, Options{Skip: []string{"Signature"}}))
		for _, want := range []string{
			c.root + "/infBPe/@Id|pattern", c.root + "/infBPe/ide/mod|enum", c.root + "/infBPe/ide/bogus|unknown_tag", c.root + "/infBPe/emit|required",
		} {
			if !got[want] {
				t.Errorf("%s: faltou %s; got %v", c.spec, want, got)
			}
		}
	}
}

// 112140 (fornecimento não realizado, só TA): schema escrito à mão da NT 2025.002 6.3; vIBS e vCBS com 13v2.
func TestBPeFornecNaoRealizSchema(t *testing.T) {
	s, err := Load("evento_bpe_evFornecNaoRealizBPe")
	if err != nil {
		t.Fatal(err)
	}
	ok := map[string]any{"descEvento": "Fornecimento Não Realizado", "nProt": "135260000000001", "vIBS": "10.00", "vCBS": "5.00"}
	if got := s.Validate(ok, Options{}); len(got) != 0 {
		t.Errorf("evento válido com achados: %+v", got)
	}
	bad := map[string]any{"descEvento": "Fornecimento Não Realizado", "nProt": "135260000000001", "vIBS": "10.001", "vCBS": "5.00"}
	if got := codes(s.Validate(bad, Options{})); !got["evFornecNaoRealizBPe/vIBS|pattern"] {
		t.Errorf("vIBS com 3 casas devia falhar: %v", got)
	}
}
