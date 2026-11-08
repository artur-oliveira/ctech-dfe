package validate

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/internal/xmlops/xsdorder"
)

// A ordem dos filhos que o builder de XML usa (xsdorder) tem de seguir a sequência do XSD: ordem errada é
// rejeição 215 da SEFAZ. Compara cada grupo do esquema com a tabela, para NF-e, MDF-e e CT-e.
func TestXSDOrderMatchesSpecs(t *testing.T) {
	var problems []string
	for _, name := range []string{"nfe", "mdfe", "cte"} {
		sp, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		var walk func(parent string, n *node, path string)
		walk = func(parent string, n *node, path string) {
			if len(n.Children) >= 2 {
				if order, ok := xsdorder.Resolve(parent, n.Name); ok {
					problems = append(problems, compareOrder(name, path, order, n)...)
				} else if !onlyAny(n) && !allChoice(n) {
					problems = append(problems, fmt.Sprintf("%s %s: sem entrada no xsdorder (%d filhos)", name, path, len(n.Children)))
				}
			}
			for _, c := range n.Children {
				walk(anc(parent, n.Name), c, path+"/"+c.Name)
			}
		}
		walk("", sp.root, sp.root.Name)
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

// Mesma conferência para os XSDs de evento: um spec por evento (detEvento da NF-e; grupo evXxx do CT-e e do
// MDF-e) e o envelope de cada documento. A tabela de detEvento da NF-e é única para todos os tipos, então um
// evento que pede ordem diferente de outro aparece aqui.
func TestXSDOrderMatchesEventSpecs(t *testing.T) {
	// Eventos que nenhum builder emite hoje: sem entrada no xsdorder. Quem criar o builder acrescenta a tabela
	// e tira o evento desta lista.
	notBuilt := map[string]bool{
		"evento_cte_evApropriaCredPres": true, "evento_cte_evCancTribSN": true, "evento_cte_evCancVincPgto": true,
		"evento_cte_evEPECCTe": true, "evento_cte_evGTV": true, "evento_cte_evRegMultimodal": true,
		"evento_cte_evTributacaoSN": true, "evento_cte_evVincPgto": true,
		"evento_nfe_112120": true, "evento_nfe_112130": true, "evento_nfe_412120": true, "evento_nfe_412130": true,
	}
	files, _ := filepath.Glob("specs/evento_*.json")
	if len(files) == 0 {
		t.Fatal("nenhum spec de evento embutido")
	}
	var problems []string
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		if notBuilt[name] {
			continue
		}
		sp, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		var walk func(parent string, n *node, path string)
		walk = func(parent string, n *node, path string) {
			if len(n.Children) >= 2 {
				if order, ok := xsdorder.Resolve(parent, n.Name); ok {
					problems = append(problems, compareOrder(name, path, order, n)...)
				} else if !onlyAny(n) && !allChoice(n) {
					problems = append(problems, fmt.Sprintf("%s %s: sem entrada no xsdorder (%d filhos)", name, path, len(n.Children)))
				}
			}
			for _, c := range n.Children {
				walk(anc(parent, n.Name), c, path+"/"+c.Name)
			}
		}
		walk("", sp.root, sp.root.Name)
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

// anc monta o caminho de ancestrais "avô:pai" que o builder de XML passa a Resolve.
func anc(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + ":" + name
}

// allChoice: grupo de alternativas (ICMS, PIS, COFINS); cada braço tem a própria ordem.
func allChoice(n *node) bool {
	for _, c := range n.Children {
		if c.Choice == 0 && !c.Any {
			return false
		}
	}
	return true
}

func onlyAny(n *node) bool {
	for _, c := range n.Children {
		if !c.Any {
			return false
		}
	}
	return true
}

func compareOrder(doc, path string, order []string, n *node) []string {
	pos := map[string]int{}
	for i, t := range order {
		pos[t] = i
	}
	var out []string
	last, lastName := -1, ""
	for _, c := range n.Children {
		if c.Any || c.overlay || c.Choice != 0 {
			continue
		}
		p, ok := pos[c.Name]
		if !ok {
			out = append(out, fmt.Sprintf("%s %s: %s falta no xsdorder", doc, path, c.Name))
			continue
		}
		if p < last {
			out = append(out, fmt.Sprintf("%s %s: %s deveria vir depois de %s", doc, path, c.Name, lastName))
			continue
		}
		last, lastName = p, c.Name
	}
	return out
}
