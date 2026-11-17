package bpes

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// protocolRe é o nProt do BP-e (XSD do evento: 15 dígitos).
var protocolRe = regexp.MustCompile(`^\d{15}$`)

// RegisterPaymentLink registra a vinculação de pagamento (110300) de um BP-e autorizado. seq 0 usa o próximo número.
func (s *BpeService) RegisterPaymentLink(ctx context.Context, orgPK, accessKey string, p services.PaymentLink, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := p.Check(); err != nil {
		return nil, err
	}
	// Tenant primeiro: a tabela de eventos é chaveada só pela chave de acesso.
	if _, err := s.resolveEventContext(ctx, orgPK, accessKey); err != nil {
		return nil, err
	}
	seq, err := services.NextEventSequence(ctx, s.eventRepo, accessKey, services.TpEventoVinculoPagamento, seq)
	if err != nil {
		return nil, err
	}
	return s.send(ctx, orgPK, accessKey, services.TpEventoVinculoPagamento, seq, "", userID, userName,
		func(nProt string) map[string]any { return services.PaymentLinkDet(p, nProt) })
}

// CancelPaymentLink cancela um 110300 (110301) escolhido por event_sk; o protocolo dele vem do registro.
func (s *BpeService) CancelPaymentLink(ctx context.Context, orgPK, accessKey, eventSK string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	// O documento precisa ser do tenant antes de ler a tabela de eventos, chaveada só pela chave de acesso.
	if _, err := s.resolveEventContext(ctx, orgPK, accessKey); err != nil {
		return nil, err
	}
	linkProt, err := services.AuthorizedEventProtocol(ctx, s.eventRepo, accessKey, eventSK, services.TpEventoVinculoPagamento, protocolRe)
	if err != nil {
		return nil, err
	}
	if seq, err = services.NextEventSequence(ctx, s.eventRepo, accessKey, services.TpEventoCancVinculoPagamento, seq); err != nil {
		return nil, err
	}
	return s.send(ctx, orgPK, accessKey, services.TpEventoCancVinculoPagamento, seq, "", userID, userName,
		func(nProt string) map[string]any { return services.CancelPaymentLinkDet(nProt, linkProt) })
}
