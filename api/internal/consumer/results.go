// Package consumer contains background workers that process SQS messages.
package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	resultsMaxMessages  = 10
	resultsWaitSeconds  = 20
	resultsIdleSleep    = 30 * time.Second
	resultsErrSleepMax  = 60 * time.Second
	resultsErrSleepBase = 5 * time.Second
)

// Fields and values of the worker's result message (worker/internal/service/
// helpers.go, notifyKey*). They are the wire contract between the two services;
// changing one without the other silently stops billing every emission.
const (
	resultKeyAccessKey  = "access_key"
	resultKeyDocPK      = "doc_pk"
	resultKeyOrgPK      = "org_pk"
	resultKeyResultKind = "result_kind"
	resultKeyStatus     = "status"
	resultKeyTableName  = "table_name"
	resultKeyType       = "type"

	resultKindDocument = "document"

	statusAuthorized = "authorized"
	statusRejected   = "rejected"
	statusFailed     = "failed"

	resultTypeDfe = "dfe_result"
)

// ResultsConsumer polls the NF-e/distribution results SQS queue and broadcasts
// each result to connected WebSocket clients, mirroring results_consumer.py.
type ResultsConsumer struct {
	sqsClient *sqs.Client
	queueURL  string
	registry  ws.Registry
	cache     cache.Backend
	billing   *services.BillingService
}

func NewResultsConsumer(sqsClient *sqs.Client, queueURL string, reg ws.Registry, c cache.Backend, billing *services.BillingService) *ResultsConsumer {
	return &ResultsConsumer{
		sqsClient: sqsClient,
		queueURL:  queueURL,
		registry:  reg,
		cache:     c,
		billing:   billing,
	}
}

// Start runs the consume loop until ctx is cancelled.
func (r *ResultsConsumer) Start(ctx context.Context) {
	if r.queueURL == "" {
		slog.Warn("DFE_RESULTS_QUEUE_URL not set — results consumer disabled")
		return
	}

	slog.Info("results consumer started", "queue", r.queueURL)
	errDelay := resultsErrSleepBase

	for {
		select {
		case <-ctx.Done():
			slog.Info("results consumer stopped")
			return
		default:
		}

		msgs, err := r.receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("results consumer receive error", "err", err, "retry_in", errDelay)
			select {
			case <-time.After(errDelay):
			case <-ctx.Done():
				return
			}
			errDelay = min(errDelay*2, resultsErrSleepMax)
			continue
		}
		errDelay = resultsErrSleepBase

		if len(msgs) == 0 {
			select {
			case <-time.After(resultsIdleSleep):
			case <-ctx.Done():
				return
			}
			continue
		}

		for _, msg := range msgs {
			if ctx.Err() != nil {
				return
			}
			// A message that could not be settled with billing is left on the
			// queue: SQS redelivers it three times and then raises the results
			// DLQ alarm, which is the operator's signal. Deleting it would drop
			// a usage report — money — with nothing but a log line behind it.
			if err := r.dispatch(ctx, msg); err != nil {
				continue
			}
			r.delete(ctx, aws.ToString(msg.ReceiptHandle))
		}
	}
}

func (r *ResultsConsumer) receive(ctx context.Context) ([]sqstypes.Message, error) {
	out, err := r.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(r.queueURL),
		MaxNumberOfMessages: resultsMaxMessages,
		WaitTimeSeconds:     resultsWaitSeconds,
	})
	if err != nil {
		return nil, err
	}
	return out.Messages, nil
}

