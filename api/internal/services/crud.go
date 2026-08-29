package services

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// QueryResultJSON is the JSON-serializable representation of QueryResult.
type QueryResultJSON struct {
	Items            []map[string]any `json:"items"`
	LastEvaluatedKey map[string]any   `json:"last_evaluated_key,omitempty"`
}

// BuildListCacheKey builds a unique key for a list query within an organization.
func BuildListCacheKey[T any](orgPK, resourceType string, opts T) string {
	data, err := json.Marshal(opts)
	if err != nil {
		return fmt.Sprintf("dfe:res:%s:%s:list:default", orgPK, resourceType)
	}
	hasher := md5.New()
	hasher.Write(data)
	hash := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf("dfe:res:%s:%s:list:%s", orgPK, resourceType, hash)
}

// BuildItemCacheKey returns the cache key for a single item.
func BuildItemCacheKey(orgPK, resourceType, sk string) string {
	return fmt.Sprintf("dfe:res:%s:%s:%s", orgPK, resourceType, sk)
}

// BuildListCachePrefix returns the prefix to evict all list cache keys for a resource type.
func BuildListCachePrefix(orgPK, resourceType string) string {
	return fmt.Sprintf("dfe:res:%s:%s:list:", orgPK, resourceType)
}

// BuildContextCachePrefix returns the prefix to evict all cache keys (items and lists) in the org context.
func BuildContextCachePrefix(orgPK, resourceType string) string {
	return fmt.Sprintf("dfe:res:%s:%s:", orgPK, resourceType)
}

// CacheGetQueryResult retrieves a QueryResult from the cache.
func CacheGetQueryResult(ctx context.Context, c cache.Backend, key string) (*repositories.QueryResult, bool) {
	data, ok, err := c.Get(ctx, key)
	if err != nil || !ok || len(data) == 0 {
		return nil, false
	}
	var qrJSON QueryResultJSON
	if err := json.Unmarshal(data, &qrJSON); err != nil {
		return nil, false
	}
	items := make([]map[string]types.AttributeValue, len(qrJSON.Items))
	for i, itemMap := range qrJSON.Items {
		av, err := attributevalue.MarshalMap(itemMap)
		if err != nil {
			return nil, false
		}
		items[i] = av
	}
	var lastEvaluatedKey map[string]types.AttributeValue
	if qrJSON.LastEvaluatedKey != nil {
		lek, err := attributevalue.MarshalMap(qrJSON.LastEvaluatedKey)
		if err != nil {
			return nil, false
		}
		lastEvaluatedKey = lek
	}
	return &repositories.QueryResult{
		Items:            items,
		LastEvaluatedKey: lastEvaluatedKey,
	}, true
}

// CacheSetQueryResult stores a QueryResult in the cache.
func CacheSetQueryResult(ctx context.Context, c cache.Backend, key string, qr *repositories.QueryResult, ttl int) {
	items := make([]map[string]any, len(qr.Items))
	for i, itemAV := range qr.Items {
		var m map[string]any
		if err := attributevalue.UnmarshalMap(itemAV, &m); err != nil {
			observability.Warn(ctx, "query result item conversion failed", err, "cache_key", key)
			return
		}
		items[i] = m
	}
	var lastEvaluatedKey map[string]any
	if qr.LastEvaluatedKey != nil {
		if err := attributevalue.UnmarshalMap(qr.LastEvaluatedKey, &lastEvaluatedKey); err != nil {
			observability.Warn(ctx, "query cursor conversion failed", err, "cache_key", key)
			return
		}
	}
	qrJSON := QueryResultJSON{
		Items:            items,
		LastEvaluatedKey: lastEvaluatedKey,
	}
	data, err := json.Marshal(qrJSON)
	if err != nil {
		observability.Warn(ctx, "query result serialization failed", err, "cache_key", key)
		return
	}
	if err := c.Set(ctx, key, data, ttl); err != nil {
		observability.Warn(ctx, "query result cache write failed", err, "cache_key", key)
	}
}

// GetCachedItem encapsulates the item detail caching pattern.
func GetCachedItem(
	ctx context.Context,
	c cache.Backend,
	key string,
	fetch func(ctx context.Context) (map[string]types.AttributeValue, error),
	notFoundMsg string,
) (map[string]types.AttributeValue, error) {
	if v, ok := cacheGetItem(ctx, c, key); ok {
		return v, nil
	}
	item, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound(notFoundMsg)
	}
	cacheSetItem(ctx, c, key, item, 300)
	return item, nil
}

