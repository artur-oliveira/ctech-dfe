package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type fakeDynamo struct {
	calls []*dynamodb.UpdateItemInput
	err   error
}

func (f *fakeDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls = append(f.calls, in)
	return &dynamodb.UpdateItemOutput{}, nil
}

func attrS(t *testing.T, av types.AttributeValue) string {
	t.Helper()
	s, ok := av.(*types.AttributeValueMemberS)
	if !ok {
		t.Fatalf("attribute is not a string: %#v", av)
	}
	return s.Value
}

func ptr(s string) *string { return &s }

func TestHandler_DocumentMessage_WritesFailedStatus(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = ""
	fd := &fakeDynamo{}
	dynamoClient = fd

	msg := service.WorkerMessage{
		DocPK:     "prod#CNPJ_12345678000195",
		AccessKey: "35250512345678000195550010000000011000000011",
		TableName: "nfes",
	}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m1", Body: string(body)}}}

	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected no batch item failures, got %v", resp.BatchItemFailures)
	}
	if len(fd.calls) != 1 {
		t.Fatalf("expected 1 UpdateItem call, got %d", len(fd.calls))
	}
	call := fd.calls[0]
	if *call.TableName != "dev_nfes" {
		t.Errorf("table = %q, want dev_nfes", *call.TableName)
	}
	if attrS(t, call.Key["pk"]) != msg.DocPK {
		t.Errorf("pk = %q, want %q", attrS(t, call.Key["pk"]), msg.DocPK)
	}
	if attrS(t, call.Key["sk"]) != msg.AccessKey {
		t.Errorf("sk = %q, want %q", attrS(t, call.Key["sk"]), msg.AccessKey)
	}
	if attrS(t, call.ExpressionAttributeValues[":status"]) != service.StatusFailed {
		t.Errorf("status = %q, want %q", attrS(t, call.ExpressionAttributeValues[":status"]), service.StatusFailed)
	}
	if call.ConditionExpression == nil || *call.ConditionExpression != "attribute_exists(pk)" {
		t.Errorf("condition expression = %v, want attribute_exists(pk)", call.ConditionExpression)
	}
}

func TestHandler_EventMessage_WritesEventErrorStatus(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = ""
	fd := &fakeDynamo{}
	dynamoClient = fd

	eventSK := "01930000-0000-7000-8000-000000000001"
	msg := service.WorkerMessage{
		DocPK:           "prod#CNPJ_12345678000195",
		AccessKey:       "35250512345678000195550010000000011000000011",
		TableName:       "nfes",
		EventsTableName: new("nfe_events"),
		EventType:       new("110111"),
		EventSK:         &eventSK,
	}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m2", Body: string(body)}}}

	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected no batch item failures, got %v", resp.BatchItemFailures)
	}
	if len(fd.calls) != 1 {
		t.Fatalf("expected 1 UpdateItem call, got %d", len(fd.calls))
	}
	call := fd.calls[0]
	if *call.TableName != "dev_nfe_events" {
		t.Errorf("table = %q, want dev_nfe_events", *call.TableName)
	}
	if attrS(t, call.Key["pk"]) != msg.AccessKey {
		t.Errorf("pk = %q, want %q", attrS(t, call.Key["pk"]), msg.AccessKey)
	}
	if attrS(t, call.Key["sk"]) != eventSK {
		t.Errorf("sk = %q, want %q", attrS(t, call.Key["sk"]), eventSK)
	}
	if attrS(t, call.ExpressionAttributeValues[":status"]) != service.EventStatusError {
		t.Errorf("status = %q, want %q", attrS(t, call.ExpressionAttributeValues[":status"]), service.EventStatusError)
	}
}

func TestHandler_DynamoUpdateFails_ReportsRecordAsBatchItemFailure(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = "" // keep SNS a no-op for this test; only asserting failure reporting
	fd := &fakeDynamo{err: context.DeadlineExceeded}
	dynamoClient = fd

	msg := service.WorkerMessage{DocPK: "pk", AccessKey: "ak", TableName: "nfes"}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m3", Body: string(body)}}}

	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "m3" {
		t.Fatalf("expected m3 reported as a batch item failure, got %v", resp.BatchItemFailures)
	}
}

// A poison message (e.g. its Dynamo condition check is permanently false)
// must not cause a sibling message in the same batch that succeeded to be
// redelivered — only the failing record should come back in
// BatchItemFailures. This is the behavior reportBatchItemFailures depends on.
func TestHandler_OneFailureInBatch_DoesNotFailSiblingRecord(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = ""

	failingMsg := service.WorkerMessage{DocPK: "pk-bad", AccessKey: "ak-bad", TableName: "nfes"}
	okMsg := service.WorkerMessage{DocPK: "pk-good", AccessKey: "ak-good", TableName: "nfes"}
	failingBody, _ := json.Marshal(failingMsg)
	okBody, _ := json.Marshal(okMsg)

	fd := &conditionalDynamo{failPK: failingMsg.DocPK}
	dynamoClient = fd

	event := sqsEvent{Records: []sqsRecord{
		{MessageID: "bad", Body: string(failingBody)},
		{MessageID: "good", Body: string(okBody)},
	}}

	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.BatchItemFailures) != 1 || resp.BatchItemFailures[0].ItemIdentifier != "bad" {
		t.Fatalf("expected only \"bad\" reported as a batch item failure, got %v", resp.BatchItemFailures)
	}
	if len(fd.calls) != 2 {
		t.Fatalf("expected both records to be attempted, got %d calls", len(fd.calls))
	}
}

// conditionalDynamo fails UpdateItem only for a specific pk, so a test can
// assert that one poison message doesn't take down a sibling in the batch.
type conditionalDynamo struct {
	failPK string
	calls  []*dynamodb.UpdateItemInput
}

func (f *conditionalDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.calls = append(f.calls, in)
	if s, ok := in.Key["pk"].(*types.AttributeValueMemberS); ok && s.Value == f.failPK {
		return nil, context.DeadlineExceeded
	}
	return &dynamodb.UpdateItemOutput{}, nil
}
