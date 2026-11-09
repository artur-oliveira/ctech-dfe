package mdfes

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

const (
	parityKey  = "35251112345678000195580010000000071000000015"
	parityProt = "135250000000001"
)

func sp(s string) *string { return &s }

// Paridade builder x XSD dos eventos do MDF-e: o envelope eventoMDFe e o grupo evXxx de cada evento contra os
// specs gerados do pacote de schemas (go-dfe/validate/specs/evento_mdfe_*.json).
func TestEventBodies_ParidadeComXSD(t *testing.T) {
	ec := &eventContext{environment: 2, cnpj: "12345678000195", docTag: "CNPJ"}
	s := &MdfeService{}
	payments := []MdfePayment{
		{CPF: sp("12345678909"), Name: sp("Fulano"), Components: []MdfePaymentComponent{{Type: "01", Value: "100.00"}},
			ContractValue: "100.00", PaymentType: "0", Bank: MdfePaymentBank{PIX: sp("fulano@example.com")}},
		{CNPJ: sp("11222333000181"), Components: []MdfePaymentComponent{{Type: "99", Value: "10.00", Description: sp("outros")}},
			ContractValue: "10.00", PaymentType: "1", AdvanceValue: sp("5.00"), AdvanceRequest: sp("1"), AdvanceKind: sp("1"),
			Installments: []MdfePaymentInstallment{{Number: "001", DueDate: "2026-12-01", Value: "5.00"}},
			Bank:         MdfePaymentBank{BankCode: sp("001"), AgencyCode: sp("1234")}},
	}
	infPag, err := buildInfPag(payments)
	if err != nil {
		t.Fatal(err)
	}
	docs := []IncludeDFeDoc{{CMunDescarga: "3550308", XMunDescarga: "Sao Paulo", ChNFe: strings.Repeat("1", 44)}}
	dets := map[string]struct {
		tp  string
		det map[string]any
	}{
		"110111 cancelamento": {TpEventoCancelamento, cancelDet(parityProt, "justificativa com mais de quinze caracteres")},
		"110112 encerramento": {TpEventoEncerramento, closeDet(parityProt, "35", "3550308", false, time.Now())},
		"110112 por terceiro": {TpEventoEncerramento, closeDet(parityProt, "35", "3550308", true, time.Now())},
		"110114 condutor":     {TpEventoInclusaoCond, includeCondutorDet("Fulano de Tal", "123.456.789-09")},
		"110115 inclusao dfe": {TpEventoInclusaoDFe, includeDFeDet(parityProt, "3550308", "Sao Paulo", docs)},
		"110116 pagamento":    {TpEventoPagamentoOper, payTransportDet(parityProt, MdfePaymentTrips{Total: "00002", Number: "00001"}, infPag)},
		"110117 confirma":     {TpEventoConfirmaServico, confirmServiceDet(parityProt)},
		"110118 altera":       {TpEventoAlteracaoPagto, changePaymentDet(parityProt, infPag)},
	}
	for name, c := range dets {
		body := s.buildEventEnvelope(ec, parityKey, c.tp, 1, c.det)
		for _, v := range dfecheck.EventFindings("mdfe", body) {
			t.Errorf("%s: %s %s: %s", name, v.Path, v.Code, v.Message)
		}
	}
}

// B-01: qtdViagens/nroViagem sem zeros a esquerda ("1") violavam [0-9]{5} do XSD.
func TestEventBodies_PagamentoViagensSemZeros(t *testing.T) {
	ec := &eventContext{environment: 2, cnpj: "12345678000195", docTag: "CNPJ"}
	s := &MdfeService{}
	infPag, err := buildInfPag([]MdfePayment{{CPF: sp("12345678909"), Name: sp("Fulano"),
		Components: []MdfePaymentComponent{{Type: "01", Value: "100.00"}}, ContractValue: "100.00",
		PaymentType: "0", Bank: MdfePaymentBank{PIX: sp("fulano@example.com")}}})
	if err != nil {
		t.Fatal(err)
	}
	body := s.buildEventEnvelope(ec, parityKey, TpEventoPagamentoOper, 1,
		payTransportDet(parityProt, MdfePaymentTrips{Total: "1", Number: "1"}, infPag))
	for _, v := range dfecheck.EventFindings("mdfe", body) {
		t.Errorf("%s %s: %s", v.Path, v.Code, v.Message)
	}
}