// GetCachedList encapsulates the list query caching pattern.
func GetCachedList[TOpts any](
	ctx context.Context,
	c cache.Backend,
	orgPK, resourceType string,
	opts TOpts,
	fetch func(ctx context.Context) (*repositories.QueryResult, error),
) (*repositories.QueryResult, error) {
	key := BuildListCacheKey(orgPK, resourceType, opts)
	if v, ok := CacheGetQueryResult(ctx, c, key); ok {
		return v, nil
	}
	res, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	CacheSetQueryResult(ctx, c, key, res, 300)
	return res, nil
}

// CRUDMutationHelper wraps standard cache-evicting, audited mutations.
type CRUDMutationHelper struct {
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewCRUDMutationHelper(auditRepo *repositories.AuditLogRepository, c cache.Backend) *CRUDMutationHelper {
	return &CRUDMutationHelper{auditRepo: auditRepo, cache: c}
}

func (h *CRUDMutationHelper) Create(
	ctx context.Context,
	orgPK string,
	resourceType string,
	userID, userName string,
	buildTx func() (types.TransactWriteItem, map[string]types.AttributeValue, error),
	transactWrite func(ctx context.Context, items []types.TransactWriteItem) error,
) (map[string]types.AttributeValue, error) {
	txItem, finalItem, err := buildTx()
	if err != nil {
		return nil, err
	}
	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}
	sk := attrStrAV(finalItem, "sk")
	auditTx, err := h.auditRepo.BuildLogTxItem(
		orgPK, resourceType, sk, repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}
	if err := transactWrite(ctx, []types.TransactWriteItem{txItem, auditTx}); err != nil {
		return nil, err
	}
	if err := h.cache.DeletePrefix(ctx, BuildContextCachePrefix(orgPK, strings.ToLower(resourceType)+"s")); err != nil {
		observability.Warn(ctx, "resource list cache invalidation failed", err, "resource_type", resourceType)
	}
	return finalItem, nil
}

func (h *CRUDMutationHelper) Update(
	ctx context.Context,
	orgPK, sk string,
	resourceType string,
	updates map[string]any,
	userID, userName string,
	getFn func(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error),
	buildTx func(ctx context.Context) (types.TransactWriteItem, error),
	transactWrite func(ctx context.Context, items []types.TransactWriteItem) error,
) (map[string]types.AttributeValue, error) {
	current, err := getFn(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound(fmt.Sprintf("%s not found", strings.ToLower(resourceType)))
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}
	txItem, err := buildTx(ctx)
	if err != nil {
		return nil, err
	}
	afterMap := make(map[string]any, len(beforeMap))
	for k, v := range beforeMap {
		afterMap[k] = v
	}
	for k, v := range updates {
		afterMap[k] = v
	}
	auditTx, err := h.auditRepo.BuildLogTxItem(
		orgPK, resourceType, attrStrAV(current, "sk"), repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}
	if err := transactWrite(ctx, []types.TransactWriteItem{txItem, auditTx}); err != nil {
		return nil, err
	}
	prefix := BuildContextCachePrefix(orgPK, strings.ToLower(resourceType)+"s")
	if err := h.cache.DeletePrefix(ctx, prefix); err != nil {
		observability.Warn(ctx, "resource cache invalidation failed", err, "resource_type", resourceType)
	}
	return getFn(ctx, orgPK, sk)
}

func (h *CRUDMutationHelper) Delete(
	ctx context.Context,
	orgPK, sk string,
	resourceType string,
	userID, userName string,
	getFn func(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error),
	buildTx func(ctx context.Context) (types.TransactWriteItem, error),
	transactWrite func(ctx context.Context, items []types.TransactWriteItem) error,
) error {
	current, err := getFn(ctx, orgPK, sk)
	if err != nil {
		return err
	}
	if current == nil {
		return problem.NotFound(fmt.Sprintf("%s not found", strings.ToLower(resourceType)))
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return err
	}
	txItem, err := buildTx(ctx)
	if err != nil {
		return err
	}
	auditTx, err := h.auditRepo.BuildLogTxItem(
		orgPK, resourceType, attrStrAV(current, "sk"), repositories.AuditActionDelete,
		userID, userName, Diff(beforeMap, nil),
	)
	if err != nil {
		return err
	}
	if err := transactWrite(ctx, []types.TransactWriteItem{txItem, auditTx}); err != nil {
		return err
	}
	prefix := BuildContextCachePrefix(orgPK, strings.ToLower(resourceType)+"s")
	if err := h.cache.DeletePrefix(ctx, prefix); err != nil {
		observability.Warn(ctx, "resource cache invalidation failed", err, "resource_type", resourceType)
	}
	return nil
}
