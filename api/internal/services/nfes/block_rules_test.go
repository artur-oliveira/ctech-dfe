package nfes

import (
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// Todo id promovido a block pela api existe no catálogo da família: um id errado nunca recusaria nada, em silêncio.
// O Mode do catálogo é só o padrão da regra; quem recusa é este mapa (ver CONDUCT.md, portão pré-SEFAZ).
func TestBlockRulesExistInCatalog(t *testing.T) {
	known := map[string]bool{}
	for _, r := range validate.Rules("nfe") {
		known[r.ID] = true
	}
	for id, m := range blockRulesNFCe {
		if m != validate.ModeBlock || !known[id] {
			t.Errorf("%s: modo %s, existe no catálogo: %v", id, m, known[id])
		}
	}
}
