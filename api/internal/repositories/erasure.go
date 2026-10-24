package repositories

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/dynamo"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// ErasureRepository is the account-deletion purge's access to every table by
// name. The purge is table-driven (services/erasure_catalog.go), so it works on
// keys and pages instead of the typed repositories, which each know one table.
type ErasureRepository struct {
	db     *dynamodb.Client
	prefix string
}

func NewErasureRepository(db *dynamodb.Client, cfg *config.Config) *ErasureRepository {
	return &ErasureRepository{db: db, prefix: cfg.TablePrefix}
}

// Page is one page of items and the key to continue from (nil at the end).
type Page struct {
	Items []map[string]types.AttributeValue
	Next  map[string]types.AttributeValue
}

const (
	erasurePageSize      = 100
	batchWriteMax        = 25
	batchWriteMaxRetries = 5
)

// SAV is a string attribute value.
func SAV(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

func (r *ErasureRepository) name(table string) *string {
	return aws.String(dynamo.TableName(r.prefix, table))
}

func (r *ErasureRepository) GetItem(ctx context.Context, table string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: r.name(table), Key: key, ConsistentRead: aws.Bool(true)})
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	return out.Item, nil
}

func (r *ErasureRepository) query(ctx context.Context, in *dynamodb.QueryInput) (Page, error) {
	in.Limit = aws.Int32(erasurePageSize)
	out, err := r.db.Query(ctx, in)
	if err != nil {
		return Page{}, wrapDynamoErr(err)
	}
	return Page{Items: out.Items, Next: out.LastEvaluatedKey}, nil
}

func (r *ErasureRepository) QueryPartition(ctx context.Context, table, pk string, start map[string]types.AttributeValue) (Page, error) {
	return r.query(ctx, &dynamodb.QueryInput{
		TableName:                 r.name(table),
		KeyConditionExpression:    aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": SAV(pk)},
		ExclusiveStartKey:         start,
		ConsistentRead:            aws.Bool(true),
	})
}

func (r *ErasureRepository) QueryPrefix(ctx context.Context, table, pk, skPrefix string, start map[string]types.AttributeValue) (Page, error) {
	return r.query(ctx, &dynamodb.QueryInput{
		TableName:                 r.name(table),
		KeyConditionExpression:    aws.String("pk = :pk AND begins_with(sk, :p)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": SAV(pk), ":p": SAV(skPrefix)},
		ExclusiveStartKey:         start,
		ConsistentRead:            aws.Bool(true),
	})
}

func (r *ErasureRepository) QueryIndex(ctx context.Context, table, index, attr, value string, start map[string]types.AttributeValue) (Page, error) {
	return r.query(ctx, &dynamodb.QueryInput{
		TableName:                 r.name(table),
		IndexName:                 aws.String(index),
		KeyConditionExpression:    aws.String("#k = :v"),
		ExpressionAttributeNames:  map[string]string{"#k": attr},
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": SAV(value)},
		ExclusiveStartKey:         start,
	})
}

func (r *ErasureRepository) DeleteKeys(ctx context.Context, table string, keys []map[string]types.AttributeValue) error {
	for len(keys) > 0 {
		batch := keys[:min(batchWriteMax, len(keys))]
		keys = keys[len(batch):]
		reqs := make([]types.WriteRequest, 0, len(batch))
		for _, k := range batch {
			reqs = append(reqs, types.WriteRequest{DeleteRequest: &types.DeleteRequest{Key: k}})
		}
		pending := map[string][]types.WriteRequest{*r.name(table): reqs}
		for attempt := 0; len(pending) > 0; attempt++ {
			if attempt == batchWriteMaxRetries {
				return fmt.Errorf("dynamodb: %s: deletes still unprocessed after %d attempts", table, attempt)
			}
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
			}
			out, err := r.db.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{RequestItems: pending})
			if err != nil {
				return wrapDynamoErr(err)
			}
			pending = out.UnprocessedItems
		}
	}
	return nil
}

func (r *ErasureRepository) DeleteKey(ctx context.Context, table string, key map[string]types.AttributeValue) (bool, error) {
	out, err := r.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: r.name(table), Key: key, ReturnValues: types.ReturnValueAllOld,
	})
	if err != nil {
		return false, wrapDynamoErr(err)
	}
	return len(out.Attributes) > 0, nil
}

// SetAttrs overwrites attrs on an existing row. A missing row is left missing:
// an anonymization must never create the row the purge already deleted.
func (r *ErasureRepository) SetAttrs(ctx context.Context, table string, key, attrs map[string]types.AttributeValue) error {
	names := make([]string, 0, len(attrs))
	for n := range attrs {
		names = append(names, n)
	}
	sort.Strings(names)
	exprNames := map[string]string{}
	exprValues := map[string]types.AttributeValue{}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		nk, vk := fmt.Sprintf("#a%d", i), fmt.Sprintf(":v%d", i)
		exprNames[nk], exprValues[vk] = n, attrs[n]
		parts = append(parts, nk+" = "+vk)
	}
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: r.name(table), Key: key,
		UpdateExpression:          aws.String("SET " + strings.Join(parts, ", ")),
		ConditionExpression:       aws.String("attribute_exists(pk)"),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})
	if IsConditionFailed(err) {
		return nil
	}
	return wrapDynamoErr(err)
}
