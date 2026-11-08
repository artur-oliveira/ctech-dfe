package nfes

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

const (
	parityKey  = "35260711222333000181550010000000011600000010"
	parityProt = "135260000000001"
	parityJust = "justificativa com mais de quinze caracteres"
	parityDec  = "10.00"
	parityVer  = "dfe-1.0"
	parityCNPJ = "11222333000181"
)

// failOnFindings falha o teste com cada achado do esquema de evento (tag que o XSD não conhece, obrigatória
// ausente, tamanho, padrão ou enumeração). Sem filtro: o corpo do evento é completo.
func failOnFindings(t *testing.T, name, doc string, body map[string]any) {
	t.Helper()
	for _, v := range dfecheck.EventFindings(doc, body) {
		t.Errorf("%s: %s %s: %s", name, v.Path, v.Code, v.Message)
	}
}

func wrapDet(tp string, det map[string]any) map[string]any {
	ectx := &nfeEventContext{environment: 2, cnpj: parityCNPJ, docTag: services.TagCNPJ}
	return buildDetEventoBody(parityKey, ectx, tp, 1, det)
}

// Paridade builder x XSD dos eventos da NF-e: cada corpo é conferido contra o spec gerado do XSD do evento
// (go-dfe/validate/specs/evento_nfe_*.json) e contra o envelope envEvento.
func TestEventBodies_ParidadeComXSD(t *testing.T) {
	bodies := map[string]map[string]any{
		"110111 cancelamento":  buildCancelBody(parityKey, services.TagCNPJ, parityCNPJ, 2, parityProt, parityJust, 1),
		"110110 cce":           buildCCeBody(parityKey, services.TagCNPJ, parityCNPJ, 2, "correcao com mais de quinze caracteres", 1),
		"110110 cce MT":        buildCCeBody("51"+parityKey[2:], services.TagCNPJ, parityCNPJ, 2, "correcao com mais de quinze caracteres", 1),
		"110112 substituicao":  buildSubstituteBody(parityKey, services.TagCNPJ, parityCNPJ, 2, parityProt, parityKey, parityJust, 1, parityVer),
		"210240 nao realizada": buildManifestBody(parityKey, services.TagCNPJ, parityCNPJ, 2, TpEventoNaoRealizacao, 1, ptr(parityJust)),
		"210210 ciencia":       buildManifestBody(parityKey, services.TagCNPJ, parityCNPJ, 2, TpEventoCienciaOperacao, 1, nil),
		"210200 confirmacao":   buildManifestBody(parityKey, services.TagCNPJ, parityCNPJ, 2, TpEventoConfirmacaoOperacao, 1, nil),
		"210220 desconhece":    buildManifestBody(parityKey, services.TagCNPJ, parityCNPJ, 2, TpEventoDesconhecimento, 1, ptr(parityJust)),
		"111500 prorrogacao": wrapDet(TpEventoProrrogacao1, prorrogationDet(parityProt,
			[]ProrrogationItem{{ItemNumber: 1, Quantity: "2"}, {ItemNumber: 2, Quantity: "1.5"}})),
		"111502 canc prorrogacao": wrapDet(TpEventoCancProrrogacao1, cancProrrogationDet("ID"+strings.Repeat("1", 52), parityProt)),
		"110001 canc evento":      wrapDet(TpEventoCancelamentoEvento, cancEventDet(parityKey, parityVer, "112110", parityProt)),
		"110140 epec": buildEPECBody(services.TagCNPJ, parityCNPJ, 2, EPECSummary{
			AccessKey: parityKey, VerAplic: parityVer, IssuedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
			TpNF: "1", IE: "123456789", DestUF: "SP", DestTag: services.TagCNPJ, DestDoc: "99888777000166",
			VNF: parityDec, VICMS: parityDec, VST: "0.00",
		}, 1),
	}
	for name, body := range bodies {
		failOnFindings(t, name, "nfe", body)
	}
}

func ptr(s string) *string { return &s }

// Eventos da reforma: o detEvento que reformEvents monta, completo com os campos comuns.
func TestReformEventBodies_ParidadeComXSD(t *testing.T) {
	on, off := 1, 0
	item := ReformItem{
		ItemNumber: 1, IBS: parityDec, CBS: parityDec, Quantity: "2.5000", Unit: "UN", CreditIBS: parityDec, CreditCBS: parityDec,
		BaseValue: parityDec, PresumedCode: "01",
		IBSPresumed: &PresumedCredit{Code: "01", Percent: "10.00", Value: parityDec},
		CBSPresumed: &PresumedCredit{Code: "01", Percent: "10.00", Value: parityDec},
	}
	second := item
	second.ItemNumber = 2
	for tp, spec := range reformEvents {
		req := ReformEventRequest{EventType: tp, Indicator: &on, DeliveryForecast: "2026-12-31", Items: []ReformItem{item, second}}
		if tp == TpEventoAceiteDebito || tp == TpEventoSucessaoIBS || tp == TpEventoSucessaoCBS {
			req.Indicator = &off
		}
		det, err := spec.build(req)
		if err != nil {
			t.Fatalf("%s: %v", tp, err)
		}
		completeReformDet(det, spec.desc, "35", spec.authors[0], parityVer)
		failOnFindings(t, tp, "nfe", wrapDet(tp, det))
	}
}
