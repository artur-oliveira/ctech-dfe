package nfes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

func operationMap(fields map[string]any) map[string]any {
	m := map[string]any{"pk": "CNPJ_1", "sk": "OPERATION_1", "name": "Venda para revenda"}
	for k, v := range fields {
		m[k] = v
	}
	return m
}

// A escada inteira desta fase: request > operação > default.
func TestFirstNonNil_RequestWinsOverOperation(t *testing.T) {
	fromRequest := "9"
	op := operationMap(map[string]any{opFieldIndPres: "1"})

	if got := firstNonNil(&fromRequest, operationDefault(op, opFieldIndPres)); *got != "9" {
		t.Errorf("got %q, esperado 9 — request vence a operação", *got)
	}
	if got := firstNonNil(nil, operationDefault(op, opFieldIndPres)); *got != "1" {
		t.Errorf("got %q, esperado 1 — sem valor no request, a operação preenche", *got)
	}
	if got := firstNonNil(nil, operationDefault(nil, opFieldIndPres)); got != nil {
		t.Errorf("got %v, esperado nil — sem request e sem operação", *got)
	}
	// String vazia conta como ausência: um campo em branco não pode vencer o
	// default e ir vazio para o XML.
	empty := ""
	if got := firstNonNil(&empty, operationDefault(op, opFieldIndPres)); *got != "1" {
		t.Errorf("got %q, esperado 1 — string vazia não vence a operação", *got)
	}
}

func TestResolveItemCFOP(t *testing.T) {
	op := operationMap(map[string]any{opFieldCfopSuffix: "102"})

	// CFOP explícito no item vence sempre.
	if got, err := resolveItemCFOP("5405", op, "SP", "MG"); err != nil || got != "5405" {
		t.Errorf("got %q, err %v — CFOP do item tem que vencer", got, err)
	}
	// Sem CFOP, resolve pelo escopo.
	if got, err := resolveItemCFOP("", op, "SP", "SP"); err != nil || got != "5102" {
		t.Errorf("got %q, err %v — esperado 5102", got, err)
	}
	if got, err := resolveItemCFOP("", op, "SP", "MG"); err != nil || got != "6102" {
		t.Errorf("got %q, err %v — esperado 6102", got, err)
	}
}

// Sem CFOP e sem operação, a mensagem tem que continuar dizendo que o CFOP é
// obrigatório — quem já emitia assim não pode ver uma mensagem nova e confusa.
func TestResolveItemCFOP_WithoutOperationSaysCfopIsRequired(t *testing.T) {
	_, err := resolveItemCFOP("", nil, "SP", "SP")
	if err == nil {
		t.Fatal("esperado erro")
	}
	if !strings.Contains(err.Error(), "cfop é obrigatório") {
		t.Errorf("mensagem = %q", err.Error())
	}
}

// Operação sem cfop_suffix é o mesmo que não ter operação para efeito de CFOP.
func TestResolveItemCFOP_OperationWithoutSuffix(t *testing.T) {
	if _, err := resolveItemCFOP("", operationMap(nil), "SP", "SP"); err == nil {
		t.Fatal("esperado erro: a operação não define cfop_suffix")
	}
}

func TestInterpolateOperationText(t *testing.T) {
	op := operationMap(map[string]any{opFieldInfCpl: "Total {{v_nf}} — {{cliente}}"})
	got, err := interpolateOperationText(op, opFieldInfCpl, map[string]string{
		"v_nf": "100,00", "cliente": "ACME",
	})
	if err != nil {
		t.Fatalf("interpolateOperationText: %v", err)
	}
	if got == nil || *got != "Total 100,00 — ACME" {
		t.Errorf("got %v", got)
	}

	// Operação sem o campo não gera texto — nada de campo em branco no XML.
	if got, err := interpolateOperationText(operationMap(nil), opFieldInfCpl, nil); err != nil || got != nil {
		t.Errorf("got %v, err %v — esperado nil", got, err)
	}
}

func TestOperationObsInterpola(t *testing.T) {
	op := map[string]any{"obs_cont": []any{
		map[string]any{"x_campo": "Cliente", "x_texto": "Venda para {{cliente}}"},
		map[string]any{"x_campo": "Vazio", "x_texto": ""},
	}}
	got := operationObs(op, opFieldObsCont, map[string]string{services.PlaceholderCliente: "ACME"})
	if len(got) != 1 {
		t.Fatalf("observação vazia deveria ser descartada: %v", got)
	}
	if got[0]["@xCampo"] != "Cliente" || got[0]["xTexto"] != "Venda para ACME" {
		t.Fatalf("interpolação errada: %v", got[0])
	}
}

func TestOperationObsSemOperacaoDevolveNil(t *testing.T) {
	if got := operationObs(nil, opFieldObsCont, nil); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}
