package service

// epec.go: the worker side of the NF-e EPEC (event 110140; docs/specs/2026-11-08-contingencia.md section 9).
// The API stores the signed NF-e as contingency_awaiting and queues the EPEC event. Only an event that
// ended registered (success, or the duplicate 573) arms the document for the retransmission sweep; a
// rejection (or a terminal failure) leaves the document rejected and never armed.

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/contingency"
)

// epecEvent is the NF-e event type of the Evento Prévio de Emissão em Contingência.
const epecEvent = "110140"

func isEPECEvent(docType string, eventType *string) bool {
	return docType == docTypeNFe && eventType != nil && *eventType == epecEvent
}

func docKey(msg WorkerMessage) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: msg.DocPK},
		"sk": &types.AttributeValueMemberS{Value: msg.AccessKey},
	}
}

// armEPEC puts the document in the sparse contingency-index with the 168 h deadline counted from now (the
// registration). Idempotent: a re-delivered or duplicate event keeps the first deadline, and a document the
// retransmission already finished (no stored message, or another state) is left alone.
func (s *DfeService) armEPEC(ctx context.Context, msg WorkerMessage) error {
	deadline := time.Now().UTC().Add(contingency.EPECWindowHours * time.Hour).Format(time.RFC3339)
	table := s.cfg.TablePrefix + "_" + msg.TableName
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(table),
		Key:                 docKey(msg),
		UpdateExpression:    aws.String("SET #sw = :one, #st = :pending, #dl = if_not_exists(#dl, :dl)"),
		ConditionExpression: aws.String("attribute_exists(#msg) AND (#st = :awaiting OR #st = :pending)"),
		ExpressionAttributeNames: map[string]string{
			"#sw": contingency.SweepAttr, "#st": contingency.StateAttr, "#dl": contingency.DeadlineAttr, "#msg": contingency.MsgAttr,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":      &types.AttributeValueMemberS{Value: contingency.SweepValue},
			":pending":  &types.AttributeValueMemberS{Value: contingency.StatePending},
			":awaiting": &types.AttributeValueMemberS{Value: contingency.StateAwaitingEPEC},
			":dl":       &types.AttributeValueMemberS{Value: deadline},
		},
	})
	var stale *types.ConditionalCheckFailedException
	if errors.As(err, &stale) {
		slog.Info("EPEC registered but document no longer awaiting it; not armed", "access_key", msg.AccessKey)
		return nil
	}
	return err
}

// rejectEPEC ends the document: the stored signed XML and the contingency state are dropped (it can never be
// retransmitted) and the document is marked rejected with the event's cStat and motive. The result goes to SNS
// with the message's billing fields, so the API gives the quota back like for any rejected emission.
func (s *DfeService) rejectEPEC(ctx context.Context, msg WorkerMessage, cStat, motive *string) error {
	table := s.cfg.TablePrefix + "_" + msg.TableName
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(table),
		Key:              docKey(msg),
		UpdateExpression: aws.String("REMOVE #msg, #st, #sw, #dl"),
		ExpressionAttributeNames: map[string]string{
			"#sw": contingency.SweepAttr, "#st": contingency.StateAttr, "#dl": contingency.DeadlineAttr, "#msg": contingency.MsgAttr,
		},
	})
	if err != nil {
		return err
	}
	return s.updateStatusOwned(ctx, msg, StatusRejected, updateAttrs{SefazStatus: cStat, SefazMotive: motive}, true, "")
}
