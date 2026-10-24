package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/dfe/worker/internal/boot"
)

const (
	outboxStatusPending   = "pending"
	outboxStatusPublished = "published"
)

type publisher interface {
	Publish(context.Context, *sns.PublishInput, ...func(*sns.Options)) (*sns.PublishOutput, error)
}

type updater interface {
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

var (
	snsClient    publisher
	dynamoClient updater
	topicARN     string
	tableName    string
	// alerter tells an operator that an outbox record is stuck. Nop until main
	// wires the shared SNS topic (tests keep Nop).
	alerter alerts.Publisher = alerts.Nop{}
)

// stuckAfter is how old an outbox record must be before a failing publish pages
// a human: Lambda retries the stream record with backoff, so a blip heals
// silently and only a record that stays unpublished is worth an email.
const stuckAfter = 10 * time.Minute

func setup(ctx context.Context) error {
	topicARN = os.Getenv("EVENT_BUS_TOPIC_ARN")
	tableName = os.Getenv("OUTBOX_TABLE_NAME")
	cfg, err := boot.AWSConfig(ctx)
	if err != nil {
		return err
	}
	snsClient = sns.NewFromConfig(cfg)
	dynamoClient = dynamodb.NewFromConfig(cfg)
	alerter = boot.Alerts(ctx, "dfe-outbox-publisher")
	return nil
}

func processRecord(ctx context.Context, record events.DynamoDBEventRecord) error {
	image := record.Change.NewImage
	status, ok := image["status"]
	if !ok || status.String() != outboxStatusPending {
		return nil
	}
	pk, pkOK := image["pk"]
	sk, skOK := image["sk"]
	payload, payloadOK := image["payload"]
	if !pkOK || !skOK || !payloadOK {
		return fmt.Errorf("outbox stream record %s is missing pk, sk, or payload", record.EventID)
	}
	if topicARN == "" || tableName == "" {
		return errors.New("EVENT_BUS_TOPIC_ARN and OUTBOX_TABLE_NAME are required")
	}

	out, err := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(topicARN),
		Message:  aws.String(payload.String()),
	})
	if err != nil {
		return fmt.Errorf("publish outbox %s: %w", pk.String(), err)
	}
	messageID := ""
	if out != nil && out.MessageId != nil {
		messageID = *out.MessageId
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk.String()},
			"sk": &types.AttributeValueMemberS{Value: sk.String()},
		},
		UpdateExpression:    aws.String("SET #status = :published, published_at = :published_at, sns_message_id = :message_id"),
		ConditionExpression: aws.String("#status = :pending"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pending":      &types.AttributeValueMemberS{Value: outboxStatusPending},
			":published":    &types.AttributeValueMemberS{Value: outboxStatusPublished},
			":published_at": &types.AttributeValueMemberS{Value: now},
			":message_id":   &types.AttributeValueMemberS{Value: messageID},
		},
	})
	if err != nil {
		if _, ok2 := errors.AsType[*types.ConditionalCheckFailedException](err); ok2 {
			return nil
		}
		return fmt.Errorf("acknowledge outbox %s: %w", pk.String(), err)
	}
	slog.Info("published worker outbox", "operation_id", pk.String(), "sns_message_id", messageID)
	return nil
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	for _, record := range event.Records {
		if err := processRecord(ctx, record); err != nil {
			alertIfStuck(ctx, record, err)
			return err
		}
	}
	return nil
}

func alertIfStuck(ctx context.Context, record events.DynamoDBEventRecord, err error) {
	age := time.Since(record.Change.ApproximateCreationDateTime.Time)
	if age < stuckAfter {
		return
	}
	alerter.Alert(ctx, alerts.Alert{
		Job:     "outbox-stuck",
		Summary: "An outbox record cannot be published to the command topic; the stream keeps retrying it.",
		Err:     err,
		Detail:  fmt.Sprintf("stream_event_id: %s\nage: %s", record.EventID, age.Round(time.Minute)),
	})
}

func main() {
	ctx, stop := boot.Context()
	defer stop()
	if err := setup(ctx); err != nil {
		panic("setup: " + err.Error())
	}
	lambda.StartWithOptions(handler, lambda.WithContext(ctx))
}
