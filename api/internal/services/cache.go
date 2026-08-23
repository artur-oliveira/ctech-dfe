// Package services contains business logic for the API layer.
package services

import (
	"context"
	"encoding/json"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/observability"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CacheGet deserializes a cached JSON value into T. Returns zero value if miss.
// Exported so the per-doc-type service packages (nfses, …) cache through the
// same helper instead of hand-rolling JSON + TTL handling.
func CacheGet[T any](ctx context.Context, c cache.Backend, key string) (*T, bool) {
	data, ok, err := c.Get(ctx, key)
	if err != nil {
		observability.Warn(ctx, "cache read failed", err, "cache_key", key)
		return nil, false
	}
	if !ok || len(data) == 0 {
		return nil, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		observability.Warn(ctx, "cached value decode failed", err, "cache_key", key)
		return nil, false
	}
	return &v, true
}

// CacheSet serializes v to JSON and stores it with the given TTL (seconds).
func CacheSet[T any](ctx context.Context, c cache.Backend, key string, v T, ttl int) {
	data, err := json.Marshal(v)
	if err != nil {
		observability.Warn(ctx, "cache value serialization failed", err, "cache_key", key)
		return
	}
	if err := c.Set(ctx, key, data, ttl); err != nil {
		observability.Warn(ctx, "cache write failed", err, "cache_key", key)
	}
}

func cacheDelete(ctx context.Context, c cache.Backend, key string) {
	if err := c.Delete(ctx, key); err != nil {
		observability.Warn(ctx, "cache invalidation failed", err, "cache_key", key)
	}
}

// cacheGetItem retrieves a DynamoDB item from cache.
// types.AttributeValue is a closed interface and cannot round-trip through encoding/json,
// so we store items as map[string]any (via attributevalue.UnmarshalMap) and restore
// them with attributevalue.MarshalMap on retrieval.
func cacheGetItem(ctx context.Context, c cache.Backend, key string) (map[string]types.AttributeValue, bool) {
	data, ok, err := c.Get(ctx, key)
	if err != nil {
		observability.Warn(ctx, "cache item read failed", err, "cache_key", key)
		return nil, false
	}
	if !ok || len(data) == 0 {
		return nil, false
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		observability.Warn(ctx, "cached item decode failed", err, "cache_key", key)
		return nil, false
	}
	item, err := attributevalue.MarshalMap(generic)
	if err != nil {
		observability.Warn(ctx, "cached item attribute conversion failed", err, "cache_key", key)
		return nil, false
	}
	return item, true
}

// cacheSetItem stores a DynamoDB item in cache by first converting it to map[string]any.
func cacheSetItem(ctx context.Context, c cache.Backend, key string, item map[string]types.AttributeValue, ttl int) {
	var generic map[string]any
	if err := attributevalue.UnmarshalMap(item, &generic); err != nil {
		observability.Warn(ctx, "cache item attribute conversion failed", err, "cache_key", key)
		return
	}
	data, err := json.Marshal(generic)
	if err != nil {
		observability.Warn(ctx, "cache item serialization failed", err, "cache_key", key)
		return
	}
	if err := c.Set(ctx, key, data, ttl); err != nil {
		observability.Warn(ctx, "cache item write failed", err, "cache_key", key)
	}
}
