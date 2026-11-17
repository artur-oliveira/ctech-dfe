package nfcoms

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// protocolRe é o nProt da NFCom (XSD: 16 dígitos).
var protocolRe = regexp.MustCompile(`^\d{16}$`)

// RegisterPaymentLink registra a vinculação de pagamento (110300) de uma NFCom autorizada. seq 0 usa o próximo número.
func (s *NfcomService) RegisterPaymentLink(ctx context.Context, orgPK, accessKey string, p services.PaymentLink, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := p.Check(); err != nil {
		return nil, err
	}
	ec, nProt, err := s.paymentEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if seq, err = services.NextEventSequence(ctx, s.eventRepo, accessKey, services.TpEventoVinculoPagamento, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, services.TpEventoVinculoPagamento, seq, services.PaymentLinkDet(p, nProt))
	return s.dispatchEvent(ctx, ec, accessKey, services.TpEventoVinculoPagamento, seq, body, "", userID, userName)
}

// CancelPaymentLink cancela um 110300 (110301) escolhido por event_sk; o protocolo dele vem do registro.
func (s *NfcomService) CancelPaymentLink(ctx context.Context, orgPK, accessKey, eventSK string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	ec, nProt, err := s.paymentEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	linkProt, err := services.AuthorizedEventProtocol(ctx, s.eventRepo, accessKey, eventSK, services.TpEventoVinculoPagamento, protocolRe)
	if err != nil {
		return nil, err
	}
	if seq, err = services.NextEventSequence(ctx, s.eventRepo, accessKey, services.TpEventoCancVinculoPagamento, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, services.TpEventoCancVinculoPagamento, seq, services.CancelPaymentLinkDet(nProt, linkProt))
	return s.dispatchEvent(ctx, ec, accessKey, services.TpEventoCancVinculoPagamento, seq, body, "", userID, userName)
}

func (s *NfcomService) paymentEventContext(ctx context.Context, orgPK, accessKey string) (*eventContext, string, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, "", err
	}
	nProt := strAttr(ec.doc, "sefaz_protocol")
	if nProt == "" {
		return nil, "", problem.BadRequest("protocolo de autorização não encontrado na NFCom")
	}
	return ec, nProt, nil
}
