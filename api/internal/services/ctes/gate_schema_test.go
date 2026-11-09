package ctes

import (
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// O CFOP fora do padrão do XSD (achado M1 da auditoria) agora recusa no portão; o interruptor o devolve ao shadow.
func TestGateCTeSchemaBlocksPattern(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA_CTE", "block")
	build := func() map[string]any {
		body := BuildCTe(baseParams(t, RoleRemetente))
		body["CTe"].(map[string]any)["infCte"].(map[string]any)["ide"].(map[string]any)["CFOP"] = "5000"
		return body
	}
	err := gateCTe(build())
	pr, ok := err.(*problem.Problem)
	if !ok || pr.Code != dfecheck.CodeFiscalPrevalidation {
		t.Fatalf("want fiscal_prevalidation, got %v", err)
	}
	found := false
	for _, e := range pr.Errors {
		found = found || e.Field == "CTe/infCte/ide/CFOP" && e.Code == dfecheck.FieldCodeSchema
	}
	if !found {
		t.Fatalf("CFOP sem achado: %+v", pr.Errors)
	}
	t.Setenv("DFE_GATE_SCHEMA_CTE", "") // padrão: shadow
	if err := gateCTe(build()); err != nil {
		t.Fatalf("shadow não recusa: %v", err)
	}
}

func TestGateCTeEventSchema(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA_CTE_EVENT", "block")
	body := buildEventEnvelope(evCtx(), eventKey, TpEventoCancelamento, 1, "evCancCTe", cancelDet("12", "justificativa com mais de quinze caracteres"))
	if found := dfecheck.GateEvent("cte", body); len(found) == 0 {
		t.Fatal("nProt fora do padrão deveria recusar")
	}
	ok := buildEventEnvelope(evCtx(), eventKey, TpEventoCancelamento, 1, "evCancCTe", cancelDet(parityProt, "justificativa com mais de quinze caracteres"))
	if found := dfecheck.GateEvent("cte", ok); len(found) != 0 {
		t.Fatalf("evento válido recusado: %+v", found)
	}
}
