// Command contingency-retransmit handles one document pending retransmission.
// ponytail: only the deadline part is wired (a document past its deadline is
// marked overdue and leaves the index). The resend through the egress needs the
// signed, untransmitted XML that the offline emission slice (D) stores; until
// then the index is empty and a pending job is left for the next sweep.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/boot"
	"gopkg.aoctech.app/dfe/worker/internal/contingency"
)

func main() {
	prefix := os.Getenv("TABLE_PREFIX")
	if prefix == "" {
		panic("TABLE_PREFIX is required")
	}
	rootCtx, stop := boot.Context()
	defer stop()
	ac, err := boot.AWSConfig(rootCtx)
	if err != nil {
		panic("aws config: " + err.Error())
	}
	db := dynamodb.NewFromConfig(ac)
	lambda.StartWithOptions(func(ctx context.Context, j contingency.Job) error {
		return handle(ctx, prefix, db, j, time.Now())
	}, lambda.WithContext(rootCtx))
}

type store interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func str(it map[string]types.AttributeValue, k string) string {
	v, _ := it[k].(*types.AttributeValueMemberS)
	if v == nil {
		return ""
	}
	return v.Value
}

func handle(ctx context.Context, prefix string, db store, j contingency.Job, now time.Time) error {
	table := prefix + "_" + j.Table
	key := map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: j.PK}, "sk": &types.AttributeValueMemberS{Value: j.SK}}
	out, err := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &table, Key: key})
	if err != nil {
		return err
	}
	if out.Item == nil || str(out.Item, contingency.StateAttr) != contingency.StatePending {
		return nil // already reconciled: the index entry is stale, nothing to do
	}
	deadline, _ := time.Parse(time.RFC3339, str(out.Item, contingency.DeadlineAttr))
	if contingency.Classify(now, deadline, 0, false) != contingency.OutcomeOverdue {
		slog.Warn("contingency resend not wired yet; left pending", "table", j.Table, "pk", j.PK)
		return nil
	}
	_, err = db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &table, Key: key,
		UpdateExpression:         new("SET #st = :o REMOVE #sw"),
		ExpressionAttributeNames: map[string]string{"#st": contingency.StateAttr, "#sw": contingency.SweepAttr},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":o": &types.AttributeValueMemberS{Value: contingency.StateOverdue},
		},
	})
	if err == nil {
		slog.Error("contingency document overdue", "table", j.Table, "pk", j.PK, "sk", j.SK) // ponytail: go-common alerts when the alert wiring lands
	}
	return err
}
