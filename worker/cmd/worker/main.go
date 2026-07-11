package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/artur-oliveira/ctech-dfe/worker/internal/config"
	"github.com/artur-oliveira/ctech-dfe/worker/internal/service"
)

type sqsEvent struct {
	Records []sqsRecord `json:"Records"`
}

type sqsRecord struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
}

type batchResponse struct {
	BatchItemFailures []batchItemFailure `json:"batchItemFailures"`
}

type batchItemFailure struct {
	ItemIdentifier string `json:"itemIdentifier"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("config: " + err.Error())
	}

	ac, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("aws config: " + err.Error())
	}

	svc := service.New(service.Clients{
		S3:     s3.NewFromConfig(ac),
		Lambda: lambdaSDK.NewFromConfig(ac),
		Dynamo: dynamodb.NewFromConfig(ac),
		SNS:    sns.NewFromConfig(ac),
	}, cfg)

	lambda.Start(func(ctx context.Context, raw json.RawMessage) (batchResponse, error) {
		var ev sqsEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			slog.Error("failed to parse SQS event", "err", err)
			return batchResponse{}, err
		}

		var failures []batchItemFailure
		for _, record := range ev.Records {
			var msg service.WorkerMessage
			if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
				slog.Error("failed to parse SQS record body", "id", record.MessageID, "err", err)
				failures = append(failures, batchItemFailure{ItemIdentifier: record.MessageID})
				continue
			}
			if err := svc.Process(ctx, msg); err != nil {
				slog.Error("failed to process message", "id", record.MessageID, "err", err)
				failures = append(failures, batchItemFailure{ItemIdentifier: record.MessageID})
			}
		}
		return batchResponse{BatchItemFailures: failures}, nil
	})
}
