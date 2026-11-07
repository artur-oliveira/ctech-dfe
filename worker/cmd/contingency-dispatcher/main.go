// Command contingency-dispatcher lists the documents pending retransmission
// through the sparse contingency-index (no Scan) and invokes the retransmission
// worker once per document (async: Lambda retries twice, then the dlq-processor).
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
	"gopkg.aoctech.app/dfe/worker/internal/contingency"
)

func main() {
	cfg, err := config.LoadContingencyDispatcher()
	if err != nil {
		panic("config: " + err.Error())
	}
	rootCtx, stop := boot.Context()
	defer stop()
	ac, err := boot.AWSConfig(rootCtx)
	if err != nil {
		panic("aws config: " + err.Error())
	}
	db, lc := dynamodb.NewFromConfig(ac), lambdaSDK.NewFromConfig(ac)
	lambda.StartWithOptions(func(ctx context.Context, _ json.RawMessage) error {
		return dispatch(ctx, cfg, db, lc)
	}, lambda.WithContext(rootCtx))
}

type querier interface {
	Query(ctx context.Context, in *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

type invoker interface {
	Invoke(ctx context.Context, in *lambdaSDK.InvokeInput, optFns ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error)
}

func dispatch(ctx context.Context, cfg *config.Config, db querier, inv invoker) error {
	for _, t := range contingency.Tables {
		table := cfg.TablePrefix + "_" + t
		jobs, err := pending(ctx, db, table)
		if err != nil {
			slog.Error("failed to list contingency index", "table", table, "err", err)
			continue // the next 15-minute sweep redoes it
		}
		for _, j := range jobs {
			j.Table = t
			body, _ := json.Marshal(j)
			if _, err := inv.Invoke(ctx, &lambdaSDK.InvokeInput{
				FunctionName:   &cfg.ContingencyFunctionName,
				InvocationType: lambdaTypes.InvocationTypeEvent,
				Payload:        body,
			}); err != nil {
				slog.Error("failed to invoke retransmission", "table", t, "pk", j.PK, "err", err)
			}
		}
	}
	return nil
}

// pending pages through the whole index (it holds only unreconciled documents,
// so it is bounded by the contingency backlog, not by the table).
func pending(ctx context.Context, db querier, table string) ([]contingency.Job, error) {
	var jobs []contingency.Job
	in := &dynamodb.QueryInput{
		TableName:                 &table,
		IndexName:                 new(contingency.IndexName),
		KeyConditionExpression:    new("#s = :v"),
		ExpressionAttributeNames:  map[string]string{"#s": contingency.SweepAttr},
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": &types.AttributeValueMemberS{Value: contingency.SweepValue}},
	}
	for {
		out, err := db.Query(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, it := range out.Items {
			pk, _ := it["pk"].(*types.AttributeValueMemberS)
			sk, _ := it["sk"].(*types.AttributeValueMemberS)
			if pk != nil && sk != nil {
				jobs = append(jobs, contingency.Job{PK: pk.Value, SK: sk.Value})
			}
		}
		if out.LastEvaluatedKey == nil {
			return jobs, nil
		}
		in.ExclusiveStartKey = out.LastEvaluatedKey
	}
}
