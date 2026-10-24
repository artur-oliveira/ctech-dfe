package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/dfe/worker/internal/boot"
	"gopkg.aoctech.app/dfe/worker/internal/service"
)

// dynamoUpdater is the DynamoDB subset the DLQ processor needs — narrow
// enough to fake in tests without stubbing the full dynamodb.Client.
type dynamoUpdater interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

var (
	snsClient       *sns.Client
	dynamoClient    dynamoUpdater
	resultsTopicARN string
	tablePrefix     string
	// alerter tells an operator that a message exhausted its retries. Nop until
	// main wires the shared SNS topic (tests keep Nop).
	alerter alerts.Publisher = alerts.Nop{}
)

func setup(ctx context.Context) error {
	resultsTopicARN = os.Getenv("RESULTS_TOPIC_ARN")
	tablePrefix = os.Getenv("TABLE_PREFIX")
	ac, err := boot.AWSConfig(ctx)
	if err != nil {
		return err
	}
	snsClient = sns.NewFromConfig(ac)
	dynamoClient = dynamodb.NewFromConfig(ac)
	alerter = boot.Alerts(ctx, "dfe-worker")
	return nil
}

type sqsEvent struct {
	Records []sqsRecord `json:"Records"`
}

type sqsRecord struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
}

// batchResponse/batchItemFailure mirror the shape cmd/worker/main.go returns
// for partial batch failure — only the records named here get redelivered by
// SQS; the rest of the batch is acked even when one record fails.
type batchResponse struct {
	BatchItemFailures []batchItemFailure `json:"batchItemFailures"`
}

type batchItemFailure struct {
	ItemIdentifier string `json:"itemIdentifier"`
}

const dlqFailureMotive = "Falha após todas as tentativas de reprocessamento"

// terminalUpdateTarget resolves which table/key/status to write for a message
// that exhausted retries — mirrors the document-vs-event routing already used
// by DfeService.failDoc (worker/internal/service/dfe.go).
func terminalUpdateTarget(msg service.WorkerMessage) (table, pk, sk, status string) {
	if msg.EventsTableName != nil && msg.EventSK != nil {
		return tablePrefix + "_" + *msg.EventsTableName, msg.AccessKey, *msg.EventSK, service.EventStatusError
	}
	return tablePrefix + "_" + msg.TableName, msg.DocPK, msg.AccessKey, service.StatusFailed
}

// writeTerminalStatus marks the document or event as terminally failed. Both
// this write and the result publication must succeed; either failure is
// returned so Lambda/SQS retries the record.
func writeTerminalStatus(ctx context.Context, msg service.WorkerMessage) error {
	table, pk, sk, status := terminalUpdateTarget(msg)
	_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET #status = :status, sefaz_motive = :motive, updated_at = :updated"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":  &types.AttributeValueMemberS{Value: status},
			":motive":  &types.AttributeValueMemberS{Value: dlqFailureMotive},
			":updated": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
		},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	})
	return err
}

func handler(ctx context.Context, event sqsEvent) (batchResponse, error) {
	var failures []batchItemFailure
	for _, record := range event.Records {
		var msg service.WorkerMessage
		if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
			slog.Error("DLQ: failed to parse message body", "id", record.MessageID, "err", err)
			continue
		}

		slog.Warn("DLQ: message exhausted retries",
			"id", record.MessageID,
			"access_key", msg.AccessKey,
			"doc_pk", msg.DocPK,
		)

		if err := writeTerminalStatus(ctx, msg); err != nil {
			slog.Error("DLQ: failed to write terminal status", "id", record.MessageID, "access_key", msg.AccessKey, "err", err)
			failures = append(failures, batchItemFailure{ItemIdentifier: record.MessageID})
			continue
		}
		// The document is now terminally failed: a human decides what to do with it.
		alerter.Alert(ctx, alerts.Alert{
			Job:     "dlq-exhausted",
			Summary: "A document or event exhausted its retries and was marked failed.",
			Detail:  fmt.Sprintf("message_id: %s\naccess_key: %s\ndoc_pk: %s", record.MessageID, msg.AccessKey, msg.DocPK),
		})

		if resultsTopicARN == "" {
			continue
		}

		resultStatus := service.StatusFailed
		result := map[string]any{
			"result_kind":             "document",
			"access_key":              msg.AccessKey,
			"doc_pk":                  msg.DocPK,
			"table_name":              msg.TableName,
			"status":                  resultStatus,
			"sefaz_status":            nil,
			"sefaz_motive":            dlqFailureMotive,
			"sefaz_protocol":          nil,
			"xml_s3_key":              nil,
			"billing_user_id":         msg.BillingUserID,
			"billing_organization_id": msg.BillingOrganizationID,
			"billing_period":          msg.BillingPeriod,
			"billing_subscription_id": msg.BillingSubscriptionID,
			"billing_price_id":        msg.BillingPriceID,
			"billing_meter":           msg.BillingMeter,
			"billing_exempt":          msg.BillingExempt,
		}
		if msg.EventsTableName != nil {
			resultStatus = service.EventStatusError
			result["result_kind"] = "event"
			result["status"] = resultStatus
			result["event_type"] = msg.EventType
			result["event_sk"] = msg.EventSK
		}
		msgJSON, _ := json.Marshal(result)

		if _, err := snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(resultsTopicARN),
			Message:  aws.String(string(msgJSON)),
		}); err != nil {
			slog.Error("failed to publish DLQ result", "id", record.MessageID, "err", err)
			failures = append(failures, batchItemFailure{ItemIdentifier: record.MessageID})
		}
	}
	return batchResponse{BatchItemFailures: failures}, nil
}

func main() {
	ctx, stop := boot.Context()
	defer stop()
	if err := setup(ctx); err != nil {
		panic("setup: " + err.Error())
	}
	lambda.StartWithOptions(handler, lambda.WithContext(ctx))
}
