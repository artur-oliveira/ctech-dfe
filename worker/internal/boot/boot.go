// Package boot is the one place a worker binary creates its root context and
// wires the shared go-common pieces (AWS config, operator alerts), so no
// handler or service reaches for context.Background().
package boot

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/api-commons/awsconfig"
)

// Context returns the process root context: JSON slog on stdout (go-common
// observability contract) and cancelled on SIGTERM/SIGINT, which is how the
// Lambda runtime announces shutdown. Pass it to lambda.WithContext.
func Context() (context.Context, context.CancelFunc) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// Region is the Lambda region (AWS_REGION is always set by the runtime).
func Region() string {
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	return "us-east-1"
}

// AWSConfig loads the default AWS config through go-common.
func AWSConfig(ctx context.Context) (aws.Config, error) {
	return awsconfig.Load(ctx, Region())
}

// Alerts builds the operator-alert publisher for this Lambda from
// ALERTS_TOPIC_ARN and APP_ENVIRONMENT. Unset or unavailable degrades to
// alerts.Nop (logged): a broken alert channel never stops a job.
func Alerts(ctx context.Context, service string) alerts.Publisher {
	p, err := alerts.New(ctx, Region(), os.Getenv("ALERTS_TOPIC_ARN"), service, os.Getenv("APP_ENVIRONMENT"))
	if err != nil {
		slog.WarnContext(ctx, "alerts: publisher unavailable, operator alerts are log-only", "err", err)
		return alerts.Nop{}
	}
	return p
}
