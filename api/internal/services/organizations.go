package services

import (
	"context"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const orgCacheTTL = 300

// OrganizationService mirrors api/app/services/organizations.py.
type OrganizationService struct {
	repo      *repositories.OrganizationRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewOrganizationService(repo *repositories.OrganizationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *OrganizationService {
	return &OrganizationService{repo: repo, auditRepo: auditRepo, cache: c}
}

func (s *OrganizationService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	cacheKey := "org:" + orgPK
	if v, ok := cacheGetItem(ctx, s.cache, cacheKey); ok {
		return v, nil
	}
	item, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item != nil {
		cacheSetItem(ctx, s.cache, cacheKey, item, orgCacheTTL)
	}
	return item, nil
}

func (s *OrganizationService) Create(ctx context.Context, cpfOrCNPJ string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	existing, err := s.Get(ctx, cpfOrCNPJ)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	if err := s.repo.CreateOrganization(ctx, cpfOrCNPJ, fields); err != nil {
		return nil, err
	}
	return s.repo.GetOrganization(ctx, cpfOrCNPJ)
}

// Update writes the organization's company-data change and its UPDATE audit
// row atomically. Fetches the current item first so only actually-changed
// fields are logged.
func (s *OrganizationService) Update(ctx context.Context, orgPK string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	// Read directly from the repository (bypassing the service cache) so the
	// audit "before" snapshot always reflects the latest DynamoDB state, even
	// under concurrent updates hitting different replicas (cache is per-
	// instance, 300s TTL — see api/CLAUDE.md).
	current, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("organization not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}

	orgTx, err := s.repo.BuildUpdateTxItem(orgPK, updates)
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

	pk := attrStrAV(current, "pk")
	auditTx, err := s.auditRepo.BuildLogTxItem(
		pk, repositories.AuditResourceOrganization, pk, repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{orgTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.Delete(ctx, "org:"+orgPK)
	return s.repo.GetOrganization(ctx, orgPK)
}
