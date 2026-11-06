package validate

import (
	"fmt"
	"math"
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

const centavo = 0.0051

func near(a, b float64) bool { return math.Abs(a-b) <= centavo }

// --- regras NF-e/NFC-e ---

var catalog = []Rule{
	{ID: "I11-vProd", CStat: 629, Mode: ModeWarn, Doc: "nfe", Check: func(doc map[string]any) []string {
		var out []string
		for i, det := range list(sub(doc, "infNFe"), "det") {
			p := sub(det, "prod")
			q, ok1 := num(p, "qCom")
			u, ok2 := num(p, "vUnCom")
			v, ok3 := num(p, "vProd")
			if ok1 && ok2 && ok3 && math.Abs(q*u-v) > 0.011 {
				out = append(out, fmt.Sprintf("item %d: vProd %.2f difere de qCom x vUnCom (%.2f)", i+1, v, q*u))
			}
		}
		return out
	}},
	{ID: "W07-vProd", CStat: 531, Mode: ModeWarn, Doc: "nfe", Check: func(doc map[string]any) []string {
		inf := sub(doc, "infNFe")
		tot, ok := num(sub(inf, "total", "ICMSTot"), "vProd")
		if !ok {
			return nil
		}
		var sum float64
		for _, det := range list(inf, "det") {
			if v, ok := num(sub(det, "prod"), "vProd"); ok {
				sum += v
			}
		}
		if !near(sum, tot) {
			return []string{fmt.Sprintf("total vProd %.2f difere da soma dos itens %.2f", tot, sum)}
		}
		return nil
	}},
	{ID: "W16-vNF", CStat: 610, Mode: ModeWarn, Doc: "nfe", Check: func(doc map[string]any) []string {
		t := sub(doc, "infNFe", "total", "ICMSTot")
		vnf, ok := num(t, "vNF")
		if !ok {
			return nil
		}
		g := func(k string) float64 { v, _ := num(t, k); return v }
		exp := g("vProd") - g("vDesc") - g("vICMSDeson") + g("vST") + g("vFCPST") + g("vFrete") + g("vSeg") + g("vOutro") + g("vII") + g("vIPI") + g("vIPIDevol")
		if !near(exp, vnf) && math.Abs(exp-vnf) > 0.011 {
			return []string{fmt.Sprintf("vNF %.2f difere do calculado pelo MOC %.2f", vnf, exp)}
		}
		return nil
	}},
}
