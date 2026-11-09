package nfes

import (
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// Gate de schema em block (NF-e, NFC-e e eventos): o documento válido do builder passa; padrão e enum fora do XSD recusam.
func TestGateSchemaBlockNFeNFCe(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA", "block")
	for _, model := range []string{nfModel55, nfModel65} {
		if errs := schemaErrs(f3Doc{model: model}.build(), model); len(errs) != 0 {
			t.Fatalf("modelo %s válido recusado: %+v", model, errs)
		}
		body := f3Doc{model: model}.build()
		ide := f3Inf(body)["ide"].(map[string]any)
		ide["tpImp"] = "9"
		ide["cUF"] = "2"
		errs := schemaErrs(body, model)
		if !hasField(errs, "NFe/infNFe/ide/tpImp") || !hasField(errs, "NFe/infNFe/ide/cUF") {
			t.Errorf("modelo %s: enum e padrão deviam recusar: %+v", model, errs)
		}
		for _, e := range errs {
			if e.Code != dfecheck.FieldCodeSchema && e.Code != dfecheck.FieldCodeFiscalRule &&
				e.Code != dfecheck.FieldCodeTooLong && e.Code != dfecheck.FieldCodeTooShort {
				t.Errorf("code inesperado: %s", e.Code)
			}
		}
	}
}

// schemaErrs são os erros do portão que não vêm de regra do MOC (o fixture f3 tem regras de pagamento próprias).
func schemaErrs(body map[string]any, model string) []problem.FieldError {
	var out []problem.FieldError
	for _, e := range gateErrs(body, model) {
		if e.Code != dfecheck.FieldCodeFiscalRule {
			out = append(out, e)
		}
	}
	return out
}

// Padrão shadow: nada de schema recusa (só o tamanho, como antes).
func TestGateSchemaDefaultShadowDoesNotRefuse(t *testing.T) {
	for _, model := range []string{nfModel55, nfModel65} {
		body := f3Doc{model: model}.build()
		f3Inf(body)["ide"].(map[string]any)["tpImp"] = "9"
		if errs := schemaErrs(body, model); len(errs) != 0 {
			t.Errorf("modelo %s: shadow recusou: %+v", model, errs)
		}
	}
	if _, err := gateNFeEvent(parityKey, buildCancelBody(parityKey, services.TagCNPJ, parityCNPJ, 2, "abc", parityJust, 1)); err != nil {
		t.Errorf("evento em shadow recusou: %v", err)
	}
}

func TestGateSchemaNFeKillSwitch(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA", "block")
	t.Setenv("DFE_GATE_SCHEMA_NFE", "shadow")
	body := f3Doc{}.build()
	f3Inf(body)["ide"].(map[string]any)["tpImp"] = "9"
	if errs := schemaErrs(body, nfModel55); len(errs) != 0 {
		t.Fatalf("shadow não recusa: %+v", errs)
	}
	// NFC-e tem o interruptor próprio e continua em block.
	nb := f3Doc{model: nfModel65}.build()
	f3Inf(nb)["ide"].(map[string]any)["tpImp"] = "9"
	if errs := schemaErrs(nb, nfModel65); len(errs) == 0 {
		t.Fatal("NFC-e segue em block")
	}
}

func TestGateNFeEventSchema(t *testing.T) {
	t.Setenv("DFE_GATE_SCHEMA_NFE_EVENT", "block")
	ok := buildCancelBody(parityKey, services.TagCNPJ, parityCNPJ, 2, parityProt, parityJust, 1)
	if _, err := gateNFeEvent(parityKey, ok); err != nil {
		t.Fatalf("evento válido recusado: %v", err)
	}
	bad := buildCancelBody(parityKey, services.TagCNPJ, parityCNPJ, 2, "abc", parityJust, 1)
	if _, err := gateNFeEvent(parityKey, bad); err == nil {
		t.Fatal("nProt fora do padrão deveria recusar")
	}
}
