package services

import (
	"fmt"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Um CFOP de saída é [escopo][natureza]: o primeiro dígito é o escopo do
// destino, os três últimos são a natureza fiscal (5920 e 6920 são a mesma
// natureza "920" em escopos diferentes).
const (
	CFOPScopeIntraUF = "5" // dentro da UF do emitente
	CFOPScopeInterUF = "6" // outra UF
	CFOPScopeForeign = "7" // exterior

	// CFOPSuffixLen é o tamanho da natureza fiscal (os 3 últimos dígitos).
	CFOPSuffixLen = 3
	// CFOPLen é o tamanho de um CFOP completo.
	CFOPLen = 4

	// UFForeign é a UF que representa destino no exterior.
	UFForeign = "EX"
)

// ResolveCFOPScope monta o CFOP concreto a partir da natureza fiscal (sufixo de
// 3 dígitos) e das UFs de emitente e destinatário.
//
// Esta é a **fonte da verdade** dessa regra. A implementação em TypeScript
// (ui/src/lib/data/cfop.ts) passa a ser apenas agrupamento de exibição no
// dropdown; o teste de paridade entre as duas roda sobre a mesma tabela de
// casos (testdata/cfop_scope_cases.json).
func ResolveCFOPScope(suffix, emitUF, destUF string) (string, error) {
	suffix = strings.TrimSpace(suffix)
	if len(suffix) != CFOPSuffixLen || !isDigits(suffix) {
		return "", problem.BadRequest(fmt.Sprintf(
			"natureza de operação inválida: %q — esperados %d dígitos (ex.: 102 para venda)",
			suffix, CFOPSuffixLen))
	}
	emitUF = strings.ToUpper(strings.TrimSpace(emitUF))
	destUF = strings.ToUpper(strings.TrimSpace(destUF))
	if emitUF == "" || destUF == "" {
		return "", problem.BadRequest("UF do emitente e do destinatário são obrigatórias para resolver o CFOP")
	}

	switch {
	case destUF == UFForeign:
		return CFOPScopeForeign + suffix, nil
	case emitUF == destUF:
		return CFOPScopeIntraUF + suffix, nil
	default:
		return CFOPScopeInterUF + suffix, nil
	}
}

// CFOPSuffix devolve a natureza fiscal de um CFOP (os 3 últimos dígitos),
// compartilhada entre as variantes intra e interestadual.
func CFOPSuffix(cfop string) string {
	if len(cfop) != CFOPLen {
		return ""
	}
	return cfop[1:]
}

// CFOPScope devolve o dígito de escopo de um CFOP.
func CFOPScope(cfop string) string {
	if len(cfop) != CFOPLen {
		return ""
	}
	return cfop[:1]
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
