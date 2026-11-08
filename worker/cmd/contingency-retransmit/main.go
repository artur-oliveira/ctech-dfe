// Command contingency-retransmit handles one document pending retransmission.
// It sends the stored message (contingency_msg: the signed XML as body) through the
// normal authorization path (service.DfeService.Process: egress NFeAutorizacao,
// duplicity reconciliation, S3, status, notification), then classifies the answer with
// contingency.Classify: authorized or authorized late (cStat 150, or 100 past the
// deadline) are successes, a rejection ends the retransmission, no answer past the
// deadline is overdue. Idempotent: a document no longer pending is skipped, and Process
// itself holds the processing lease.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/dfe/worker/internal/boot"
	"gopkg.aoctech.app/dfe/worker/internal/config"
	"gopkg.aoctech.app/dfe/worker/internal/contingency"
	"gopkg.aoctech.app/dfe/worker/internal/service"
)

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
	db := dynamodb.NewFromConfig(ac)
	svc := service.New(service.Clients{
		S3:     s3.NewFromConfig(ac),
		Lambda: lambdaSDK.NewFromConfig(ac, func(o *lambdaSDK.Options) { o.Region = cfg.DfeEgressRegion }),
		Dynamo: db,
		SNS:    sns.NewFromConfig(ac),
	}, cfg).WithAlerter(boot.Alerts(rootCtx, "contingency-retransmit"))
	lambda.StartWithOptions(func(ctx context.Context, j contingency.Job) error {
		if j.Kind == contingency.KindProbe { // health probe of the automatic contingency (spec 4.1)
			return svc.Probe(ctx, service.ProbeJob{Table: j.Table, PK: j.PK})
		}
		return handle(ctx, cfg.TablePrefix, db, svc, j, time.Now())
	}, lambda.WithContext(rootCtx))
}

type store interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// processor is service.DfeService.Process; faked in tests.
type processor interface {
	Process(ctx context.Context, msg service.WorkerMessage) error
}

func str(it map[string]types.AttributeValue, k string) string {
	v, _ := it[k].(*types.AttributeValueMemberS)
	if v == nil {
		return ""
	}
	return v.Value
}

func handle(ctx context.Context, prefix string, db store, send processor, j contingency.Job, now time.Time) error {
	table := prefix + "_" + j.Table
	key := map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: j.PK}, "sk": &types.AttributeValueMemberS{Value: j.SK}}
	get := func() (map[string]types.AttributeValue, error) {
		out, err := db.GetItem(ctx, &dynamodb.GetItemInput{TableName: &table, Key: key, ConsistentRead: new(true)})
		if err != nil {
			return nil, err
		}
		return out.Item, nil
	}
	item, err := get()
	if err != nil {
		return err
	}
	if item == nil || str(item, contingency.StateAttr) != contingency.StatePending {
		return nil // already reconciled: the index entry is stale, nothing to do
	}
	deadline, _ := time.Parse(time.RFC3339, str(item, contingency.DeadlineAttr))

	var msg service.WorkerMessage
	if err := json.Unmarshal([]byte(str(item, contingency.MsgAttr)), &msg); err != nil {
		return err // no stored message: a defect, let the DLQ show it
	}
	// A transport failure inside Process comes back as an error and is retried by the next sweep.
	sendErr := send.Process(ctx, msg)

	after, err := get()
	if err != nil {
		return err
	}
	cStat, _ := strconv.Atoi(str(after, "sefaz_status"))
	answered := cStat != 0
	outcome := contingency.Classify(now, deadline, cStat, answered)
	state := contingency.StateFor(outcome)
	if state == "" { // retry or reconcile: stay pending for the next 15-minute sweep
		if sendErr != nil {
			slog.Warn("contingency resend failed; left pending", "table", j.Table, "pk", j.PK, "err", sendErr)
		}
		return nil
	}
	_, err = db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &table, Key: key,
		UpdateExpression:         new("SET #st = :o REMOVE #sw, #msg"),
		ExpressionAttributeNames: map[string]string{"#st": contingency.StateAttr, "#sw": contingency.SweepAttr, "#msg": contingency.MsgAttr},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":o": &types.AttributeValueMemberS{Value: state},
		},
	})
	if err == nil && outcome == contingency.OutcomeOverdue {
		slog.Error("contingency document overdue", "table", j.Table, "pk", j.PK, "sk", j.SK) // ponytail: go-common alerts when the alert wiring lands
	}
	return err
}
