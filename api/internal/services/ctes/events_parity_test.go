package ctes

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

const (
	parityProt = "135250000000001"
	parityHash = "AAAAAAAAAAAAAAAAAAAAAAAAAAA="
)

// Paridade builder x XSD dos eventos do CT-e: o envelope eventoCTe e o grupo evXxx de cada evento contra os
// specs gerados do pacote de schemas (go-dfe/validate/specs/evento_cte_*.json).
func TestEventBodies_ParidadeComXSD(t *testing.T) {
	at := time.Date(2026, 7, 1, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	proof := DeliveryProof{OccurredAt: at, HashAt: at, Hash: parityHash, Latitude: "-23.550520", Longitude: "-46.633308",
		NFeKeys: []string{strings.Repeat("1", 44), strings.Repeat("2", 44)}}
	bodies := map[string]map[string]any{
		"110111 cancelamento": buildEventEnvelope(evCtx(), eventKey, TpEventoCancelamento, 1, "evCancCTe", cancelDet(parityProt, " justificativa com mais de quinze caracteres ")),
		"110110 cce": buildEventEnvelope(evCtx(), eventKey, TpEventoCCe, 2, "evCCeCTe", cceDet([]map[string]any{
			{"grupoAlterado": "rem", "campoAlterado": "xNome", "valorAlterado": "X", "nroItemAlterado": "1"}})),
		"610110 desacordo":      buildEventEnvelope(evCtx(), eventKey, TpEventoDesacordo, 1, "evPrestDesacordo", disagreementDet("observacao do tomador")),
		"610111 canc desacordo": buildEventEnvelope(evCtx(), eventKey, TpEventoCancDesacordo, 1, "evCancPrestDesacordo", cancDisagreementDet(parityProt)),
		"110180 comprovante": buildEventEnvelope(evCtx(), eventKey, TpEventoComprovanteEntrega, 1, "evCECTe",
			receiptDet(DeliveryReceipt{DeliveryProof: proof, DocNumber: "123456", Name: "Fulano de Tal"}, parityProt)),
		"110190 insucesso": buildEventEnvelope(evCtx(), eventKey, TpEventoInsucessoEntrega, 1, "evIECTe",
			failureDet(DeliveryFailure{DeliveryProof: proof, Attempt: 2, ReasonType: tpMotivoOutros}, parityProt, "destinatario ausente no local")),
		"110181 canc comprovante": buildEventEnvelope(evCtx(), eventKey, TpEventoCancComprovanteEntrega, 1, "evCancCECTe",
			map[string]any{"descEvento": descCancComprovanteEntrega, "nProt": parityProt, "nProtCE": parityProt}),
		"110191 canc insucesso": buildEventEnvelope(evCtx(), eventKey, TpEventoCancInsucessoEntrega, 1, "evCancIECTe",
			map[string]any{"descEvento": descCancInsucessoEntrega, "nProt": parityProt, "nProtIE": parityProt}),
		"110300 vinculo pgto": buildEventEnvelope(evCtx(), eventKey, TpEventoVinculoPagamento, 1, "evVincPgto", paymentLinkDet(validLink(), parityProt)),
		"110301 canc vinculo": buildEventEnvelope(evCtx(), eventKey, TpEventoCancVinculoPagamento, 1, "evCancVincPgto", cancPaymentLinkDet(parityProt, parityProt)),
	}
	for name, body := range bodies {
		for _, v := range dfecheck.EventFindings("cte", body) {
			t.Errorf("%s: %s %s: %s", name, v.Path, v.Code, v.Message)
		}
	}
}
