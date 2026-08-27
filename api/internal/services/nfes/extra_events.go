package nfes

// extra_events.go implements the NF-e events that are neither cancellation,
// CC-e nor manifestação:
//
//	111500 / 111501 — Pedido de Prorrogação (1º e 2º prazo)
//	111502 / 111503 — Cancelamento do Pedido de Prorrogação
//	110001          — Cancelamento de Evento
//
// Caso de uso da prorrogação: remessa para industrialização, cuja suspensão do
// ICMS tem prazo. O pedido é por item da NF-e (quantidade a prorrogar), não pela
// nota inteira.

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	// Prorrogação do prazo de ICMS suspenso (remessa para industrialização).
	TpEventoProrrogacao1     = "111500"
	TpEventoProrrogacao2     = "111501"
	TpEventoCancProrrogacao1 = "111502"
	TpEventoCancProrrogacao2 = "111503"

	// Cancelamento de evento (novo no PL_010e). Só cancela eventos da reforma
	// tributária (1121xx/2111xx/2121xx/4121xx) — a lista `tpEventoAut` do XSD.
	TpEventoCancelamentoEvento = "110001"

	descProrrogacao     = "Pedido de Prorrogação"
	descCancProrrogacao = "Cancelamento de Pedido de Prorrogacao"
	descCancEvento      = "Cancelamento de Evento"
)

// prorrogacaoEvents / cancProrrogacaoEvents bound the accepted event codes so a
// caller cannot ask for an arbitrary tpEvento.
var prorrogacaoEvents = map[string]bool{
	TpEventoProrrogacao1: true, TpEventoProrrogacao2: true,
}

var cancProrrogacaoEvents = map[string]bool{
	TpEventoCancProrrogacao1: true, TpEventoCancProrrogacao2: true,
}

// cancelableEventTypes mirrors the `tpEventoAut` enumeration of e110001_v1.00:
// only reforma-tributária events can be cancelled by event 110001.
var cancelableEventTypes = map[string]bool{
	"112110": true, "112120": true, "112130": true, "112140": true, "112150": true,
	"211110": true, "211120": true, "211124": true, "211128": true, "211130": true,
	"211140": true, "211150": true, "212110": true, "212120": true,
	"412120": true, "412130": true,
}

// ProrrogationItem is one item of the NF-e whose suspended-ICMS deadline is
// being extended (`itemPedido`).
type ProrrogationItem struct {
	ItemNumber int    `json:"item_number" validate:"required,gte=1,lte=990"` // @numItem
	Quantity   string `json:"quantity" validate:"required,decimalv"`         // qtdeItem
}

// RequestProrrogation dispatches a Pedido de Prorrogação (111500 = 1º prazo,
// 111501 = 2º prazo).
func (s *NfeService) RequestProrrogation(
	ctx context.Context, orgPK, accessKey, eventType string,
	items []ProrrogationItem, seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	if !prorrogacaoEvents[eventType] {
		return nil, problem.BadRequest("evento de prorrogação inválido: " + eventType)
	}
	if len(items) == 0 {
		return nil, problem.BadRequest("informe ao menos um item para prorrogação")
	}
	nfe, ectx, nProt, err := s.resolveAuthorizedEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}

	itemPedido := make([]map[string]any, 0, len(items))
	for _, it := range items {
		itemPedido = append(itemPedido, map[string]any{
			"@numItem": fmt.Sprintf("%d", it.ItemNumber),
			"qtdeItem": it.Quantity,
		})
	}
	body := buildDetEventoBody(accessKey, ectx, eventType, seq, map[string]any{
		"descEvento": descProrrogacao,
		"nProt":      nProt,
		"itemPedido": itemPedido,
	})
	return s.dispatchExtraEvent(ctx, ectx, nfe, accessKey, eventType, seq, body, userID, userName)
}

// CancelProrrogation dispatches a Cancelamento de Pedido de Prorrogação
// (111502 cancels 111500, 111503 cancels 111501). requestID is the `Id` of the
// prorrogation event being cancelled (`idPedidoCancelado`).
func (s *NfeService) CancelProrrogation(
	ctx context.Context, orgPK, accessKey, eventType, requestID string,
	seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	if !cancProrrogacaoEvents[eventType] {
		return nil, problem.BadRequest("evento de cancelamento de prorrogação inválido: " + eventType)
	}
	if requestID == "" {
		return nil, problem.BadRequest("informe o Id do pedido de prorrogação a cancelar")
	}
	nfe, ectx, nProt, err := s.resolveAuthorizedEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	body := buildDetEventoBody(accessKey, ectx, eventType, seq, map[string]any{
		"descEvento":        descCancProrrogacao,
		"idPedidoCancelado": requestID,
		"nProt":             nProt,
	})
	return s.dispatchExtraEvent(ctx, ectx, nfe, accessKey, eventType, seq, body, userID, userName)
}

