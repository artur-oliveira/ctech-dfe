package dfecheck

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// O portão só recusa as regras promovidas a block e os achados de tamanho; o resto continua só no shadow.
func TestGateOnlyBlocksPromotedRulesAndDeterministicSchema(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA", "block")
	nfe := map[string]any{"infNFe": map[string]any{
		"ide":  map[string]any{"mod": "55", "finNFe": "1", "tpNFDebito": "01", "tpNF": "1"},
		"dest": map[string]any{"xNome": strings.Repeat("a", 61)},
	}}
	body := map[string]any{"enviNFe": map[string]any{"NFe": nfe}}

	found := GateEnviNFe(body, validate.Modes{"B25.1-10": validate.ModeBlock})
	var rule, size bool
	for _, v := range found {
		switch {
		case v.Rule == "B25.1-10":
			rule = true
		case v.Code == validate.CodeMaxLength && v.Path == "NFe/infNFe/dest/xNome":
			size = true
		case v.Rule != "":
			t.Errorf("regra não promovida recusou: %s", v.Rule)
		case v.Code != validate.CodeMinLength && v.Code != validate.CodeMaxLength && v.Code != validate.CodeRequired &&
			v.Code != validate.CodeChoiceMissing:
			// o documento do teste é parcial: obrigatório e choice ausentes são esperados com o schema em block.
			t.Errorf("achado de schema fora da tabela de block recusou: %s %s", v.Path, v.Code)
		}
	}
	if !rule || !size {
		t.Fatalf("esperado B25.1-10 e o tamanho de dest/xNome; achados: %+v", found)
	}
	if got := GateEnviNFe(map[string]any{}, nil); got != nil {
		t.Errorf("corpo sem NFe deveria passar: %v", got)
	}
}

func TestSchemaModeKillSwitch(t *testing.T) {
	for _, k := range []Kind{KindNFe, KindNFCe, KindCTe, KindCTeOS, KindMDFe, KindNFeEvent, KindCTeEvent, KindMDFeEvent} {
		if got := SchemaMode(k); got != validate.ModeShadow {
			t.Fatalf("padrão de %s = %s, quer shadow", k, got)
		}
	}
	t.Setenv("DFE_GATE_SCHEMA", "warn")
	if got := SchemaMode(KindCTe); got != validate.ModeWarn {
		t.Errorf("geral warn = %s", got)
	}
	t.Setenv("DFE_GATE_SCHEMA_CTE", "OFF")
	if got := SchemaMode(KindCTe); got != validate.ModeOff {
		t.Errorf("por tipo off = %s", got)
	}
	t.Setenv("DFE_GATE_SCHEMA_MDFE", "blokc") // erro de digitação: cai no geral, não liga nem desliga sozinho
	if got := SchemaMode(KindMDFe); got != validate.ModeWarn {
		t.Errorf("valor inválido = %s", got)
	}
}

func TestGateSchemaModesPerKind(t *testing.T) {
	bad := map[string]any{"infNFe": map[string]any{"@Id": "x", "ide": map[string]any{"mod": "99"}}}
	codes := func(m validate.Mode) map[string]bool {
		out := map[string]bool{}
		for _, v := range Gate(SpecNFe, "nfe", bad, nil, m) {
			out[v.Code] = true
		}
		return out
	}
	if got := codes(validate.ModeBlock); !got[validate.CodePattern] || !got[validate.CodeEnum] || !got[validate.CodeRequired] {
		t.Errorf("block deve recusar pattern, enum e required: %v", got)
	}
	for _, m := range []validate.Mode{validate.ModeWarn, validate.ModeShadow} {
		if got := codes(m); got[validate.CodePattern] || got[validate.CodeEnum] || got[validate.CodeRequired] {
			t.Errorf("%s recusa só tamanho: %v", m, got)
		}
	}
	if got := codes(validate.ModeOff); len(got) != 0 {
		t.Errorf("off não recusa nada: %v", got)
	}
}

// Eventos: o portão recusa o corpo fora do XSD com 422 fiscal_prevalidation e só caminho e motivo na mensagem.
func TestGateEventBlocksAndKillSwitch(t *testing.T) {
	body := map[string]any{"envEvento": map[string]any{"idLote": "ABC"}}
	if got := GateEvent("nfe", body); got != nil {
		t.Fatalf("o padrão é shadow e não recusa: %v", got)
	}
	t.Setenv("DFE_GATE_SCHEMA_NFE_EVENT", "block")
	found := GateEvent("nfe", body)
	if len(found) == 0 {
		t.Fatal("corpo fora do XSD deveria recusar")
	}
	err := EventProblem("NF-e", found)
	p, ok := err.(*problem.Problem)
	if !ok || p.Status != 422 || p.Code != CodeFiscalPrevalidation {
		t.Fatalf("want 422 fiscal_prevalidation, got %#v", err)
	}
	for _, e := range p.Errors {
		if strings.Contains(e.Message, "ABC") {
			t.Errorf("mensagem vaza o valor: %s", e.Message)
		}
	}
	t.Setenv("DFE_GATE_SCHEMA_NFE_EVENT", "shadow")
	if got := GateEvent("nfe", body); got != nil {
		t.Errorf("shadow não recusa: %v", got)
	}
	if EventProblem("NF-e", nil) != nil {
		t.Error("sem achados não há erro")
	}
}
