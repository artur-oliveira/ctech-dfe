// Command webhook-deliverer delivers the webhooks that are due. EventBridge
// Scheduler runs it every minute; each run reads the sparse due-index of the
// `webhooks` table (a handful of rows, not the history), attempts what is due
// through the shared engine (go-common/webhooks) and returns. The state lives in
// the table, so a run that fails loses nothing: the next one picks up the same
// rows. docs/specs/2026-11-15-webhooks.md.
//
// It lives in the api module because the repository it drives is api/internal;
// the CDK builds it with the same bundler as the worker Lambdas.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/api-commons/awsconfig"
	"gopkg.aoctech.app/api-commons/webhooks"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	defaultBatch   = 200 // deliveries read per run; the rest waits one minute
	defaultWorkers = 8   // endpoints delivered to at once
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	region := os.Getenv("AWS_REGION")
	ac, err := awsconfig.Load(rootCtx, region)
	if err != nil {
		panic("aws config: " + err.Error())
	}
	key, err := secretKey(rootCtx, ssm.NewFromConfig(ac), os.Getenv("WEBHOOK_SECRET_KEY_SSM_PATH"))
	if err != nil {
		panic("webhook secret key: " + err.Error())
	}
	repo := repositories.NewWebhookRepository(dynamodb.NewFromConfig(ac), &config.Config{
		TablePrefix: os.Getenv("TABLE_PREFIX"), WebhookSecretKey: key,
	})
	if !repo.Enabled() {
		panic("webhook secret key is not a valid fieldcrypt key")
	}

	pub, err := alerts.New(rootCtx, region, os.Getenv("ALERTS_TOPIC_ARN"), "dfe-webhooks", os.Getenv("APP_ENVIRONMENT"))
	if err != nil {
		slog.WarnContext(rootCtx, "alerts: publisher unavailable, operator alerts are log-only", "err", err)
		pub = alerts.Nop{}
	}
	engine := services.NewWebhookEngine(repo, func(e repositories.WebhookEndpoint) {
		pub.Alert(rootCtx, alerts.Alert{
			Job:     "webhook-disabled",
			Summary: "webhook endpoint disabled after consecutive failures",
			Detail:  "organization " + e.OrganizationID + ", endpoint " + e.ID,
		})
	}, envInt("WEBHOOK_WORKERS", defaultWorkers))

	batch := envInt("WEBHOOK_BATCH", defaultBatch)
	lambda.StartWithOptions(func(ctx context.Context, _ json.RawMessage) error {
		return run(ctx, engine, batch, time.Now())
	}, lambda.WithContext(rootCtx))
}

type parameterReader interface {
	GetParameter(ctx context.Context, in *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// secretKey reads the key that opens the endpoints' sealed secrets. Read once per
// cold start, never written to the environment or the logs.
func secretKey(ctx context.Context, c parameterReader, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("WEBHOOK_SECRET_KEY_SSM_PATH is not set")
	}
	out, err := c.GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(path), WithDecryption: aws.Bool(true)})
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return aws.ToString(out.Parameter.Value), nil
}

func envInt(name string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n > 0 {
		return n
	}
	return def
}

// run is one pass. A delivery a receiver refuses is not an error here: it is
// recorded on its row and retried with backoff. What fails the run is this job
// being unable to do its work (the queue unreadable, a row it cannot update).
func run(ctx context.Context, engine *webhooks.Deliverer, limit int, now time.Time) error {
	res := engine.Deliver(ctx, limit, now)
	for _, e := range res.Errors {
		slog.ErrorContext(ctx, "webhook delivery error", "error", e)
	}
	slog.InfoContext(ctx, "webhook delivery finished",
		"examined", res.Examined, "handled", res.Handled, "deferred", res.Deferred, "failed", res.Failed)
	if len(res.Errors) > 0 {
		return fmt.Errorf("%d job-level errors, first: %s", len(res.Errors), res.Errors[0])
	}
	return nil
}
