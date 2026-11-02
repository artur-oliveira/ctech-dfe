package main

import (
	"context"
	"encoding/json"
	"errors"
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

// failureEnvelope is what Lambda sends to an async on-failure destination: the
// original invoke payload (requestPayload) plus why it was dropped. Lambda invokes
// this function once per failed event, after the worker's retries (or maxEventAge)
// ran out. Condition is "RetriesExhausted" or "EventAgeExceeded"; both end the same.
type failureEnvelope struct {
	RequestContext struct {
		RequestID string `json:"requestId"`
		Condition string `json:"condition"`
	} `json:"requestContext"`
	RequestPayload json.RawMessage `json:"requestPayload"`
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
// returned so Lambda retries the invoke.
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

// handler marks the failed document/event, publishes the result and alerts. Any
// error is returned so Lambda retries this function (2 retries); after that the
// event lands in the parking queue (the function's own on-failure destination).
func handler(ctx context.Context, env failureEnvelope) error {
	id := env.RequestContext.RequestID

	// A failed keep-warm ping carries nothing to mark failed or alert on.
	if service.IsPingEvent(env.RequestPayload) {
		return nil
	}

	// SNS-fed workers: requestPayload is the SNS event, the job is Records[].Sns.Message.
	records, snsErr := service.SNSMessages(env.RequestPayload)
	if snsErr == nil && len(records) > 0 {
		var errs []error
		for _, r := range records {
			var msg service.WorkerMessage
			if err := json.Unmarshal([]byte(r.Body), &msg); err != nil {
				alertUnparseable(ctx, id, len(r.Body), err)
				continue
			}
			if err := handleWorkerMessage(ctx, id, msg); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	// Directly-invoked distribution worker: requestPayload is the job itself. There is
	// no document row to mark failed (the next 30-min sweep or the user's retry redoes
	// it), so an operator alert is the whole handling.
	var job service.DistributionMessage
	if err := json.Unmarshal(env.RequestPayload, &job); err == nil && job.OrgPK != "" {
		slog.Warn("DLQ: distribution job exhausted retries", "id", id, "job_type", job.JobType, "org_pk", job.OrgPK, "condition", env.RequestContext.Condition)
		alerter.Alert(ctx, alerts.Alert{
			Job:     "dlq-exhausted",
			Summary: "A distribution job exhausted its retries.",
			Detail:  fmt.Sprintf("request_id: %s\njob_type: %s\norg_pk: %s\ndoc_type: %s\ntrigger: %s\ncondition: %s", id, job.JobType, job.OrgPK, job.DocType, job.Trigger, env.RequestContext.Condition),
		})
		return nil
	}

	// Nothing can retry it and the destination invoke ends here, so this alert is the
	// only trace that a message was lost. The payload stays out of it: it is not ours
	// to assume.
	alertUnparseable(ctx, id, len(env.RequestPayload), snsErr)
	return nil
}

func alertUnparseable(ctx context.Context, id string, bodyBytes int, err error) {
	slog.Error("DLQ: failed to parse failed payload", "id", id, "err", err)
	alerter.Alert(ctx, alerts.Alert{
		Job:     "dlq-unparseable",
		Summary: "A failed worker payload could not be parsed and was dropped.",
		Detail:  fmt.Sprintf("request_id: %s\nbody_bytes: %d", id, bodyBytes),
		Err:     err,
	})
}

func handleWorkerMessage(ctx context.Context, id string, msg service.WorkerMessage) error {
	slog.Warn("DLQ: message exhausted retries",
		"id", id,
		"access_key", msg.AccessKey,
		"doc_pk", msg.DocPK,
	)

	if err := writeTerminalStatus(ctx, msg); err != nil {
		slog.Error("DLQ: failed to write terminal status", "id", id, "access_key", msg.AccessKey, "err", err)
		// This function is retried and then parked: an operator must know it is stuck.
		alerter.Alert(ctx, alerts.Alert{
			Job:     "dlq-processor-failed",
			Summary: "The dead-letter processor could not mark a failed document; it will retry, then park the event.",
			Detail:  fmt.Sprintf("request_id: %s\naccess_key: %s\ndoc_pk: %s", id, msg.AccessKey, msg.DocPK),
			Err:     err,
		})
		return err
	}
	// The document is now terminally failed: a human decides what to do with it.
	alerter.Alert(ctx, alerts.Alert{
		Job:     "dlq-exhausted",
		Summary: "A document or event exhausted its retries and was marked failed.",
		Detail:  fmt.Sprintf("request_id: %s\naccess_key: %s\ndoc_pk: %s", id, msg.AccessKey, msg.DocPK),
	})

	if resultsTopicARN == "" {
		return nil
	}

	result := map[string]any{
		"result_kind":             "document",
		"access_key":              msg.AccessKey,
		"doc_pk":                  msg.DocPK,
		"table_name":              msg.TableName,
		"status":                  service.StatusFailed,
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
		result["result_kind"] = "event"
		result["status"] = service.EventStatusError
		result["event_type"] = msg.EventType
		result["event_sk"] = msg.EventSK
	}
	msgJSON, _ := json.Marshal(result)

	if _, err := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(resultsTopicARN),
		Message:  aws.String(string(msgJSON)),
	}); err != nil {
		slog.Error("failed to publish DLQ result", "id", id, "err", err)
		return err
	}
	return nil
}

func main() {
	ctx, stop := boot.Context()
	defer stop()
	if err := setup(ctx); err != nil {
		panic("setup: " + err.Error())
	}
	lambda.StartWithOptions(handler, lambda.WithContext(ctx))
}
