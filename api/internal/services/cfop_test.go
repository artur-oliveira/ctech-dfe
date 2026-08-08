package services

import (
	"encoding/json"
	"os"
	"testing"
)

// cfopScopeCase espelha uma linha de testdata/cfop_scope_cases.json — o mesmo
// arquivo lido pelo teste de paridade em TypeScript.
type cfopScopeCase struct {
	Name   string `json:"name"`
	Suffix string `json:"suffix"`
	EmitUF string `json:"emit_uf"`
	DestUF string `json:"dest_uf"`
	CFOP   string `json:"cfop"`
	Error  bool   `json:"error"`
}

func loadCFOPScopeCases(t *testing.T) []cfopScopeCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/cfop_scope_cases.json")
	if err != nil {
		t.Fatalf("ler tabela de casos: %v", err)
	}
	var doc struct {
		Cases []cfopScopeCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decodificar tabela de casos: %v", err)
	}
	if len(doc.Cases) == 0 {
		t.Fatal("tabela de casos vazia")
	}
	return doc.Cases
}

func TestResolveCFOPScope(t *testing.T) {
	for _, tc := range loadCFOPScopeCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := ResolveCFOPScope(tc.Suffix, tc.EmitUF, tc.DestUF)
			if tc.Error {
				if err == nil {
					t.Fatalf("esperado erro, obtido %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveCFOPScope: %v", err)
			}
			if got != tc.CFOP {
				t.Errorf("CFOP = %q, esperado %q", got, tc.CFOP)
			}
		})
	}
}

func TestCFOPSuffixAndScope(t *testing.T) {
	if got := CFOPSuffix("5920"); got != "920" {
		t.Errorf("CFOPSuffix = %q, esperado 920", got)
	}
	if got := CFOPScope("6102"); got != CFOPScopeInterUF {
		t.Errorf("CFOPScope = %q, esperado %q", got, CFOPScopeInterUF)
	}
	// CFOP de tamanho errado devolve vazio em vez de fatiar fora do intervalo.
	if got := CFOPSuffix("51"); got != "" {
		t.Errorf("CFOPSuffix de CFOP curto = %q, esperado vazio", got)
	}
}