func (r *ResultsConsumer) delete(ctx context.Context, receiptHandle string) {
	_, err := r.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(r.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil && ctx.Err() == nil {
		slog.Warn("results consumer delete failed", "err", err)
	}
}

// dispatch handles one result message. A returned error means the message was
// not fully processed and must stay on the queue; a malformed or unroutable
// message returns nil, because replaying it would only fail the same way.
func (r *ResultsConsumer) dispatch(ctx context.Context, msg sqstypes.Message) error {
	body := aws.ToString(msg.Body)
	if body == "" {
		return nil
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		slog.Warn("results consumer: malformed message", "msg_id", aws.ToString(msg.MessageId), "err", err)
		return nil
	}

	accessKey, _ := event[resultKeyAccessKey].(string)
	docPK, _ := event[resultKeyDocPK].(string)
	rawOrgPK, _ := event[resultKeyOrgPK].(string)

	// Two message shapes reach this consumer: doc-result messages carry
	// doc_pk ("{env}#{org_pk}", e.g. "prod#CNPJ_12345678000195"); the
	// distribution worker's new_distribution_nfe/new_distribution_cte/
	// new_distribution_mdfe messages (worker/internal/service/distribution.go
	// notifyResult) carry org_pk directly and never set doc_pk. Accept either.
	var orgPK string
	switch {
	case docPK != "" && strings.Contains(docPK, "#"):
		orgPK = strings.SplitN(docPK, "#", 2)[1]
	case rawOrgPK != "":
		orgPK = rawOrgPK
	default:
		slog.Warn("results consumer: missing doc_pk and org_pk", "doc_pk", docPK)
		return nil
	}

	// Invalidate NF-e cache entries for this document.
	if accessKey != "" {
		_ = r.cache.Delete(ctx, "res:"+orgPK+":nfes:"+accessKey)
	}
	_ = r.cache.DeletePrefix(ctx, "res:"+orgPK+":nfes:list:")
	slog.Info("results consumer: cache invalidated", "org", orgPK, "key", accessKey)

	// Doc-result messages never set "type" themselves — default them to
	// "dfe_result". Distribution-notification messages (new_distribution_nfe/
	// _cte/_mdfe) already set their own "type"; preserve it, or the frontend's
	// type-specific handling (useRealtimeUpdates.ts) never matches.
	if _, hasType := event[resultKeyType]; !hasType {
		event[resultKeyType] = resultTypeDfe
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("results consumer: marshal failed", "err", err)
		return nil
	}

	r.registry.Broadcast(ctx, orgPK, payload)
	slog.Info("results consumer: broadcast sent", "org", orgPK, "key", accessKey)

	return r.settleBilling(ctx, event, orgPK, accessKey)
}

// billingAction is what a result message owes billing.
type billingAction int

const (
	billingNone billingAction = iota
	billingReport
	billingRefund
)

// billingActionFor decides, from the message alone, what the outcome costs.
//
// Only documents count. A cancellation or a carta de correção is an event on a
// document that was already billed, and charging it again would bill one
// emission twice; a distribution notification is somebody else's document
// arriving, which this account never issued.
func billingActionFor(event map[string]any) (billingAction, string) {
	if kind, _ := event[resultKeyResultKind].(string); kind != resultKindDocument {
		return billingNone, ""
	}
	table, _ := event[resultKeyTableName].(string)
	meter, ok := services.MeterForTable[table]
	if !ok {
		return billingNone, ""
	}
	switch status, _ := event[resultKeyStatus].(string); status {
	case statusAuthorized:
		return billingReport, meter
	case statusRejected, statusFailed:
		// Both are terminal and neither produced a fiscal document, so the slot
		// the API reserved when the request came in has to go back. Anything else
		// — retryable_failed above all — is still in flight and keeps its slot.
		return billingRefund, meter
	default:
		return billingNone, ""
	}
}

// settleBilling turns a terminal document outcome into money.
//
// It runs in the API rather than in the worker, where the plan first put it,
// because everything it needs is already here: the billing client, the account
// snapshot, and the quota counters. The worker is a separate Go module and would
// need a second copy of all three — a token manager included — to reach the same
// rows, which is the duplication the repo's DRY rule exists to prevent.
//
// A failed report is returned so the message is retried. A failed refund is not:
// its once-only marker is already claimed, so a retry would skip it anyway, and
// the customer is short one slot rather than the queue being stuck.
func (r *ResultsConsumer) settleBilling(ctx context.Context, event map[string]any, orgPK, accessKey string) error {
	if r.billing == nil || accessKey == "" {
		return nil
	}
	action, meter := billingActionFor(event)
	switch action {
	case billingReport:
		if err := r.billing.ReportUsage(ctx, orgPK, meter, accessKey); err != nil {
			slog.ErrorContext(ctx, "results consumer: usage report failed",
				"org", orgPK, "key", accessKey, "meter", meter, "err", err)
			return err
		}
	case billingRefund:
		r.billing.RefundOnce(ctx, orgPK, meter, accessKey)
	}
	return nil
}
