// Package validate valida o documento fiscal que os builders produzem (o dicionário que vira XML)
// ANTES de ele ir à SEFAZ: schema (tipo, tamanho, casas decimais, enumeração, ocorrência e choice,
// gerados dos XSDs oficiais em go-dfe/schemas) e regras de negócio do MOC (rules.go).
//
// Nada aqui bloqueia por padrão: cada achado tem severidade e o chamador decide o que fazer
// (log, aviso ao usuário ou recusa). Os esquemas vêm de `go run gen/main.go` e são versionados.
package validate

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

//go:embed specs/*.json
var specFS embed.FS

// Códigos dos achados de schema.
const (
	CodeRequired       = "required"
	CodeUnknownTag     = "unknown_tag"
	CodeMaxOccurs      = "max_occurs"
	CodeChoiceConflict = "choice_conflict"
	CodeChoiceMissing  = "choice_missing"
	CodePattern        = "pattern"
	CodeEnum           = "enum"
	CodeLength         = "length"
	CodeMinLength      = "min_length"
	CodeMaxLength      = "max_length"
	CodeTotalDigits    = "total_digits"
	CodeFractionDigits = "fraction_digits"
	CodeMinInclusive   = "min_inclusive"
	CodeMaxInclusive   = "max_inclusive"
	CodeAttrRequired   = "attr_required"
)

// Severity de um achado. Erro de schema é erro (a SEFAZ recusa com 225); tag desconhecida é aviso.
type Severity string

const (
	SevError Severity = "error"
	SevWarn  Severity = "warn"
)

