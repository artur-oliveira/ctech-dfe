package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/dfe/worker/internal/service"
)

// envelope builds the Lambda on-failure destination payload for a failed SNS-fed worker invoke.
func envelope(t *testing.T, msg service.WorkerMessage) failureEnvelope {
	t.Helper()
	body, _ := json.Marshal(msg)
	return rawEnvelope(t, snsEventOf(string(body)))
}

func snsEventOf(message string) map[string]any {
	return map[string]any{"Records": []any{map[string]any{"Sns": map[string]any{"MessageId": "sns-1", "Message": message}}}}
}

func rawEnvelope(t *testing.T, payload any) failureEnvelope {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{
		"requestContext": map[string]any{"requestId": "req-1", "condition": "RetriesExhausted"},
		"requestPayload": payload,
	})
	var env failureEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env
}

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
	env := envelope(t, msg)

	if err := handler(context.Background(), env); err != nil {
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

	eventSK := "01930000-0000-7000-8000-000000000001"
	msg := service.WorkerMessage{
		DocPK:           "prod#CNPJ_12345678000195",
		AccessKey:       "35250512345678000195550010000000011000000011",
		TableName:       "nfes",
		EventsTableName: new("nfe_events"),
		EventType:       new("110111"),
		EventSK:         &eventSK,
	}
	env := envelope(t, msg)

	if err := handler(context.Background(), env); err != nil {
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

func TestHandler_DynamoUpdateFails_ReturnsErrorAndAlerts(t *testing.T) {
	tablePrefix, resultsTopicARN = "dev", ""
	dynamoClient = &fakeDynamo{err: context.DeadlineExceeded}
	ra := &recordingAlerts{}
	alerter = ra
	t.Cleanup(func() { alerter = alerts.Nop{} })

	err := handler(context.Background(), envelope(t, service.WorkerMessage{DocPK: "pk", AccessKey: "ak", TableName: "nfes"}))
	if err == nil {
		t.Fatal("a failed terminal write must return an error so Lambda retries, then parks the event")
	}
	if len(ra.got) != 1 || ra.got[0].Job != "dlq-processor-failed" {
		t.Fatalf("want one dlq-processor-failed alert, got %+v", ra.got)
	}
}

type recordingAlerts struct{ got []alerts.Alert }

func (r *recordingAlerts) Alert(_ context.Context, a alerts.Alert) { r.got = append(r.got, a) }

func TestHandler_ExhaustedMessageAlertsOperator(t *testing.T) {
	tablePrefix, resultsTopicARN = "dev", ""
	dynamoClient = &fakeDynamo{}
	ra := &recordingAlerts{}
	alerter = ra
	t.Cleanup(func() { alerter = alerts.Nop{} })

	if err := handler(context.Background(), envelope(t, service.WorkerMessage{DocPK: "prod#CNPJ_1", AccessKey: "KEY1", TableName: "nfes"})); err != nil {
		t.Fatal(err)
	}
	if len(ra.got) != 1 || ra.got[0].Job != "dlq-exhausted" || !strings.Contains(ra.got[0].Detail, "KEY1") {
		t.Fatalf("want one dlq-exhausted alert naming the access key, got %+v", ra.got)
	}
}

// Nothing can retry an unparseable payload and the destination invoke ends here:
// the alert is the only trace that it was lost, and the body must not leak into it.
func TestHandler_UnparseableMessageAlertsOperatorAndIsDropped(t *testing.T) {
	fd := &fakeDynamo{}
	dynamoClient = fd
	ra := &recordingAlerts{}
	alerter = ra
	t.Cleanup(func() { alerter = alerts.Nop{} })

	for name, env := range map[string]failureEnvelope{
		"bad sns body":    rawEnvelope(t, snsEventOf("SECRET not json")),
		"unknown payload": rawEnvelope(t, map[string]any{"x": 1}),
		"non-object":      rawEnvelope(t, "just a string"),
	} {
		ra.got = nil
		if err := handler(context.Background(), env); err != nil {
			t.Fatalf("%s: dropped payload must not error (nothing to retry): %v", name, err)
		}
		if len(ra.got) != 1 || ra.got[0].Job != "dlq-unparseable" {
			t.Fatalf("%s: want one dlq-unparseable alert, got %+v", name, ra.got)
		}
		if strings.Contains(ra.got[0].Detail, "SECRET") {
			t.Fatalf("%s: alert must not carry the body: %q", name, ra.got[0].Detail)
		}
	}
	if len(fd.calls) != 0 {
		t.Fatalf("unparseable payload must not write: %v", fd.calls)
	}
}

func TestHandler_DistributionJobAlertsWithoutWriting(t *testing.T) {
	fd := &fakeDynamo{}
	dynamoClient = fd
	ra := &recordingAlerts{}
	alerter = ra
	t.Cleanup(func() { alerter = alerts.Nop{} })

	env := rawEnvelope(t, map[string]any{"job_type": "dist_nsu", "org_pk": "CNPJ_1", "doc_type": "nfe", "trigger": "user"})
	if err := handler(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if len(fd.calls) != 0 || len(ra.got) != 1 || ra.got[0].Job != "dlq-exhausted" || !strings.Contains(ra.got[0].Detail, "CNPJ_1") {
		t.Fatalf("want one alert and no write: calls=%v alerts=%+v", fd.calls, ra.got)
	}
}

func TestHandler_FailedKeepWarmPingIsIgnored(t *testing.T) {
	ra := &recordingAlerts{}
	alerter = ra
	t.Cleanup(func() { alerter = alerts.Nop{} })
	if err := handler(context.Background(), rawEnvelope(t, map[string]any{"ping": true})); err != nil || len(ra.got) != 0 {
		t.Fatalf("ping failure must be a silent no-op: err=%v alerts=%+v", err, ra.got)
	}
}
