package main

import (
	"context"
	"encoding/json"
	"gopkg.aoctech.app/api-commons/erasure"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/dfe/worker/internal/boot"
	"gopkg.aoctech.app/dfe/worker/internal/config"
	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type processor interface {
	Process(ctx context.Context, msg service.DistributionMessage) error
}

// handle processes a distribution job. The payload is the job itself: the API and
// the dispatcher invoke this function asynchronously (InvocationType Event), so a
// failure is returned to Lambda's async retry / dlq-processor destination. Jobs are
// idempotent (NSU cursors and writes are conditional, quota is re-checked here).
func handle(ctx context.Context, raw json.RawMessage, p processor) error {
	if service.IsPingEvent(raw) {
		slog.Info("ping: keep-warm invoke, no-op")
		return nil
	}
	var msg service.DistributionMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		slog.Error("failed to parse distribution message", "err", err)
		return err
	}
	if err := p.Process(ctx, msg); err != nil {
		slog.Error("failed to process distribution message", "job_type", msg.JobType, "org_pk", msg.OrgPK, "err", err)
		return err
	}
	return nil
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
	svc := service.NewDistribution(service.DistributionClients{
		S3:     s3.NewFromConfig(ac),
		Lambda: lambdaSDK.NewFromConfig(ac, func(o *lambdaSDK.Options) { o.Region = cfg.DfeEgressRegion }),
		Dynamo: dyn,
		SNS:    sns.NewFromConfig(ac),
	}, cfg).WithOrgStopped(stopped)

	lambda.StartWithOptions(func(ctx context.Context, raw json.RawMessage) error {
		return handle(ctx, raw, svc)
	}, lambda.WithContext(rootCtx))
}
