package services

import (
	"context"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// OrgEntityService is the shared business layer for the reusable registry
// entities. Everything they have in common — cached get/list, audited
// create/update/delete in a single TransactWrite — lives here once; entity
// specific rules live on the concrete service that embeds it.
type OrgEntityService struct {
	repo *repositories.OrgEntityRepository
	crud *CRUDMutationHelper
	// cacheScope is the resource segment of the cache key (BuildItemCacheKey).
	cacheScope string
	// auditResource is the audit_logs resource_type for this entity.
	auditResource string
	// notFound is the message of the 404 returned by Get.
	notFound string
	cache    cache.Backend
}

func newOrgEntityService(
	repo *repositories.OrgEntityRepository,
	auditRepo *repositories.AuditLogRepository,
	c cache.Backend,
	cacheScope, auditResource, notFound string,
) OrgEntityService {
	return OrgEntityService{
		repo:          repo,
		crud:          NewCRUDMutationHelper(auditRepo, c),
		cacheScope:    cacheScope,
		auditResource: auditResource,
		notFound:      notFound,
		cache:         c,
	}
}

func (s *OrgEntityService) Get(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	key := BuildItemCacheKey(orgPK, s.cacheScope, id)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, id)
	}, s.notFound)
}

func (s *OrgEntityService) List(ctx context.Context, orgPK string, opts repositories.OrgEntityListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, s.cacheScope, opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.List(ctx, orgPK, opts)
	})
}

func (s *OrgEntityService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Create(ctx, orgPK, s.auditResource, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, fields)
		return tx, item, nil
	}, s.repo.TransactWrite)
}

func (s *OrgEntityService) Update(ctx context.Context, orgPK, id string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Update(ctx, orgPK, id, s.auditResource, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, id, updates)
	}, s.repo.TransactWrite)
}

func (s *OrgEntityService) Delete(ctx context.Context, orgPK, id, userID, userName string) error {
	return s.crud.Delete(ctx, orgPK, id, s.auditResource, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, id), nil
	}, s.repo.TransactWrite)
}

// ── Concrete registries ──────────────────────────────────────────────────────

// TaxProfileService owns organization_tax_profiles.
type TaxProfileService struct{ OrgEntityService }

func NewTaxProfileService(repo *repositories.TaxProfileRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *TaxProfileService {
	return &TaxProfileService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeTaxProfiles, repositories.AuditResourceTaxProfile, "tax profile not found",
	)}
}

// OperationService owns organization_operations, e é a única das quatro
// entidades com uma regra própria: no máximo uma operação padrão por
// organização.
type OperationService struct {
	OrgEntityService
	repo *repositories.OperationRepository
}

func NewOperationService(repo *repositories.OperationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *OperationService {
	return &OperationService{
		OrgEntityService: newOrgEntityService(
			&repo.OrgEntityRepository, auditRepo, c,
			CacheScopeOperations, repositories.AuditResourceOperation, "operation not found",
		),
		repo: repo,
	}
}

// Create grava a operação e, quando ela vem marcada como padrão, desmarca a
// anterior **no mesmo TransactWrite** — duas operações padrão deixariam a UI
// escolhendo por sorte de ordenação.
func (s *OperationService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	clear, err := s.clearDefaultTxItems(ctx, orgPK, fields, "")
	if err != nil {
		return nil, err
	}
	return s.crud.Create(ctx, orgPK, s.auditResource, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, fields)
		return tx, item, nil
	}, s.transactWith(clear))
}

// Update segue a mesma regra do Create para is_default.
func (s *OperationService) Update(ctx context.Context, orgPK, id string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	av, err := repositories.MarshalMapOmitNull(updates)
	if err != nil {
		return nil, problem.InternalServer(err.Error())
	}
	clear, err := s.clearDefaultTxItems(ctx, orgPK, av, s.repo.SK(id))
	if err != nil {
		return nil, err
	}
	return s.crud.Update(ctx, orgPK, id, s.auditResource, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, id, updates)
	}, s.transactWith(clear))
}

// clearDefaultTxItems devolve os updates que desmarcam a operação padrão atual,
// ou nada quando os campos recebidos não pedem para virar padrão.
func (s *OperationService) clearDefaultTxItems(
	ctx context.Context, orgPK string, fields map[string]types.AttributeValue, skipSK string,
) ([]types.TransactWriteItem, error) {
	flag, ok := fields[repositories.OperationIsDefaultField].(*types.AttributeValueMemberBOOL)
	if !ok || !flag.Value {
		return nil, nil
	}
	current, err := s.repo.ListDefaults(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	var items []types.TransactWriteItem
	for _, item := range current {
		sk, _ := item["sk"].(*types.AttributeValueMemberS)
		if sk == nil || sk.Value == skipSK {
			continue
		}
		tx, err := s.repo.BuildUpdateTxItem(orgPK, sk.Value,
			map[string]any{repositories.OperationIsDefaultField: false})
		if err != nil {
			return nil, err
		}
		items = append(items, tx)
	}
	return items, nil
}

// transactWith devolve um TransactWrite que prefixa os itens extras — assim a
// desmarcação da operação padrão anterior é atômica com a escrita nova.
func (s *OperationService) transactWith(extra []types.TransactWriteItem) func(context.Context, []types.TransactWriteItem) error {
	return func(ctx context.Context, items []types.TransactWriteItem) error {
		return s.repo.TransactWrite(ctx, append(extra, items...))
	}
}

// Cache scopes for the registry entities (segment of the cache key).
const (
	CacheScopeTaxProfiles  = "tax_profiles"
	CacheScopeOperations   = "operations"
	CacheScopePaymentTerms = "payment_terms"
	CacheScopeVehicleSets  = "vehicle_sets"
)
