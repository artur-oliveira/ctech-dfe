package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"gopkg.aoctech.app/dfe/worker/internal/boot"
	"gopkg.aoctech.app/dfe/worker/internal/config"
)

var docTypes = []string{"nfe", "cte", "mdfe", "nfse"}

func main() {
	cfg, err := config.LoadDispatcher()
	if err != nil {
		panic("config: " + err.Error())
	}

	rootCtx, stop := boot.Context()
	defer stop()

	ac, err := boot.AWSConfig(rootCtx)
	if err != nil {
		panic("aws config: " + err.Error())
	}

	dynamo := dynamodb.NewFromConfig(ac)
	lambdaClient := lambdaSDK.NewFromConfig(ac)

	lambda.StartWithOptions(func(ctx context.Context, _ json.RawMessage) error {
		return dispatch(ctx, cfg, dynamo, lambdaClient)
	}, lambda.WithContext(rootCtx))
}

// sweepIndex/sweepAttr/sweepValue mirror api/internal/repositories (AttrSweep,
// SweepValue, SweepIndex): the API stamps sweep = "1" on every fiscal-config row and
// the `sweep-index` GSI lists them, so no Scan is needed to enumerate organizations.
const (
	sweepIndex = "sweep-index"
	sweepAttr  = "sweep"
	sweepValue = "1"
)

type orgQuerier interface {
	Query(ctx context.Context, in *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// jobInvoker starts one async (InvocationType Event) run of the distribution worker.
// Lambda queues the event, retries it twice and hands it to the dlq-processor
// destination on failure — the same resilience the SQS queue + DLQ used to give.
type jobInvoker interface {
	Invoke(ctx context.Context, in *lambdaSDK.InvokeInput, optFns ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error)
}

func dispatch(
	ctx context.Context,
	cfg *config.Config,
	db orgQuerier,
	invoker jobInvoker,
) error {
	for _, docType := range docTypes {
		table := cfg.TablePrefix + "_organization_" + docType + "_configs"

		orgPKs, err := queryOrgPKs(ctx, db, table)
		if err != nil {
			slog.Error("failed to list config table", "table", table, "err", err)
			continue
		}

		for _, orgPK := range orgPKs {
			msg, _ := json.Marshal(map[string]any{
				"job_type": "dist_nsu",
				"org_pk":   orgPK,
				"doc_type": docType,
				"trigger":  "scheduler",
			})
			// Async invoke returns 202 once Lambda has queued the event; a failure here is
			// logged and skipped like the SQS send was — the next 30-minute sweep redoes it.
			_, err := invoker.Invoke(ctx, &lambdaSDK.InvokeInput{
				FunctionName:   &cfg.DistributionFunctionName,
				InvocationType: lambdaTypes.InvocationTypeEvent,
				Payload:        msg,
			})
			if err != nil {
				slog.Error("failed to invoke distribution job", "org_pk", orgPK, "doc_type", docType, "err", err)
				continue
			}
			slog.Info("invoked distribution job", "org_pk", orgPK, "doc_type", docType)
		}
	}
	return nil
}

// queryOrgPKs lists every org PK of a config table through the sweep-index
// (auto-paginates). The index is KEYS_ONLY, so the table's pk comes back as-is.
func queryOrgPKs(ctx context.Context, db orgQuerier, table string) ([]string, error) {
	var orgPKs []string
	input := &dynamodb.QueryInput{
		TableName:                 &table,
		IndexName:                 new(sweepIndex),
		KeyConditionExpression:    new("#s = :v"),
		ExpressionAttributeNames:  map[string]string{"#s": sweepAttr},
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": &types.AttributeValueMemberS{Value: sweepValue}},
	}
	for {
		out, err := db.Query(ctx, input)
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
