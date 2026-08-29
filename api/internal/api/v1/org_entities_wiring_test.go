package v1

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// configResourceSuffix marks the RBAC resources that are covered by a document
// family's scope instead of having a scope family of their own.
const configResourceSuffix = "_configs"

// TestEveryRegistryResourceHasScopeFamily falha quando um cadastro novo entra em
// repositories.resources mas ninguém lembra de dar-lhe uma família de escopo —
// o token com escopo do cadastro passaria a não autorizar nada.
func TestEveryRegistryResourceHasScopeFamily(t *testing.T) {
	families := middleware.ScopeFamilies()
	for _, r := range repositories.AllResources() {
		if !strings.HasPrefix(r, "organization_") || strings.HasSuffix(r, configResourceSuffix) {
			continue
		}
		if _, ok := families[r]; !ok {
			t.Fatalf("resource %q sem família de escopo em middleware/scopes.go", r)
		}
	}
}
