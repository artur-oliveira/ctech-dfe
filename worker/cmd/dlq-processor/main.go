package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

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
)

func init() {
	resultsTopicARN = os.Getenv("RESULTS_TOPIC_ARN")
	tablePrefix = os.Getenv("TABLE_PREFIX")
	ac, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("aws config: " + err.Error())
	}
	snsClient = sns.NewFromConfig(ac)
	dynamoClient = dynamodb.NewFromConfig(ac)
}

type sqsEvent struct {
	Records []sqsRecord `json:"Records"`
}

type sqsRecord struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
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

func handler(ctx context.Context, event sqsEvent) error {
	var failures []error
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
			failures = append(failures, err)
			continue
		}

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
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func main() {
	lambda.Start(handler)
}
