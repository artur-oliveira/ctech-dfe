package services

import (
	"context"
	"fmt"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const productCacheTTL = 300

// ProductService mirrors api/app/services/products.py.
type ProductService struct {
	repo      *repositories.ProductRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewProductService(repo *repositories.ProductRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ProductService {
	return &ProductService{repo: repo, auditRepo: auditRepo, cache: c}
}

func productCacheKey(orgPK, sk string) string {
	return fmt.Sprintf("res:%s:products:%s", orgPK, sk)
}

func productListCachePrefix(orgPK string) string {
	return fmt.Sprintf("res:%s:products:", orgPK)
}

func (s *ProductService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := productCacheKey(orgPK, sk)
	if v, ok := cacheGetItem(ctx, s.cache, key); ok {
		return v, nil
	}
	item, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("product not found")
	}
	cacheSetItem(ctx, s.cache, key, item, productCacheTTL)
	return item, nil
}

func (s *ProductService) List(ctx context.Context, orgPK string, opts repositories.ProductListOpts) (*repositories.QueryResult, error) {
	return s.repo.List(ctx, orgPK, opts)
}

// Create writes the product and its CREATE audit row atomically.
func (s *ProductService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	productTx, finalItem := s.repo.BuildCreateTxItem(orgPK, fields)

	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceProduct, attrStrAV(finalItem, "sk"), repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{productTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.DeletePrefix(ctx, productListCachePrefix(orgPK))
	return finalItem, nil
}

// Update writes the product change and its UPDATE audit row atomically. Fetches
// the current item first so only actually-changed fields are logged.
func (s *ProductService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("product not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}

	productTx, err := s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	if err != nil {
		return nil, err
	}
	// updates is a partial map (only the fields the caller wants to change).
	// Merge it over beforeMap so Diff only reports fields that actually
	// changed, instead of treating every omitted field as "changed to nil".
	afterMap := make(map[string]any, len(beforeMap))
	for k, v := range beforeMap {
		afterMap[k] = v
	}
	for k, v := range updates {
		afterMap[k] = v
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceProduct, attrStrAV(current, "sk"), repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{productTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.Delete(ctx, productCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, productListCachePrefix(orgPK))
	return s.repo.Get(ctx, orgPK, sk)
}

// Delete removes the product and writes its DELETE audit row atomically.
func (s *ProductService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return err
	}
	if current == nil {
		return problem.NotFound("product not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return err
	}

	productTx := s.repo.BuildDeleteTxItem(orgPK, sk)
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceProduct, attrStrAV(current, "sk"), repositories.AuditActionDelete,
		userID, userName, Diff(beforeMap, nil),
	)
	if err != nil {
		return err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{productTx, auditTx}); err != nil {
		return err
	}
	_ = s.cache.Delete(ctx, productCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, productListCachePrefix(orgPK))
	return nil
}
