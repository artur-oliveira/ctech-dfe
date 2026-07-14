package services

import (
	"context"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ProductService mirrors api/app/services/products.py.
type ProductService struct {
	repo      *repositories.ProductRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
	crud      *CRUDMutationHelper
}

func NewProductService(repo *repositories.ProductRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ProductService {
	return &ProductService{
		repo:      repo,
		auditRepo: auditRepo,
		cache:     c,
		crud:      NewCRUDMutationHelper(auditRepo, c),
	}
}

func (s *ProductService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := BuildItemCacheKey(orgPK, "products", sk)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, sk)
	}, "product not found")
}

func (s *ProductService) List(ctx context.Context, orgPK string, opts repositories.ProductListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, "products", opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.List(ctx, orgPK, opts)
	})
}

// Create writes the product and its CREATE audit row atomically.
func (s *ProductService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Create(ctx, orgPK, repositories.AuditResourceProduct, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, fields)
		return tx, item, nil
	}, s.repo.TransactWrite)
}

// Update writes the product change and its UPDATE audit row atomically.
func (s *ProductService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Update(ctx, orgPK, sk, repositories.AuditResourceProduct, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	}, s.repo.TransactWrite)
}

// Delete removes the product and writes its DELETE audit row atomically.
func (s *ProductService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	return s.crud.Delete(ctx, orgPK, sk, repositories.AuditResourceProduct, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, sk), nil
	}, s.repo.TransactWrite)
}
