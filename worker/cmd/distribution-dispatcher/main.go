package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"gopkg.aoctech.app/dfe/worker/internal/config"
)

var docTypes = []string{"nfe", "cte", "mdfe", "nfse"}

func main() {
	cfg, err := config.LoadDispatcher()
	if err != nil {
		panic("config: " + err.Error())
	}

	ac, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("aws config: " + err.Error())
	}

	dynamo := dynamodb.NewFromConfig(ac)
	sqsClient := sqs.NewFromConfig(ac)

	lambda.Start(func(ctx context.Context, _ json.RawMessage) error {
		return dispatch(ctx, cfg, dynamo, sqsClient)
	})
}

func dispatch(
	ctx context.Context,
	cfg *config.Config,
	dynamo *dynamodb.Client,
	sqsClient *sqs.Client,
) error {
	for _, docType := range docTypes {
		table := cfg.TablePrefix + "_organization_" + docType + "_configs"

		orgPKs, err := scanOrgPKs(ctx, dynamo, table)
		if err != nil {
			slog.Error("failed to scan config table", "table", table, "err", err)
			continue
		}

		for _, orgPK := range orgPKs {
			msg, _ := json.Marshal(map[string]any{
				"job_type": "dist_nsu",
				"org_pk":   orgPK,
				"doc_type": docType,
				"trigger":  "scheduler",
			})
			_, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
				QueueUrl:    &cfg.DistributionQueueURL,
				MessageBody: new(string(msg)),
			})
			if err != nil {
				slog.Error("failed to enqueue distribution job", "org_pk", orgPK, "doc_type", docType, "err", err)
				continue
			}
			slog.Info("enqueued distribution job", "org_pk", orgPK, "doc_type", docType)
		}
	}
	return nil
}

// scanOrgPKs scans the config table and returns all org PKs (auto-paginates).
func scanOrgPKs(ctx context.Context, dynamo *dynamodb.Client, table string) ([]string, error) {
	var orgPKs []string
	input := &dynamodb.ScanInput{
		TableName:            &table,
		ProjectionExpression: new("pk"),
	}
	for {
		out, err := dynamo.Scan(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, item := range out.Items {
			if pk, ok := item["pk"].(*types.AttributeValueMemberS); ok && pk.Value != "" {
				orgPKs = append(orgPKs, pk.Value)
			}
		}
		if out.LastEvaluatedKey == nil {
			break
		}
		input.ExclusiveStartKey = out.LastEvaluatedKey
	}
	return orgPKs, nil
}
