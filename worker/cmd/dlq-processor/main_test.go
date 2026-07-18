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

	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("handler: %v", err)
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

	eventsTable := "nfe_events"
	eventSK := "01930000-0000-7000-8000-000000000001"
	msg := service.WorkerMessage{
		DocPK:           "prod#CNPJ_12345678000195",
		AccessKey:       "35250512345678000195550010000000011000000011",
		TableName:       "nfes",
		EventsTableName: &eventsTable,
		EventType:       ptr("110111"),
		EventSK:         &eventSK,
	}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m2", Body: string(body)}}}

	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("handler: %v", err)
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

func TestHandler_DynamoUpdateFails_StillPublishesSNS(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = "" // keep SNS a no-op for this test; only asserting handler doesn't error
	fd := &fakeDynamo{err: context.DeadlineExceeded}
	dynamoClient = fd

	msg := service.WorkerMessage{DocPK: "pk", AccessKey: "ak", TableName: "nfes"}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m3", Body: string(body)}}}

	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("handler must not fail the whole batch on a DynamoDB error: %v", err)
	}
}
