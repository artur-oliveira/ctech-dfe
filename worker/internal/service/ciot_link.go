package service

// ciot_link.go: ligação entre o MDF-e e os CIOTs (spec CIOT, C5). O vínculo nasce na emissão do MDF-e (api).
// Aqui o worker (1) oferece cancelar ou encerrar o CIOT quando o MDF-e é cancelado ou encerrado e (2) desfaz o
// vínculo quando o MDF-e é rejeitado. O cancelamento e o encerramento do CIOT NÃO são disparados sozinhos: exigem
// motivo, peso e rotas, dados que só o operador tem; a oferta fica na linha (mdfe_cascade) e na auditoria.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/alerts"
)

const (
	ciotMdfeIndex = "mdfe-index"
	attrMdfeKey   = "mdfe_access_key"
	attrCascade   = "mdfe_cascade"
	opCascadePfx  = "cascade:"
)

// linkedCiots lists the CIOT rows linked to the MDF-e of the message.
func (s *DfeService) linkedCiots(ctx context.Context, msg WorkerMessage) ([]map[string]types.AttributeValue, error) {
	var rows []map[string]types.AttributeValue
	var start map[string]types.AttributeValue
	for {
		out, err := s.dynamo.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.cfg.TablePrefix + "_ciots"),
			IndexName:              aws.String(ciotMdfeIndex),
			KeyConditionExpression: aws.String("pk = :pk AND " + attrMdfeKey + " = :k"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: msg.DocPK},
				":k":  &types.AttributeValueMemberS{Value: msg.AccessKey},
			},
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, fmt.Errorf("query linked ciots: %w", err)
		}
		rows = append(rows, out.Items...)
		if len(out.LastEvaluatedKey) == 0 {
			return rows, nil
		}
		start = out.LastEvaluatedKey
	}
}

func (s *DfeService) alertCiotLink(ctx context.Context, msg WorkerMessage, what string, err error) {
	slog.Error("ciot link failed", "what", what, "access_key", msg.AccessKey, "err", err)
	s.alerter.Alert(ctx, alerts.Alert{Job: "ciot-mdfe-link", Summary: "A CIOT linked to an MDF-e could not be updated after the MDF-e changed state.",
		Detail: fmt.Sprintf("mdfe: %s\naction: %s", msg.AccessKey, what), Err: err})
}

// offerCiotCascade marks each open CIOT of a cancelled (op cancel) or closed (op close) MDF-e. Idempotent: a row
// already marked with the same offer is skipped by the condition, so redeliveries append no second audit entry.
// Best effort: the MDF-e is already final, so a failure alerts instead of failing the message.
func (s *DfeService) offerCiotCascade(ctx context.Context, msg WorkerMessage, op string) {
	rows, err := s.linkedCiots(ctx, msg)
	if err != nil {
		s.alertCiotLink(ctx, msg, opCascadePfx+op, err)
		return
	}
	for _, row := range rows {
		sk := rowStr(row, "sk")
		st := rowStr(row, "status")
		if st != ciotDeclared && st != ciotRectified {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName:        aws.String(s.cfg.TablePrefix + "_ciots"),
			Key:              map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: msg.DocPK}, "sk": &types.AttributeValueMemberS{Value: sk}},
			UpdateExpression: aws.String("SET " + attrCascade + " = :op, updated_at = :ts"),
			ConditionExpression: aws.String("attribute_exists(pk) AND #mk = :k AND #st IN (:d, :r) AND (attribute_not_exists(" +
				attrCascade + ") OR " + attrCascade + " <> :op)"),
			ExpressionAttributeNames: map[string]string{"#mk": attrMdfeKey, "#st": "status"},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":op": &types.AttributeValueMemberS{Value: op}, ":ts": &types.AttributeValueMemberS{Value: now},
				":k": &types.AttributeValueMemberS{Value: msg.AccessKey},
				":d": &types.AttributeValueMemberS{Value: ciotDeclared}, ":r": &types.AttributeValueMemberS{Value: ciotRectified},
			},
		})
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			continue
		}
		if err != nil {
			s.alertCiotLink(ctx, msg, opCascadePfx+op, err)
			continue
		}
		s.auditCiot(ctx, msg, sk, opCascadePfx+op, map[string]any{attrMdfeKey: msg.AccessKey, "offer": op}, 0, "")
	}
}

// unlinkCiots drops the link of every CIOT bound to an MDF-e that was rejected or failed, so the CIOTs can go
// into a new MDF-e. Idempotent (the condition requires the key to still be this one).
func (s *DfeService) unlinkCiots(ctx context.Context, msg WorkerMessage) {
	rows, err := s.linkedCiots(ctx, msg)
	if err != nil {
		s.alertCiotLink(ctx, msg, "unlink", err)
		return
	}
	for _, row := range rows {
		sk := rowStr(row, "sk")
		_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName:                aws.String(s.cfg.TablePrefix + "_ciots"),
			Key:                      map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: msg.DocPK}, "sk": &types.AttributeValueMemberS{Value: sk}},
			UpdateExpression:         aws.String("SET updated_at = :ts REMOVE #mk, " + attrCascade),
			ConditionExpression:      aws.String("#mk = :k"),
			ExpressionAttributeNames: map[string]string{"#mk": attrMdfeKey},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":ts": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
				":k":  &types.AttributeValueMemberS{Value: msg.AccessKey},
			},
		})
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			continue
		}
		if err != nil {
			s.alertCiotLink(ctx, msg, "unlink", err)
			continue
		}
		s.auditCiot(ctx, msg, sk, "unlink", map[string]any{attrMdfeKey: msg.AccessKey}, 0, "")
	}
}

// unlinkCiotsOfMdfe is unlinkCiots for the MDF-e authorization path only.
func (s *DfeService) unlinkCiotsOfMdfe(ctx context.Context, msg WorkerMessage) {
	if msg.DocType == docTypeMDFe {
		s.unlinkCiots(ctx, msg)
	}
}
