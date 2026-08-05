package services

import (
	"context"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ServiceService is the catálogo de serviços consumido pela emissão de NFS-e.
type ServiceService struct {
	repo      *repositories.ServiceRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
	crud      *CRUDMutationHelper
}

func NewServiceService(repo *repositories.ServiceRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ServiceService {
	return &ServiceService{
		repo:      repo,
		auditRepo: auditRepo,
		cache:     c,
		crud:      NewCRUDMutationHelper(auditRepo, c),
	}
}

func (s *ServiceService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := BuildItemCacheKey(orgPK, "services", sk)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, sk)
	}, "service not found")
}

func (s *ServiceService) List(ctx context.Context, orgPK string, opts repositories.ServiceListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, "services", opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.List(ctx, orgPK, opts)
	})
}

// Create writes the service and its CREATE audit row atomically.
func (s *ServiceService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Create(ctx, orgPK, repositories.AuditResourceService, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, fields)
		return tx, item, nil
	}, s.repo.TransactWrite)
}

// Update writes the service change and its UPDATE audit row atomically.
func (s *ServiceService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Update(ctx, orgPK, sk, repositories.AuditResourceService, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	}, s.repo.TransactWrite)
}

// Delete removes the service and writes its DELETE audit row atomically.
func (s *ServiceService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	return s.crud.Delete(ctx, orgPK, sk, repositories.AuditResourceService, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, sk), nil
	}, s.repo.TransactWrite)
}