// CancelEvent dispatches a Cancelamento de Evento (110001). `tpEventoAut` is the
// event being cancelled and `nProtEvento` its authorization protocol — not the
// NF-e's. Only reforma-tributária events are cancelable (XSD enumeration).
func (s *NfeService) CancelEvent(
	ctx context.Context, orgPK, accessKey, cancelledEventType, cancelledProtocol string,
	seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	if !cancelableEventTypes[cancelledEventType] {
		return nil, problem.BadRequest(
			"apenas eventos da reforma tributária podem ser cancelados pelo evento 110001: " + cancelledEventType)
	}
	if cancelledProtocol == "" {
		return nil, problem.BadRequest("informe o protocolo do evento a cancelar")
	}
	nfe, ectx, _, err := s.resolveAuthorizedEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	body := buildDetEventoBody(accessKey, ectx, TpEventoCancelamentoEvento, seq, map[string]any{
		"descEvento":  descCancEvento,
		"cOrgaoAutor": accessKey[:2],
		"verAplic":    s.tech.Version,
		"tpEventoAut": cancelledEventType,
		"nProtEvento": cancelledProtocol,
	})
	return s.dispatchExtraEvent(ctx, ectx, nfe, accessKey, TpEventoCancelamentoEvento, seq, body, userID, userName)
}

// resolveAuthorizedEvent loads an authorized NF-e plus its event context and
// authorization protocol — the preamble every event here shares.
func (s *NfeService) resolveAuthorizedEvent(ctx context.Context, orgPK, accessKey string) (
	map[string]types.AttributeValue, *nfeEventContext, string, error,
) {
	nfe, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, nil, "", err
	}
	if nfe == nil {
		return nil, nil, "", ErrNFeNotFound
	}
	if strAttr(nfe, "status") != StatusAuthorized {
		return nil, nil, "", problem.BadRequest("apenas NF-e autorizadas aceitam este evento")
	}
	nProt := strAttr(nfe, "sefaz_protocol")
	if nProt == "" {
		return nil, nil, "", problem.BadRequest("protocolo de autorização não encontrado na NF-e")
	}
	ectx, err := resolveEventContext(ctx, s.orgRepo, s.certRepo, orgPK, nfe)
	if err != nil {
		return nil, nil, "", err
	}
	return nfe, ectx, nProt, nil
}

// buildDetEventoBody wraps a detEvento in the shared envEvento envelope.
func buildDetEventoBody(accessKey string, ectx *nfeEventContext, eventType string, seq int, det map[string]any) map[string]any {
	det["@versao"] = eventVersao
	det["@xmlns"] = nfeXMLNS
	return map[string]any{
		"envEvento": map[string]any{
			"@versao": eventVersao,
			"@xmlns":  nfeXMLNS,
			"idLote":  sefazBatchID(),
			"evento": map[string]any{
				"@versao": eventVersao,
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", eventType, accessKey, seq),
					"cOrgao":     accessKey[:2],
					"tpAmb":      fmt.Sprintf("%d", ectx.environment),
					ectx.docTag:  ectx.cnpj,
					"chNFe":      accessKey,
					"dhEvento":   dhEvento(),
					"tpEvento":   eventType,
					"nSeqEvento": fmt.Sprintf("%d", seq),
					"verEvento":  eventVersao,
					"detEvento":  det,
				},
			},
		},
	}
}

// dispatchExtraEvent records the event and publishes it to the SEFAZ worker.
// These events never change the document status.
func (s *NfeService) dispatchExtraEvent(
	ctx context.Context, ectx *nfeEventContext, nfe map[string]types.AttributeValue,
	accessKey, eventType string, seq int, body map[string]any, userID, userName string,
) (map[string]types.AttributeValue, error) {
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, seq, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ectx.pk, AccessKey: accessKey,
		TableName: "nfes", S3Prefix: "nfe",
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, seq),
		CNPJ:             ectx.cnpj, UF: ectx.emitUF,
		SefazEnvironment: ectx.sefazEnv,
		CertS3Key:        ectx.cert.s3Key, CertPassword: ectx.cert.password,
		DocType: "nfe", SefazService: "RecepcaoEvento",
		Body:            body,
		EventsTableName: aws.String("nfe_events"),
		EventType:       aws.String(eventType),
		SequenceNumber:  &seq,
		EventSK:         aws.String(strAttr(event, "sk")),
	}); err != nil {
		return nil, err
	}
	return nfe, nil
}
