//go:build ignore

// Gerador da tabela de campos (FieldSpec) a partir dos XSDs oficiais versionados em
// go-dfe/schemas. Uso, a partir de go-dfe/validate:
//
//	go run gen/main.go
//
// Saída: specs/{nfe,mdfe,cte,cteos,ctesimp}.json, embutidos pelo pacote validate. As saídas são
// versionadas; este programa não roda em produção nem no CI (o CI só confere que o JSON
// versionado carrega e que os testes de paridade passam).
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ---- DOM mínimo -------------------------------------------------------------

type el struct {
	Name  string
	Attrs map[string]string
	Kids  []*el
}

func parseXSD(path string) (*el, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	var stack []*el
	var root *el
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e := &el{Name: t.Name.Local, Attrs: map[string]string{}}
			for _, a := range t.Attr {
				e.Attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.Kids = append(p.Kids, e)
			} else {
				root = e
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	return root, nil
}

func (e *el) child(name string) *el {
	for _, k := range e.Kids {
		if k.Name == name {
			return k
		}
	}
	return nil
}

func local(s string) string {
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// ---- modelo de saída --------------------------------------------------------

type Simple struct {
	Base    string   `json:"base,omitempty"`
	Pattern []string `json:"pattern,omitempty"`
	Enum    []string `json:"enum,omitempty"`
	Len     *int     `json:"len,omitempty"`
	MinLen  *int     `json:"minLen,omitempty"`
	MaxLen  *int     `json:"maxLen,omitempty"`
	Total   *int     `json:"total,omitempty"`
	Frac    *int     `json:"frac,omitempty"`
	MinIncl string   `json:"minIncl,omitempty"`
	MaxIncl string   `json:"maxIncl,omitempty"`
}

type Attr struct {
	Name     string  `json:"n"`
	Required bool    `json:"req,omitempty"`
	Simple   *Simple `json:"s,omitempty"`
}

type ChoiceGroup struct {
	ID  int `json:"id"`
	Min int `json:"min"`
}

type Node struct {
	Name     string        `json:"n"`
	Min      int           `json:"min"`
	Max      int           `json:"max"` // -1 = ilimitado
	Choice   int           `json:"choice,omitempty"`
	Arm      int           `json:"arm,omitempty"`
	Simple   *Simple       `json:"s,omitempty"`
	Attrs    []*Attr       `json:"a,omitempty"`
	Children []*Node       `json:"c,omitempty"`
	Choices  []ChoiceGroup `json:"choices,omitempty"`
	Any      bool          `json:"any,omitempty"`
}

// ---- registro de tipos ------------------------------------------------------

type registry struct {
	complexTypes map[string]*el
	simpleTypes  map[string]*el
	elements     map[string]*el
}

func load(dir string) (*registry, error) {
	r := &registry{map[string]*el{}, map[string]*el{}, map[string]*el{}}
	files, _ := filepath.Glob(filepath.Join(dir, "*.xsd"))
	sort.Strings(files)
	for _, f := range files {
		root, err := parseXSD(f)
		if err != nil || root == nil {
			return nil, fmt.Errorf("%s: %v", f, err)
		}
		for _, k := range root.Kids {
			name := k.Attrs["name"]
			switch k.Name {
			case "complexType":
				r.complexTypes[name] = k
			case "simpleType":
				r.simpleTypes[name] = k
			case "element":
				r.elements[name] = k
			}
		}
	}
	return r, nil
}

// ---- tipos simples ----------------------------------------------------------

func atoi(s string) *int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

func (r *registry) simple(e *el, depth int) *Simple {
	if e == nil || depth > 12 {
		return nil
	}
	res := &Simple{}
	rest := e.child("restriction")
	if rest == nil {
		return res // union/list: sem facetas
	}
	base := local(rest.Attrs["base"])
	if bt, ok := r.simpleTypes[base]; ok {
		res = r.simple(bt, depth+1)
		if res == nil {
			res = &Simple{}
		}
	} else {
		res.Base = base
	}
	var enums, pats []string
	for _, f := range rest.Kids {
		v := f.Attrs["value"]
		switch f.Name {
		case "enumeration":
			enums = append(enums, v)
		case "pattern":
			pats = append(pats, v)
		case "length":
			res.Len = atoi(v)
		case "minLength":
			res.MinLen = atoi(v)
		case "maxLength":
			res.MaxLen = atoi(v)
		case "totalDigits":
			res.Total = atoi(v)
		case "fractionDigits":
			res.Frac = atoi(v)
		case "minInclusive":
			res.MinIncl = v
		case "maxInclusive":
			res.MaxIncl = v
		}
	}
	if len(enums) > 0 {
		res.Enum = enums
	}
	if len(pats) > 0 {
		res.Pattern = pats // a faceta mais derivada substitui a da base
	}
	return res
}

func (r *registry) simpleFromTypeName(name string) *Simple {
	name = local(name)
	if t, ok := r.simpleTypes[name]; ok {
		return r.simple(t, 0)
	}
	return &Simple{Base: name}
}

// ---- elementos e tipos complexos -------------------------------------------

type walker struct {
	r       *registry
	stack   map[string]bool
	groupID int
	errs    []string
}

func occ(e *el) (int, int) {
	min, max := 1, 1
	if v, ok := e.Attrs["minOccurs"]; ok {
		min, _ = strconv.Atoi(v)
	}
	if v, ok := e.Attrs["maxOccurs"]; ok {
		if v == "unbounded" {
			max = -1
		} else {
			max, _ = strconv.Atoi(v)
		}
	}
	return min, max
}

func (w *walker) element(e *el) *Node {
	min, max := occ(e)
	ref := local(e.Attrs["ref"])
	if ref != "" {
		if g, ok := w.r.elements[ref]; ok {
			n := w.element(g)
			n.Min, n.Max = min, max
			return n
		}
		return &Node{Name: ref, Min: min, Max: max, Any: true} // ex.: ds:Signature
	}
	n := &Node{Name: e.Attrs["name"], Min: min, Max: max}
	w.fillType(n, e)
	return n
}

func (w *walker) fillType(n *Node, e *el) {
	if tn := local(e.Attrs["type"]); tn != "" {
		if ct, ok := w.r.complexTypes[tn]; ok {
			if w.stack[tn] {
				n.Any = true
				return
			}
			w.stack[tn] = true
			w.complexType(n, ct)
			delete(w.stack, tn)
			return
		}
		n.Simple = w.r.simpleFromTypeName(tn)
		return
	}
	if ct := e.child("complexType"); ct != nil {
		w.complexType(n, ct)
		return
	}
	if st := e.child("simpleType"); st != nil {
		n.Simple = w.r.simple(st, 0)
		return
	}
	n.Any = true
}

func (w *walker) complexType(n *Node, ct *el) {
	for _, k := range ct.Kids {
		switch k.Name {
		case "sequence", "all":
			w.particles(n, k, 0, 0)
		case "choice":
			w.choice(n, k)
		case "attribute":
			n.Attrs = append(n.Attrs, w.attr(k))
		case "simpleContent":
			if ext := k.child("extension"); ext != nil {
				n.Simple = w.r.simpleFromTypeName(ext.Attrs["base"])
				for _, a := range ext.Kids {
					if a.Name == "attribute" {
						n.Attrs = append(n.Attrs, w.attr(a))
					}
				}
			}
		case "complexContent":
			n.Any = true
		}
	}
}

func (w *walker) attr(a *el) *Attr {
	at := &Attr{Name: a.Attrs["name"], Required: a.Attrs["use"] == "required"}
	if tn := a.Attrs["type"]; tn != "" {
		at.Simple = w.r.simpleFromTypeName(tn)
	} else if st := a.child("simpleType"); st != nil {
		at.Simple = w.r.simple(st, 0)
	}
	return at
}

// particles percorre uma sequence: elementos viram filhos de n; choice vira grupo.
func (w *walker) particles(n *Node, seq *el, choice, arm int) {
	seqMin, _ := occ(seq)
	for _, k := range seq.Kids {
		switch k.Name {
		case "element":
			c := w.element(k)
			if seqMin == 0 {
				c.Min = 0
			}
			c.Choice, c.Arm = choice, arm
			n.Children = append(n.Children, c)
		case "sequence":
			w.particles(n, k, choice, arm)
		case "choice":
			w.choice(n, k)
		case "any":
			n.Children = append(n.Children, &Node{Name: "*", Min: 0, Max: -1, Any: true})
		}
	}
}

func (w *walker) choice(n *Node, ch *el) {
	w.groupID++
	id := w.groupID
	min, _ := occ(ch)
	n.Choices = append(n.Choices, ChoiceGroup{ID: id, Min: min})
	for i, k := range ch.Kids {
		arm := i + 1
		switch k.Name {
		case "element":
			c := w.element(k)
			c.Choice, c.Arm = id, arm
			n.Children = append(n.Children, c)
		case "sequence":
			w.particles(n, k, id, arm)
		case "choice":
			w.choice(n, k)
		}
	}
}

// ---- main -------------------------------------------------------------------

type target struct {
	pkg, root, out string
}

func main() {
	targets := []target{
		{"../schemas/nfe/PL_010e_v1.02", "NFe", "specs/nfe.json"},
		{"../schemas/mdfe/PL_MDFe_300b_NT012025_1.05", "MDFe", "specs/mdfe.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "CTe", "specs/cte.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "CTeOS", "specs/cteos.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "CTeSimp", "specs/ctesimp.json"},
		// Modais: o infModal do documento é xs:any; o conteúdo tem schema próprio.
		{"../schemas/mdfe/PL_MDFe_300b_NT012025_1.05", "rodo", "specs/mdfe_rodo.json"},
		{"../schemas/mdfe/PL_MDFe_300b_NT012025_1.05", "aereo", "specs/mdfe_aereo.json"},
		{"../schemas/mdfe/PL_MDFe_300b_NT012025_1.05", "aquav", "specs/mdfe_aquav.json"},
		{"../schemas/mdfe/PL_MDFe_300b_NT012025_1.05", "ferrov", "specs/mdfe_ferrov.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "rodo", "specs/cte_rodo.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "rodoOS", "specs/cte_rodoos.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "aereo", "specs/cte_aereo.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "aquav", "specs/cte_aquav.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "duto", "specs/cte_duto.json"},
		{"../schemas/cte/PL_CTe_400_NT2026.005_SN_1.00", "multimodal", "specs/cte_multimodal.json"},
	}
	_ = os.MkdirAll("specs", 0o755)
	for _, t := range targets {
		r, err := load(t.pkg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rootEl, ok := r.elements[t.root]
		if !ok {
			fmt.Fprintf(os.Stderr, "%s: elemento raiz %s não encontrado\n", t.pkg, t.root)
			os.Exit(1)
		}
		w := &walker{r: r, stack: map[string]bool{}}
		root := w.element(rootEl)
		checkPatterns(root)
		b, _ := json.MarshalIndent(root, "", " ")
		if err := os.WriteFile(t.out, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s: %d nós (%s)\n", t.out, count(root), t.root)
	}
}

func count(n *Node) int {
	c := 1
	for _, k := range n.Children {
		c += count(k)
	}
	return c
}

// checkPatterns falha o gerador se algum padrão do XSD não compila em RE2 (o validador usa regexp).
func checkPatterns(n *Node) {
	chk := func(s *Simple, where string) {
		if s == nil {
			return
		}
		for _, p := range s.Pattern {
			if _, err := regexp.Compile("^(?:" + p + ")$"); err != nil {
				fmt.Fprintf(os.Stderr, "padrão não compila em %s: %q: %v\n", where, p, err)
				os.Exit(1)
			}
		}
	}
	chk(n.Simple, n.Name)
	for _, a := range n.Attrs {
		chk(a.Simple, n.Name+"@"+a.Name)
	}
	for _, k := range n.Children {
		checkPatterns(k)
	}
}
