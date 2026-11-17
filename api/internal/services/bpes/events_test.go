package bpes

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// Cada envelope de evento que o service monta passa no schema do evento (NT 2026.005).
func TestEventEnvelopesMatchEventSchemas(t *testing.T) {
	p := testParams(t)
	ec := &eventContext{environment: 2, cnpj: testCNPJ, emitUF: "SP"}
	nProt := "135260000000001" // 15 dígitos
	cases := []struct {
		name, tpEvento string
		det            map[string]any
	}{
		{"cancelamento", TpEventoCancelamento, cancelDet(nProt, "Erro de digitacao no valor")},
		{"não embarque", TpEventoNaoEmbarque, noShowDet(nProt, "Passageiro nao compareceu ao embarque")},
		{"alteração de poltrona", TpEventoAlteracaoPoltrona, seatChangeDet(nProt, "15")},
		{"excesso de bagagem", TpEventoExcessoBagagem, excessBaggageDet(nProt, "12 kg", "30.00")},
		{"vinculação de pagamento", services.TpEventoVinculoPagamento, services.PaymentLinkDet(
			services.PaymentLink{Number: 1, TransactionID: "TX-123456", PaymentMethod: "17", ReceiverCNPJ: "12345678000190", PSPBaseCNPJ: "12345678"}, nProt)},
		{"cancelamento da vinculação", services.TpEventoCancVinculoPagamento, services.CancelPaymentLinkDet(nProt, "135260000000002")},
	}
	for _, c := range cases {
		body := buildEventEnvelope(ec, p.accessKey, c.tpEvento, 1, c.det)
		if found := dfecheck.EventFindings("bpe", body); len(found) != 0 {
			t.Errorf("%s: achados %+v", c.name, found)
		}
		inf := body["eventoBPe"].(map[string]any)["infEvento"].(map[string]any)
		if inf["@Id"] != "ID"+c.tpEvento+p.accessKey+"001" || inf["chBPe"] != p.accessKey || inf["cOrgao"] != "35" {
			t.Errorf("%s: infEvento = %v", c.name, inf)
		}
	}
}

// Elegibilidade do evento por variante (MOC I20a a I20c): TM não aceita não embarque, excesso de bagagem; TA não aceita
// alteração de poltrona nem excesso de bagagem; o comum aceita os quatro.
func TestEventEligibilityByVariant(t *testing.T) {
	cases := []struct {
		variant, tpEvento string
		want              bool
	}{
		{VariantNormal, TpEventoCancelamento, true},
		{VariantNormal, TpEventoNaoEmbarque, true},
		{VariantNormal, TpEventoAlteracaoPoltrona, true},
		{VariantNormal, TpEventoExcessoBagagem, true},
		{VariantTM, TpEventoCancelamento, true},
		{VariantTM, TpEventoNaoEmbarque, false},
		{VariantTM, TpEventoExcessoBagagem, false},
		{VariantTM, TpEventoAlteracaoPoltrona, true},
		{VariantTA, TpEventoCancelamento, true},
		{VariantTA, TpEventoNaoEmbarque, true},
		{VariantTA, TpEventoAlteracaoPoltrona, false},
		{VariantTA, TpEventoExcessoBagagem, false},
		{VariantNormal, services.TpEventoVinculoPagamento, true},
		{VariantTM, services.TpEventoCancVinculoPagamento, true},
		{VariantTA, services.TpEventoVinculoPagamento, true},
	}
	for _, c := range cases {
		if got := eventAllowed(c.variant, c.tpEvento); got != c.want {
			t.Errorf("%s/%s = %v, quer %v", c.variant, c.tpEvento, got, c.want)
		}
	}
}

func TestEmitUFFromAccessKey(t *testing.T) {
	if got := emitUFFromAccessKey("35261100000000000000630010000000101012345671"); got != "SP" {
		t.Errorf("UF = %q", got)
	}
	if got := emitUFFromAccessKey("3"); got != "" {
		t.Errorf("chave curta devolveu %q", got)
	}
}
