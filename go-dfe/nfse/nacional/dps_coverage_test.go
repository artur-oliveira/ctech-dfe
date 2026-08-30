package nacional

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// manifestPath aponta para o inventário canônico gerado do XSD por
// nfse/tables/gen/dps_manifest.py. Regerar após qualquer troca de leiaute.
const manifestPath = "testdata/dps_paths_v1.01.json"

// xmlnsAttr é declaração de namespace, não campo do leiaute.
const xmlnsAttr = "xmlns"

type dpsManifest struct {
	Schema string `json:"schema"`
	Root   string `json:"root"`
	Paths  []struct {
		Path   string `json:"path"`
		Kind   string `json:"kind"`
		Choice string `json:"choice"`
	} `json:"paths"`
}

// TestDPSCoverageMatchesSchema prova que o montador da DPS tem um campo para
// cada caminho emitível do TCDPS e nenhum caminho fora do leiaute. O gate
// substitui qualquer porcentagem manual de cobertura: falha listando os
// caminhos exatos que faltam ou sobram.
func TestDPSCoverageMatchesSchema(t *testing.T) {
	manifest := loadManifest(t)

	expected := map[string]bool{}
	for _, entry := range manifest.Paths {
		expected[entry.Path] = true
	}

	emitted := map[string]bool{}
	collectXMLPaths(reflect.TypeOf(xmlDPS{}), manifest.Root, emitted)

	if missing := difference(expected, emitted); len(missing) > 0 {
		t.Errorf("caminhos do %s sem campo no montador (%d):\n%s",
			manifest.Schema, len(missing), strings.Join(missing, "\n"))
	}
	if unexpected := difference(emitted, expected); len(unexpected) > 0 {
		t.Errorf("caminhos emitidos fora do %s (%d):\n%s",
			manifest.Schema, len(unexpected), strings.Join(unexpected, "\n"))
	}
}

// TestDPSChoiceAlternativesCovered prova que toda alternativa de xs:choice
// emitível tem campo próprio — uma escolha só coberta pela primeira alternativa
// esconde um caminho que o autorizador aceita e o sistema nunca produz.
func TestDPSChoiceAlternativesCovered(t *testing.T) {
	manifest := loadManifest(t)

	emitted := map[string]bool{}
	collectXMLPaths(reflect.TypeOf(xmlDPS{}), manifest.Root, emitted)

	alternatives := map[string][]string{}
	for _, entry := range manifest.Paths {
		if entry.Choice == "" {
			continue
		}
		group := entry.Choice[:strings.LastIndex(entry.Choice, "#choice")+len("#choice")]
		alternatives[group] = append(alternatives[group], entry.Path)
	}

	for group, paths := range alternatives {
		for _, path := range paths {
			if !emitted[path] {
				t.Errorf("alternativa não emitível da escolha %s: %s", group, path)
			}
		}
	}
}

func loadManifest(t *testing.T) dpsManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(manifestPath))
	if err != nil {
		t.Fatalf("manifesto da DPS ausente: %v", err)
	}
	var manifest dpsManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifesto da DPS inválido: %v", err)
	}
	if len(manifest.Paths) == 0 {
		t.Fatal("manifesto da DPS vazio")
	}
	return manifest
}

// collectXMLPaths percorre as structs de marshalling e registra cada caminho
// que o encoding/xml pode escrever, incluindo wrappers "a>b" e atributos.
func collectXMLPaths(t reflect.Type, prefix string, out map[string]bool) {
	out[prefix] = true
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name, isAttr, ok := xmlFieldName(field)
		if !ok {
			continue
		}
		if isAttr {
			if name != xmlnsAttr {
				out[prefix+"/@"+name] = true
			}
			continue
		}
		path := prefix
		for _, segment := range strings.Split(name, ">") {
			path += "/" + segment
			out[path] = true
		}
		collectXMLPaths(field.Type, path, out)
	}
}

func xmlFieldName(field reflect.StructField) (name string, isAttr, ok bool) {
	tag := field.Tag.Get("xml")
	if tag == "" || tag == "-" || field.Name == "XMLName" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	for _, option := range parts[1:] {
		if option == "attr" {
			isAttr = true
		}
	}
	if parts[0] == "" {
		return "", false, false
	}
	return parts[0], isAttr, true
}

func difference(from, other map[string]bool) []string {
	var out []string
	for path := range from {
		if !other[path] {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}
