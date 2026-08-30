package tables

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

// tsCatalogPath é o catálogo TypeScript gerado pelo mesmo script. Go e UI leem
// domínios fechados da mesma fonte; este teste é o que impede divergência.
const tsCatalogPath = "../../../ui/src/lib/data/nfse_enums.ts"

var (
	tsCatalogRe = regexp.MustCompile(`(?s)NFSE_ENUMS: Record<string, readonly NfseEnumOption\[\]> = (\{.*?\}) as const;`)
	tsAliasRe   = regexp.MustCompile(`export const (\w+): readonly NfseEnumOption\[\] = NFSE_ENUMS\.(\w+);`)
)

type tsEnumOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TestEnumCatalogMatchesTypeScript prova que o catálogo Go e o TypeScript são
// o mesmo dado. Falha aqui significa que alguém editou um gerado à mão ou
// regenerou só um dos lados.
func TestEnumCatalogMatchesTypeScript(t *testing.T) {
	tsCatalog := loadTSCatalog(t)

	if len(tsCatalog) != len(enumTables) {
		t.Fatalf("domínios: Go = %d, TypeScript = %d", len(enumTables), len(tsCatalog))
	}
	for typeName, goEntries := range enumTables {
		tsEntries, ok := tsCatalog[typeName]
		if !ok {
			t.Errorf("domínio %s ausente no TypeScript", typeName)
			continue
		}
		converted := make([]tsEnumOption, len(goEntries))
		for i, entry := range goEntries {
			converted[i] = tsEnumOption{Value: entry.Value, Label: entry.Label}
		}
		if !reflect.DeepEqual(converted, tsEntries) {
			t.Errorf("domínio %s difere:\nGo = %+v\nTS = %+v", typeName, converted, tsEntries)
		}
	}
}

// TestEnumAliasesResolve prova que todo alias legível exportado pela UI aponta
// para um domínio existente — um alias órfão vira `undefined` em runtime, sem
// erro de compilação, porque NFSE_ENUMS é indexado por string.
func TestEnumAliasesResolve(t *testing.T) {
	source := readTSCatalog(t)
	matches := tsAliasRe.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		t.Fatal("nenhum alias encontrado no catálogo TypeScript")
	}
	for _, match := range matches {
		if _, ok := enumTables[match[2]]; !ok {
			t.Errorf("alias %s aponta para domínio inexistente %s", match[1], match[2])
		}
	}
}

// TestEnumEntriesAreLabelled prova que nenhum domínio veio sem rótulo ou com o
// próprio código como rótulo — isso significaria documentação não parseada.
func TestEnumEntriesAreLabelled(t *testing.T) {
	for typeName, entries := range enumTables {
		if len(entries) == 0 {
			t.Errorf("domínio %s vazio", typeName)
		}
		for _, entry := range entries {
			if entry.Value == "" || entry.Label == "" || entry.Label == entry.Value {
				t.Errorf("domínio %s: entrada sem rótulo útil %+v", typeName, entry)
			}
		}
	}
}

func TestIsValidEnum(t *testing.T) {
	if !IsValidEnum(EnumRTCTpReeRepRes, "01") {
		t.Error("01 deveria pertencer a " + EnumRTCTpReeRepRes)
	}
	if IsValidEnum(EnumRTCTpReeRepRes, "05") {
		t.Error("05 não pertence a " + EnumRTCTpReeRepRes)
	}
	if IsValidEnum("TipoInexistente", "01") {
		t.Error("domínio desconhecido nunca pode validar")
	}
	if label, ok := EnumLabel(EnumRTCTipoChaveDFe, "2"); !ok || label != "NF-e" {
		t.Errorf("EnumLabel = %q, %v; esperado \"NF-e\", true", label, ok)
	}
}

func readTSCatalog(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(tsCatalogPath))
	if err != nil {
		t.Fatalf("catálogo TypeScript ausente: %v", err)
	}
	return string(raw)
}

func loadTSCatalog(t *testing.T) map[string][]tsEnumOption {
	t.Helper()
	match := tsCatalogRe.FindStringSubmatch(readTSCatalog(t))
	if match == nil {
		t.Fatal("NFSE_ENUMS não encontrado no catálogo TypeScript")
	}
	var catalog map[string][]tsEnumOption
	if err := json.Unmarshal([]byte(match[1]), &catalog); err != nil {
		t.Fatalf("catálogo TypeScript inválido: %v", err)
	}
	return catalog
}
