package validate

import (
	"strconv"
)

// Mode de uma regra de negócio. O padrão é Warn: o documento segue e o achado vira aviso.
type Mode string

const (
	ModeOff    Mode = "off"
	ModeShadow Mode = "shadow" // só registra (log); não chega ao usuário
	ModeWarn   Mode = "warn"   // aviso ao usuário, emissão segue
	ModeBlock  Mode = "block"  // recusa antes da SEFAZ; só para regra determinística e provável offline
)

// Rule é uma regra de validação do MOC avaliada offline. Check devolve a mensagem do achado ou "".
type Rule struct {
	ID    string // identificador do MOC (ex.: "W16-10")
	CStat int    // rejeição que a SEFAZ devolveria
	Mode  Mode   // padrão da regra
	Doc   string // "nfe", "mdfe", "cte"
	Check func(doc map[string]any) []string
}

// Modes sobrescreve o modo por id de regra (vem de configuração, sem deploy).
type Modes map[string]Mode

// Rules devolve o catálogo da família doc.
func Rules(doc string) []Rule {
	var out []Rule
	for _, r := range catalog {
		if r.Doc == doc {
			out = append(out, r)
		}
	}
	return out
}

// Apply roda as regras de doc sobre o documento e devolve os achados, já com a severidade do modo.
// block vira SevError; warn e shadow viram SevWarn (o chamador decide mostrar ou só logar via Mode).
func Apply(docType string, doc map[string]any, over Modes) (found []Violation, blocking bool) {
	for _, r := range Rules(docType) {
		mode := r.Mode
		if m, ok := over[r.ID]; ok {
			mode = m
		}
		if mode == ModeOff || mode == "" {
			continue
		}
		for _, msg := range r.Check(doc) {
			sev := SevWarn
			if mode == ModeBlock {
				sev = SevError
				blocking = true
			}
			found = append(found, Violation{Path: r.ID, Code: "rule", Message: msg, Severity: sev, Rule: r.ID, CStat: r.CStat})
		}
	}
	return found, blocking
}

// --- navegação tolerante no dicionário ---

func sub(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		next, _ := cur[k].(map[string]any)
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

func list(m map[string]any, key string) []map[string]any {
	switch t := m[key].(type) {
	case map[string]any:
		return []map[string]any{t}
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, x := range t {
			if mm, ok := x.(map[string]any); ok {
				out = append(out, mm)
			}
		}
		return out
	}
	return nil
}

func num(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	switch t := m[key].(type) {
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	case float64:
		return t, true
	case int:
		return float64(t), true
	}
	return 0, false
}

// O catálogo vive em rules_nfe.go, rules_mdfe.go e rules_cte.go (registrados em init).
var catalog []Rule
