package mdfes

import (
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// O fixture baseParams tem regras do MOC próprias (CNPJ de teste), por isso o portão roda sem regras block.
func TestGateMDFeSchemaBlocksEnum(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA_MDFE", "block")
	build := func() map[string]any {
		p := baseParams(nil)
		p.tpEmis = tpEmisNormal
		body := BuildMDFe(p)
		body["MDFe"].(map[string]any)["infMDFe"].(map[string]any)["ide"].(map[string]any)["tpEmit"] = "9"
		return body
	}
	found := dfecheck.GateMDFe(build(), nil)
	hit := false
	for _, v := range found { // o fixture tem chaves de NF-e falsas (pattern); o que interessa é o enum
		hit = hit || v.Path == "MDFe/infMDFe/ide/tpEmit" && v.Code == "enum"
	}
	if !hit {
		t.Fatalf("enum de tpEmit deveria recusar: %+v", found)
	}
	pr, ok := gateMDFe(build()).(*problem.Problem)
	if !ok || pr.Code != dfecheck.CodeFiscalPrevalidation {
		t.Fatalf("want fiscal_prevalidation, got %v", pr)
	}
	t.Setenv("DFE_GATE_SCHEMA_MDFE", "off")
	if f := dfecheck.GateMDFe(build(), nil); len(f) != 0 {
		t.Fatalf("off não recusa: %+v", f)
	}
}

func TestGateMDFeEventSchema(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA_MDFE_EVENT", "block")
	ec := &eventContext{environment: 2, cnpj: "12345678000195", docTag: "CNPJ"}
	s := &MdfeService{}
	bad := s.buildEventEnvelope(ec, parityKey, TpEventoCancelamento, 1, cancelDet("12", "justificativa com mais de quinze caracteres"))
	if len(dfecheck.GateEvent("mdfe", bad)) == 0 {
		t.Fatal("nProt fora do padrão deveria recusar")
	}
	ok := s.buildEventEnvelope(ec, parityKey, TpEventoCancelamento, 1, cancelDet(parityProt, "justificativa com mais de quinze caracteres"))
	if f := dfecheck.GateEvent("mdfe", ok); len(f) != 0 {
		t.Fatalf("evento válido recusado: %+v", f)
	}
}
