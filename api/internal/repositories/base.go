// Package repositories provides DynamoDB persistence layer, mirroring
// api/app/repositories/base.py.
//
// Key design rules (from CLAUDE.md):
//   - get_item > query > scan  (no scans in production)
//   - transact_write for NF-e numbering atomicity
//   - Table names are prefixed by environment: {prefix}_{table}
package repositories

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/dynamo"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Base provides common DynamoDB operations for all repositories.
type Base = dynamo.Base

// QueryResult holds paginated query results.
type QueryResult = dynamo.QueryResult

// QueryOpts configures a Query call.
type QueryOpts = dynamo.QueryOpts

// NowStr returns the current UTC time as ISO 8601, matching Python's now_str().
var NowStr = dynamo.NowStr

// MarshalMapOmitNull marshals v into a DynamoDB attribute map, omitting any
// attribute whose value is null (recursively, including nested maps and list
// elements).
var MarshalMapOmitNull = dynamo.MarshalMapOmitNull

// IsConditionFailed reports whether err represents a DynamoDB conditional
// check failure, either from a single-item call or from within a
// TransactWrite (TransactionCanceledException wrapping a condition failure).
// Exported for the services layer to translate into problem.Conflict.
var IsConditionFailed = dynamo.IsConditionFailed

// NewBase creates a Base repository with an environment-prefixed table name.
func NewBase(db *dynamodb.Client, cfg *config.Config, table string) Base {
	return dynamo.NewBase(db, cfg.TablePrefix, table)
}

// wrapDynamoErr formats a raw AWS SDK error the same way dynamo.Base's methods
// do internally. Repositories that issue their own low-level *dynamodb.Client
// calls (bypassing Base's methods for query/scan shapes Base doesn't cover)
// use this to keep error text consistent with wrapped Base calls.
func wrapDynamoErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("dynamodb: %w", err)
}

// CRUDRepository is a generic repository wrapper for CRUD operations on a DynamoDB table.
// This is dfe-specific (org-scoped multi-tenant generic CRUD wrapper) and is not part of
// the shared api-commons library.
type CRUDRepository[T any] struct {
	Base
}

// NewCRUDRepository creates a CRUDRepository.
func NewCRUDRepository[T any](db *dynamodb.Client, cfg *config.Config, tableName string) CRUDRepository[T] {
	return CRUDRepository[T]{
		Base: NewBase(db, cfg, tableName),
	}
}

func (r *CRUDRepository[T]) Create(ctx context.Context, orgPK, sk string, entity T) (map[string]types.AttributeValue, error) {
	now := NowStr()
	item, err := MarshalMapOmitNull(entity)
	if err != nil {
		return nil, err
	}
	item["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	item["sk"] = &types.AttributeValueMemberS{Value: sk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return item, r.PutItem(ctx, item)
}

func (r *CRUDRepository[T]) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, sk)
}

func (r *CRUDRepository[T]) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	updates["updated_at"] = NowStr()
	return r.UpdateItem(ctx, orgPK, &sk, updates)
}

func (r *CRUDRepository[T]) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, sk)
}

func (r *CRUDRepository[T]) BuildCreateTxItem(orgPK, sk string, entity T) (types.TransactWriteItem, map[string]types.AttributeValue, error) {
	now := NowStr()
	item, err := MarshalMapOmitNull(entity)
	if err != nil {
		return types.TransactWriteItem{}, nil, err
	}
	item["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	item["sk"] = &types.AttributeValueMemberS{Value: sk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.BuildPutTxItem(item), item, nil
}

func (r *CRUDRepository[T]) BuildCreateTxItemIfAbsent(orgPK, sk string, entity T) (types.TransactWriteItem, map[string]types.AttributeValue, error) {
	now := NowStr()
	item, err := MarshalMapOmitNull(entity)
	if err != nil {
		return types.TransactWriteItem{}, nil, err
	}
	item["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	item["sk"] = &types.AttributeValueMemberS{Value: sk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.BuildPutTxItemIfAbsent(item), item, nil
}

func (r *CRUDRepository[T]) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(orgPK, &sk, updates)
}

func (r *CRUDRepository[T]) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, sk)
}
