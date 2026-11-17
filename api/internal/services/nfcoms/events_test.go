package nfcoms

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// As vinculações de pagamento (110300 e 110301) passam no schema do evento da NFCom (NT 2026.001, protocolo de 16 dígitos).
func TestPaymentLinkEnvelopesMatchEventSchema(t *testing.T) {
	p := testParams(t)
	ec := &eventContext{environment: 2, cnpj: testCNPJ, emitUF: "SP"}
	link := services.PaymentLink{Number: 1, TransactionID: "TX-123456", PaymentMethod: "17", ReceiverCNPJ: "12345678000190", PSPBaseCNPJ: "12345678"}
	cases := map[string]struct {
		tp  string
		det map[string]any
	}{
		"110300": {services.TpEventoVinculoPagamento, services.PaymentLinkDet(link, "1352600000000011")},
		"110301": {services.TpEventoCancVinculoPagamento, services.CancelPaymentLinkDet("1352600000000011", "1352600000000022")},
	}
	for name, c := range cases {
		body := buildEventEnvelope(ec, p.accessKey, c.tp, 1, c.det)
		if found := dfecheck.EventFindings("nfcom", body); len(found) != 0 {
			t.Errorf("%s: achados %+v", name, found)
		}
	}
}

// O envelope de cancelamento que o service monta passa no schema do evento (NT 2026.005).
func TestCancelEnvelopeMatchesEventSchema(t *testing.T) {
	p := testParams(t)
	ec := &eventContext{environment: 2, cnpj: testCNPJ, emitUF: "SP"}
	body := buildEventEnvelope(ec, p.accessKey, TpEventoCancelamento, 1, cancelDet("1352600000000011", "Erro de digitacao no valor"))
	if found := dfecheck.EventFindings("nfcom", body); len(found) != 0 {
		t.Fatalf("achados: %+v", found)
	}
	inf := body["eventoNFCom"].(map[string]any)["infEvento"].(map[string]any)
	if inf["@Id"] != "ID110111"+p.accessKey+"001" || inf["cOrgao"] != "35" || inf["chNFCom"] != p.accessKey {
		t.Errorf("infEvento = %v", inf)
	}
}

func TestEmitUFFromAccessKey(t *testing.T) {
	if got := emitUFFromAccessKey("35261100000000000000620010000000101012345671"); got != "SP" {
		t.Errorf("UF = %q", got)
	}
	if got := emitUFFromAccessKey("3"); got != "" {
		t.Errorf("chave curta devolveu %q", got)
	}
}

func TestOfflineResolution(t *testing.T) {
	if tp, off := resolveNFComEmission(nil); tp != tpEmisNormal || off {
		t.Errorf("sem configuração: %s %v", tp, off)
	}
}
