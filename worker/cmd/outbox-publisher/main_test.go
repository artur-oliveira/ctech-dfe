package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/api-commons/alerts"
)

type fakePublisher struct {
	calls int
	err   error
}

func (f *fakePublisher) Publish(_ context.Context, _ *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &sns.PublishOutput{MessageId: aws.String("sns-1")}, nil
}

type fakeUpdater struct {
	calls int
}

func (f *fakeUpdater) UpdateItem(_ context.Context, _ *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.calls++
	return &dynamodb.UpdateItemOutput{}, nil
}

func outboxRecord(status string) events.DynamoDBEventRecord {
	return events.DynamoDBEventRecord{
		EventID: "stream-1",
		Change: events.DynamoDBStreamRecord{NewImage: map[string]events.DynamoDBAttributeValue{
			"pk":      events.NewStringAttribute("nfes#key"),
			"sk":      events.NewStringAttribute("command"),
			"status":  events.NewStringAttribute(status),
			"payload": events.NewStringAttribute("{\"access_key\":\"key\"}"),
		}},
	}
}

func TestProcessRecordPublishesAndAcknowledgesPendingOutbox(t *testing.T) {
	topicARN = "arn:aws:sns:us-east-1:123:commands"
	tableName = "dev_dfe_worker_outbox"
	pub := &fakePublisher{}
	upd := &fakeUpdater{}
	snsClient, dynamoClient = pub, upd

	if err := processRecord(context.Background(), outboxRecord(outboxStatusPending)); err != nil {
		t.Fatalf("processRecord: %v", err)
	}
	if pub.calls != 1 || upd.calls != 1 {
		t.Fatalf("expected one publish and one acknowledgement, got publish=%d update=%d", pub.calls, upd.calls)
	}
}

func TestProcessRecordRetriesWhenSNSPublishFails(t *testing.T) {
	topicARN = "arn:aws:sns:us-east-1:123:commands"
	tableName = "dev_dfe_worker_outbox"
	pub := &fakePublisher{err: errors.New("sns unavailable")}
	upd := &fakeUpdater{}
	snsClient, dynamoClient = pub, upd

	if err := processRecord(context.Background(), outboxRecord(outboxStatusPending)); err == nil {
		t.Fatal("expected publish error")
	}
	if upd.calls != 0 {
		t.Fatalf("failed publish must not acknowledge outbox, got %d updates", upd.calls)
	}
}

func TestProcessRecordSkipsPublishedOutbox(t *testing.T) {
	pub := &fakePublisher{}
	upd := &fakeUpdater{}
	snsClient, dynamoClient = pub, upd

	if err := processRecord(context.Background(), outboxRecord(outboxStatusPublished)); err != nil {
		t.Fatalf("processRecord: %v", err)
	}
	if pub.calls != 0 || upd.calls != 0 {
		t.Fatalf("published record must be ignored, got publish=%d update=%d", pub.calls, upd.calls)
	}
}

type recordingAlerts struct{ got []alerts.Alert }

func (r *recordingAlerts) Alert(_ context.Context, a alerts.Alert) { r.got = append(r.got, a) }

func TestHandlerAlertsOnlyWhenRecordIsStuck(t *testing.T) {
	topicARN, tableName = "arn:aws:sns:us-east-1:123:commands", "dev_dfe_worker_outbox"
	snsClient, dynamoClient = &fakePublisher{err: errors.New("sns unavailable")}, &fakeUpdater{}
	ra := &recordingAlerts{}
	alerter = ra
	t.Cleanup(func() { alerter = alerts.Nop{} })

	for _, tc := range []struct {
		age  time.Duration
		want int
	}{{time.Minute, 0}, {stuckAfter + time.Minute, 1}} {
		ra.got = nil
		rec := outboxRecord(outboxStatusPending)
		rec.Change.ApproximateCreationDateTime = events.SecondsEpochTime{Time: time.Now().Add(-tc.age)}
		if err := handler(context.Background(), events.DynamoDBEvent{Records: []events.DynamoDBEventRecord{rec}}); err == nil {
			t.Fatal("handler must still return the error so the stream retries")
		}
		if len(ra.got) != tc.want {
			t.Fatalf("age %s: want %d alerts, got %+v", tc.age, tc.want, ra.got)
		}
	}
}
