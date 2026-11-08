package services

// reissue.go: the link between a document and the one that replaces it when it is regenerated in contingency
// (docs/specs/2026-11-08-contingencia.md, 4.1 item 3). The regenerated document has another access key (tpEmis is
// in the key) and the same serie and number. The original is marked FAILED (a terminal status the worker never
// claims again) and points to the new key; the new one carries reissued_from, which the 539 reconciliation reads.
// Nothing calls this yet: regenerating an in-flight document is deferred by the owner (spec 4.2 item 1).

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	AttrReissuedFrom = "reissued_from" // on the new document: the original access key
	AttrReissuedTo   = "reissued_to"   // on the original: the new access key
	AttrReissuedAt   = "reissued_at"

	statusAuthorized = "authorized"
	statusCancelled  = "cancelled"
	statusFailed     = "failed"
)

// TxWriter is the transactional write of the repositories.
type TxWriter interface {
	TransactWrite(ctx context.Context, items []types.TransactWriteItem) error
}

// LinkReissue atomically marks the original (failed, reissued_to) and sets reissued_from on the new document.
// It is idempotent (the same pair again succeeds) and refuses to touch an original that was authorized or
// cancelled (a fiscal document is never superseded once the SEFAZ holds it) or one already superseded by
// another key. Both documents live in table under the same pk.
func LinkReissue(ctx context.Context, w TxWriter, table, pk, originalKey, newKey string, now time.Time) error {
	at := &types.AttributeValueMemberS{Value: now.UTC().Format(time.RFC3339)}
	key := func(sk string) map[string]types.AttributeValue {
		return map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: pk}, "sk": &types.AttributeValueMemberS{Value: sk}}
	}
	return w.TransactWrite(ctx, []types.TransactWriteItem{
		{Update: &types.Update{
			TableName: aws.String(table), Key: key(originalKey),
			UpdateExpression: aws.String("SET #st = :failed, #to = :new, #at = :at"),
			ConditionExpression: aws.String("attribute_exists(pk) AND (attribute_not_exists(#to) OR #to = :new) " +
				"AND #st <> :authorized AND #st <> :cancelled"),
			ExpressionAttributeNames: map[string]string{"#st": "status", "#to": AttrReissuedTo, "#at": AttrReissuedAt},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":failed": &types.AttributeValueMemberS{Value: statusFailed}, ":new": &types.AttributeValueMemberS{Value: newKey},
				":authorized": &types.AttributeValueMemberS{Value: statusAuthorized},
				":cancelled":  &types.AttributeValueMemberS{Value: statusCancelled}, ":at": at,
			},
		}},
		{Update: &types.Update{
			TableName: aws.String(table), Key: key(newKey),
			UpdateExpression:         aws.String("SET #from = :orig"),
			ConditionExpression:      aws.String("attribute_exists(pk) AND (attribute_not_exists(#from) OR #from = :orig)"),
			ExpressionAttributeNames: map[string]string{"#from": AttrReissuedFrom},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":orig": &types.AttributeValueMemberS{Value: originalKey},
			},
		}},
	})
}
