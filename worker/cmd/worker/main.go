package main

import (
	"context"
	"encoding/json"
	"errors"
	"gopkg.aoctech.app/api-commons/erasure"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"gopkg.aoctech.app/dfe/worker/internal/boot"
	"gopkg.aoctech.app/dfe/worker/internal/config"
	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type processor interface {
	Process(ctx context.Context, msg service.WorkerMessage) error
}

// handle processes an SNS event (async invoke from the event bus). Any failure is
// returned so Lambda's async retry (2 retries, then the dlq-processor destination)
// takes over; SNS delivers one record per invoke, so there is no partial batch.
// Process is idempotent under redelivery (claimProcessing lease + terminal-status
// republish), so a retry after a timeout cannot call SEFAZ twice for a done doc.
func handle(ctx context.Context, raw json.RawMessage, p processor) error {
	if service.IsPingEvent(raw) {
		slog.Info("ping: keep-warm invoke, no-op")
		return nil
	}
	records, err := service.SNSMessages(raw)
	if err != nil {
		slog.Error("failed to parse SNS event", "err", err)
		return err
	}
	var errs []error
	for _, record := range records {
		var msg service.WorkerMessage
		if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
			slog.Error("failed to parse SNS message body", "id", record.ID, "err", err)
			errs = append(errs, err)
			continue
		}
		if err := p.Process(ctx, msg); err != nil {
			slog.Error("failed to process message", "id", record.ID, "err", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("config: " + err.Error())
	}

	rootCtx, stop := boot.Context()
	defer stop()

	ac, err := boot.AWSConfig(rootCtx)
	if err != nil {
		panic("aws config: " + err.Error())
	}

	dyn := dynamodb.NewFromConfig(ac)
	stopped := service.NewOrgStopped(erasure.NewStore(dyn, cfg.TablePrefix, 0))
	svc := service.New(service.Clients{
		S3:     s3.NewFromConfig(ac),
		Lambda: lambdaSDK.NewFromConfig(ac, func(o *lambdaSDK.Options) { o.Region = cfg.DfeEgressRegion }),
		Dynamo: dyn,
		SNS:    sns.NewFromConfig(ac),
	}, cfg).WithOrgStopped(stopped).WithCiot(ssm.NewFromConfig(ac), boot.Alerts(rootCtx, "dfe-worker"))

	lambda.StartWithOptions(func(ctx context.Context, raw json.RawMessage) error {
		return handle(ctx, raw, svc)
	}, lambda.WithContext(rootCtx))
}
