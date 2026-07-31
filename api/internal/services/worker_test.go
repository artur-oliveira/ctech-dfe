package services

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestBuildOutboxTxProducesImmutablePendingCommand(t *testing.T) {
	svc := NewWorkerService(nil, "", "dev_dfe")
	msg := WorkerMessage{TableName: "nfes", AccessKey: "key-1", DocPK: "hom#org", Body: map[string]any{"NFe": "body"}}

	tx, operationID, err := svc.BuildOutboxTx(msg)
	if err != nil {
		t.Fatalf("BuildOutboxTx: %v", err)
	}
	if operationID != "nfes#key-1" {
		t.Fatalf("operationID = %q", operationID)
	}
	if tx.Put == nil || tx.Put.TableName == nil || *tx.Put.TableName != "dev_dfe_worker_outbox" {
		t.Fatalf("unexpected outbox transaction: %+v", tx)
	}
	if tx.Put.ConditionExpression == nil || *tx.Put.ConditionExpression != "attribute_not_exists(pk) AND attribute_not_exists(sk)" {
		t.Fatalf("outbox command must be immutable: %+v", tx.Put.ConditionExpression)
	}
	status, ok := tx.Put.Item["status"].(*types.AttributeValueMemberS)
	if !ok || status.Value != workerOutboxPending {
		t.Fatalf("status = %#v", tx.Put.Item["status"])
	}
	payload := tx.Put.Item["payload"].(*types.AttributeValueMemberS).Value
	var decoded WorkerMessage
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if decoded.AccessKey != msg.AccessKey || decoded.DocPK != msg.DocPK {
		t.Fatalf("decoded payload = %+v", decoded)
	}
}