// Violation é um achado de schema ou de regra de negócio.
type Violation struct {
	Path     string   `json:"path"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
	// Rule e CStat só vêm das regras de negócio (id do MOC, por exemplo "W16-10", e a rejeição).
	Rule  string `json:"rule,omitempty"`
	CStat int    `json:"cstat,omitempty"`
}

type simple struct {
	Base    string   `json:"base"`
	Pattern []string `json:"pattern"`
	Enum    []string `json:"enum"`
	Len     *int     `json:"len"`
	MinLen  *int     `json:"minLen"`
	MaxLen  *int     `json:"maxLen"`
	Total   *int     `json:"total"`
	Frac    *int     `json:"frac"`
	MinIncl string   `json:"minIncl"`
	MaxIncl string   `json:"maxIncl"`

	patterns []*regexp.Regexp
}

type attr struct {
	Name     string  `json:"n"`
	Required bool    `json:"req"`
	Simple   *simple `json:"s"`
}

type choiceGroup struct {
	ID  int `json:"id"`
	Min int `json:"min"`
}

type node struct {
	Name     string        `json:"n"`
	Min      int           `json:"min"`
	Max      int           `json:"max"`
	Choice   int           `json:"choice"`
	Arm      int           `json:"arm"`
	Simple   *simple       `json:"s"`
	Attrs    []*attr       `json:"a"`
	Children []*node       `json:"c"`
	Choices  []choiceGroup `json:"choices"`
	Any      bool          `json:"any"`

	byName  map[string]*node
	overlay bool // acrescentado por overlay.go: fora da ordem do XSD vendorizado
}

// Spec é a tabela de campos de um documento (ou de um modal).
type Spec struct {
	Name string
	root *node
}

var (
	specMu sync.Mutex
	specs  = map[string]*Spec{}
)

// Load devolve a tabela embutida de name ("nfe", "mdfe", "cte", "cteos", "ctesimp", "mdfe_rodo", ...).
func Load(name string) (*Spec, error) {
	specMu.Lock()
	defer specMu.Unlock()
	if s, ok := specs[name]; ok {
		return s, nil
	}
	raw, err := specFS.ReadFile("specs/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("validate: esquema %q não existe", name)
	}
	var root node
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("validate: esquema %q inválido: %w", name, err)
	}
	if err := prepare(&root); err != nil {
		return nil, fmt.Errorf("validate: esquema %q: %w", name, err)
	}
	if name == "nfe" {
		applyNFeOverlay(&root)
	}
	s := &Spec{Name: name, root: &root}
	specs[name] = s
	return s, nil
}

func prepare(n *node) error {
	n.byName = make(map[string]*node, len(n.Children))
	for _, c := range n.Children {
		n.byName[c.Name] = c
		if err := prepare(c); err != nil {
			return err
		}
	}
	for _, s := range append([]*simple{n.Simple}, attrSimples(n)...) {
		if s == nil {
			continue
		}
		for _, p := range s.Pattern {
			re, err := regexp.Compile("^(?:" + p + ")$")
			if err != nil {
				return fmt.Errorf("padrão %q em %s: %w", p, n.Name, err)
			}
			s.patterns = append(s.patterns, re)
		}
	}
	return nil
}

func attrSimples(n *node) []*simple {
	out := make([]*simple, 0, len(n.Attrs))
	for _, a := range n.Attrs {
		out = append(out, a.Simple)
	}
	return out
}

// Options ajusta a validação.
type Options struct {
	// Skip lista caminhos (sem o nome da raiz) que não são validados: o que é preenchido depois, como
	// a assinatura (Signature), que o egress acrescenta.
	Skip []string
}

// Validate valida doc, o conteúdo do elemento raiz do esquema (por exemplo o mapa de "NFe").
func (s *Spec) Validate(doc map[string]any, opt Options) []Violation {
	v := &validator{skip: map[string]bool{}}
	for _, p := range opt.Skip {
		v.skip[s.root.Name+"/"+strings.Trim(p, "/")] = true
	}
	v.element(s.root, doc, s.root.Name)
	sort.SliceStable(v.out, func(i, j int) bool { return v.out[i].Path < v.out[j].Path })
	return v.out
}

type validator struct {
	out  []Violation
	skip map[string]bool
}

func (v *validator) add(path, code string, sev Severity, format string, args ...any) {
	v.out = append(v.out, Violation{Path: path, Code: code, Severity: sev, Message: fmt.Sprintf(format, args...)})
}

// element valida um valor contra n. value pode ser mapa (complexo), lista (repetido) ou escalar.
func (v *validator) element(n *node, value any, path string) {
	if v.skip[path] || n.Any {
		return
	}
	m, isMap := value.(map[string]any)
	if len(n.Children) == 0 && len(n.Choices) == 0 && !isMap {
		v.simpleValue(n.Simple, scalar(value), path, SevError)
		return
	}
	if !isMap {
		// elemento complexo recebeu escalar: só é válido se o tipo aceitar texto (simpleContent)
		if n.Simple != nil {
			v.simpleValue(n.Simple, scalar(value), path, SevError)
		}
		return
	}
	if n.Simple != nil {
		if t, ok := m["#text"]; ok {
			v.simpleValue(n.Simple, scalar(t), path, SevError)
		}
	}
	v.attrs(n, m, path)

	for k := range m {
		if strings.HasPrefix(k, "@") || k == "#text" {
			continue
		}
		if _, ok := n.byName[k]; !ok && !hasWildcard(n) {
			v.add(path+"/"+k, CodeUnknownTag, SevWarn, "tag %s não existe no leiaute de %s", k, n.Name)
		}
	}

	armsPresent := map[int]map[int]bool{}
	for _, c := range n.Children {
		cp := path + "/" + c.Name
		val, present := m[c.Name]
		count := occurrences(val, present)
		if present && c.Choice != 0 && count > 0 {
			if armsPresent[c.Choice] == nil {
				armsPresent[c.Choice] = map[int]bool{}
			}
			armsPresent[c.Choice][c.Arm] = true
		}
		if count == 0 {
			if c.Min >= 1 && c.Choice == 0 && !v.skip[cp] && !c.Any {
				v.add(cp, CodeRequired, SevError, "tag %s é obrigatória em %s", c.Name, n.Name)
			}
			continue
		}
		if c.Max >= 0 && count > c.Max {
			v.add(cp, CodeMaxOccurs, SevError, "tag %s aparece %d vezes; o máximo é %d", c.Name, count, c.Max)
		}
		for i, item := range items(val) {
			ip := cp
			if count > 1 {
				ip = fmt.Sprintf("%s[%d]", cp, i)
			}
			v.element(c, item, ip)
		}
	}
	for _, g := range n.Choices {
		arms := len(armsPresent[g.ID])
		switch {
		case arms > 1:
			v.add(path, CodeChoiceConflict, SevError, "em %s só uma das alternativas pode ser informada", n.Name)
		case arms == 0 && g.Min >= 1 && !allSkipped(n, g.ID, v.skip, path):
			v.add(path, CodeChoiceMissing, SevError, "em %s é preciso informar uma das alternativas", n.Name)
		}
	}
}

func allSkipped(n *node, id int, skip map[string]bool, path string) bool {
	for _, c := range n.Children {
		if c.Choice == id && !skip[path+"/"+c.Name] && !c.Any {
			return false
		}
	}
	return true
}

func hasWildcard(n *node) bool {
	_, ok := n.byName["*"]
	return ok
}

func (v *validator) attrs(n *node, m map[string]any, path string) {
	for _, a := range n.Attrs {
		val, ok := m["@"+a.Name]
		if !ok {
			if a.Required {
				v.add(path+"/@"+a.Name, CodeAttrRequired, SevError, "atributo %s é obrigatório em %s", a.Name, n.Name)
			}
			continue
		}
		v.simpleValue(a.Simple, scalar(val), path+"/@"+a.Name, SevError)
	}
}

func occurrences(val any, present bool) int {
	if !present || val == nil {
		return 0
	}
	switch t := val.(type) {
	case []any:
		return len(t)
	case []map[string]any:
		return len(t)
	case []string:
		return len(t)
	}
	return 1
}

func items(val any) []any {
	switch t := val.(type) {
	case []any:
		return t
	case []map[string]any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = t[i]
		}
		return out
	case []string:
		out := make([]any, len(t))
		for i := range t {
			out[i] = t[i]
		}
		return out
	}
	return []any{val}
}

func scalar(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(t)
	}
}

// simpleValue aplica as facetas do tipo simples a s.
func (v *validator) simpleValue(sp *simple, s, path string, sev Severity) {
	if sp == nil {
		return
	}
	n := utf8.RuneCountInString(s)
	if sp.Len != nil && n != *sp.Len {
		v.add(path, CodeLength, sev, "%q deve ter exatamente %d caracteres (tem %d)", short(s), *sp.Len, n)
	}
	if sp.MinLen != nil && n < *sp.MinLen {
		v.add(path, CodeMinLength, sev, "%q deve ter ao menos %d caracteres (tem %d)", short(s), *sp.MinLen, n)
	}
	if sp.MaxLen != nil && n > *sp.MaxLen {
		v.add(path, CodeMaxLength, sev, "%q deve ter no máximo %d caracteres (tem %d)", short(s), *sp.MaxLen, n)
	}
	if len(sp.Enum) > 0 && !contains(sp.Enum, s) {
		v.add(path, CodeEnum, sev, "%q não está entre os valores permitidos (%s)", short(s), strings.Join(firstN(sp.Enum, 8), ", "))
	}
	if len(sp.patterns) > 0 {
		ok := false
		for _, re := range sp.patterns {
			if re.MatchString(s) {
				ok = true
				break
			}
		}
		if !ok {
			v.add(path, CodePattern, sev, "%q não segue o formato do leiaute", short(s))
		}
	}
	if sp.Total != nil || sp.Frac != nil || sp.MinIncl != "" || sp.MaxIncl != "" {
		v.numeric(sp, s, path, sev)
	}
}

func (v *validator) numeric(sp *simple, s, path string, sev Severity) {
	digits, frac := countDigits(s)
	if sp.Total != nil && digits > *sp.Total {
		v.add(path, CodeTotalDigits, sev, "%q tem %d dígitos; o máximo é %d", short(s), digits, *sp.Total)
	}
	if sp.Frac != nil && frac > *sp.Frac {
		v.add(path, CodeFractionDigits, sev, "%q tem %d casas decimais; o máximo é %d", short(s), frac, *sp.Frac)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return
	}
	if sp.MinIncl != "" {
		if lim, err := strconv.ParseFloat(sp.MinIncl, 64); err == nil && f < lim {
			v.add(path, CodeMinInclusive, sev, "%q é menor que o mínimo %s", short(s), sp.MinIncl)
		}
	}
	if sp.MaxIncl != "" {
		if lim, err := strconv.ParseFloat(sp.MaxIncl, 64); err == nil && f > lim {
			v.add(path, CodeMaxInclusive, sev, "%q é maior que o máximo %s", short(s), sp.MaxIncl)
		}
	}
}

// countDigits devolve o total de dígitos e o de casas decimais de um número em texto.
func countDigits(s string) (total, frac int) {
	s = strings.TrimLeft(s, "+-")
	whole, fr, hasDot := strings.Cut(s, ".")
	whole = strings.TrimLeft(whole, "0")
	total = len(whole) + len(fr)
	if hasDot {
		frac = len(fr)
	}
	if total == 0 {
		total = 1
	}
	return total, frac
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func firstN(list []string, n int) []string {
	if len(list) > n {
		return append(append([]string{}, list[:n]...), "…")
	}
	return list
}

func short(s string) string {
	if utf8.RuneCountInString(s) > 40 {
		return string([]rune(s)[:40]) + "…"
	}
	return s
}
