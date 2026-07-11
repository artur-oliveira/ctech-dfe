// Package services contains business logic for the API layer.
package services

import (
	"context"
	"encoding/json"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// cacheGet deserializes a cached JSON value into T. Returns zero value if miss.
func cacheGet[T any](ctx context.Context, c cache.Backend, key string) (*T, bool) {
	data, ok, err := c.Get(ctx, key)
	if err != nil || !ok || len(data) == 0 {
		return nil, false
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}
	return &v, true
}

// cacheSet serializes v to JSON and stores it with the given TTL (seconds).
func cacheSet[T any](ctx context.Context, c cache.Backend, key string, v T, ttl int) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = c.Set(ctx, key, data, ttl)
}

// cacheGetItem retrieves a DynamoDB item from cache.
// types.AttributeValue is a closed interface and cannot round-trip through encoding/json,
// so we store items as map[string]any (via attributevalue.UnmarshalMap) and restore
// them with attributevalue.MarshalMap on retrieval.
func cacheGetItem(ctx context.Context, c cache.Backend, key string) (map[string]types.AttributeValue, bool) {
	data, ok, err := c.Get(ctx, key)
	if err != nil || !ok || len(data) == 0 {
		return nil, false
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, false
	}
	item, err := attributevalue.MarshalMap(generic)
	if err != nil {
		return nil, false
	}
	return item, true
}

// cacheSetItem stores a DynamoDB item in cache by first converting it to map[string]any.
func cacheSetItem(ctx context.Context, c cache.Backend, key string, item map[string]types.AttributeValue, ttl int) {
	var generic map[string]any
	if err := attributevalue.UnmarshalMap(item, &generic); err != nil {
		return
	}
	data, err := json.Marshal(generic)
	if err != nil {
		return
	}
	_ = c.Set(ctx, key, data, ttl)
}
