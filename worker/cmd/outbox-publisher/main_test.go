package main

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
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
