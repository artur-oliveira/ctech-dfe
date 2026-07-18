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
)

const (
	resultsMaxMessages  = 10
	resultsWaitSeconds  = 20
	resultsIdleSleep    = 30 * time.Second
	resultsErrSleepMax  = 60 * time.Second
	resultsErrSleepBase = 5 * time.Second
)

// ResultsConsumer polls the NF-e/distribution results SQS queue and broadcasts
// each result to connected WebSocket clients, mirroring results_consumer.py.
type ResultsConsumer struct {
	sqsClient *sqs.Client
	queueURL  string
	registry  ws.Registry
	cache     cache.Backend
}

func NewResultsConsumer(sqsClient *sqs.Client, queueURL string, reg ws.Registry, c cache.Backend) *ResultsConsumer {
	return &ResultsConsumer{
		sqsClient: sqsClient,
		queueURL:  queueURL,
		registry:  reg,
		cache:     c,
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
			r.dispatch(ctx, msg)
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

func (r *ResultsConsumer) dispatch(ctx context.Context, msg sqstypes.Message) {
	body := aws.ToString(msg.Body)
	if body == "" {
		return
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		slog.Warn("results consumer: malformed message", "msg_id", aws.ToString(msg.MessageId), "err", err)
		return
	}

	accessKey, _ := event["access_key"].(string)
	docPK, _ := event["doc_pk"].(string)

	// doc_pk format: "{env}#{org_pk}" e.g. "prod#CNPJ_12345678000195"
	if docPK == "" || !strings.Contains(docPK, "#") {
		slog.Warn("results consumer: missing or invalid doc_pk", "doc_pk", docPK)
		return
	}
	orgPK := strings.SplitN(docPK, "#", 2)[1]

	// Invalidate NF-e cache entries for this document.
	if accessKey != "" {
		_ = r.cache.Delete(ctx, "res:"+orgPK+":nfes:"+accessKey)
	}
	_ = r.cache.DeletePrefix(ctx, "res:"+orgPK+":nfes:list:")
	slog.Info("results consumer: cache invalidated", "org", orgPK, "key", accessKey)

	event["type"] = "dfe_result"
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("results consumer: marshal failed", "err", err)
		return
	}

	r.registry.Broadcast(ctx, orgPK, payload)
	slog.Info("results consumer: broadcast sent", "org", orgPK, "key", accessKey)
}
