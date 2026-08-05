package services

import (
	"context"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// resourceID is a fixed constant per config type — these tables hold one item
// per org (no sk), so there's no natural per-item identifier to use instead.
const (
	nfeConfigResourceID  = "nfe_config"
	nfceConfigResourceID = "nfce_config"
	cteConfigResourceID  = "cte_config"
	mdfeConfigResourceID = "mdfe_config"
	nfseConfigResourceID = "nfse_config"
)

// fiscalConfigRepo é a parte de FiscalConfigRepository que o serviço usa.
// A assinatura de TransactWrite vem de dynamo.Base:
//
//	func (b *Base) TransactWrite(ctx context.Context, items []types.TransactWriteItem) error
//
// Os cinco repositórios de config a satisfazem por embedding.
type fiscalConfigRepo interface {
	Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error)
	BuildUpsertTxItem(orgPK string, fields map[string]types.AttributeValue, existing map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue, error)
	TransactWrite(ctx context.Context, items []types.TransactWriteItem) error
}

// fiscalConfigService implementa Get/Upsert para qualquer config fiscal
// singleton. Antes cada variante repetia o mesmo Upsert; a lógica de auditoria
// (diff contra os campos FINAIS, pós carry-forward dos campos preservados) é
// sutil o bastante para não valer cinco cópias — ver
// FiscalConfigRepository.BuildUpsertTxItem.
type fiscalConfigService struct {
	repo         fiscalConfigRepo
	auditRepo    *repositories.AuditLogRepository
	resourceType string
	resourceID   string
	notFoundMsg  string
}

func (s *fiscalConfigService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	item, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound(s.notFoundMsg)
	}
	return item, nil
}

// Upsert writes the config and its CREATE/UPDATE audit row atomically. The
// audit diff compares the pre-existing item against the FINAL merged fields
// (post preserve-field carry-forward), never against the caller's raw input.
// The single Get below feeds both the preserve-merge and the audit baseline:
// two independent reads could straddle a concurrent internal-process write
// (e.g. a counter increment) and misattribute it to the acting user.
func (s *fiscalConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	action := repositories.AuditActionUpdate
	var beforeMap map[string]any
	if current == nil {
		action = repositories.AuditActionCreate
	} else {
		beforeMap, err = attributeMapToPlain(current)
		if err != nil {
			return nil, err
		}
	}

	configTx, finalItem, err := s.repo.BuildUpsertTxItem(orgPK, fields, current)
	if err != nil {
		return nil, err
	}
	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}

	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, s.resourceType, s.resourceID, action,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{configTx, auditTx}); err != nil {
		return nil, err
	}
	return finalItem, nil
}

// NfeConfigService wraps NfeConfigRepository.
type NfeConfigService struct{ fiscalConfigService }

func NewNfeConfigService(repo *repositories.NfeConfigRepository, auditRepo *repositories.AuditLogRepository) *NfeConfigService {
	return &NfeConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceNfeConfig, resourceID: nfeConfigResourceID,
		notFoundMsg: "nfe config not found",
	}}
}

// NfceConfigService wraps NfceConfigRepository.
type NfceConfigService struct{ fiscalConfigService }

func NewNfceConfigService(repo *repositories.NfceConfigRepository, auditRepo *repositories.AuditLogRepository) *NfceConfigService {
	return &NfceConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceNfceConfig, resourceID: nfceConfigResourceID,
		notFoundMsg: "nfce config not found",
	}}
}

// CteConfigService wraps CteConfigRepository.
type CteConfigService struct{ fiscalConfigService }

func NewCteConfigService(repo *repositories.CteConfigRepository, auditRepo *repositories.AuditLogRepository) *CteConfigService {
	return &CteConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceCteConfig, resourceID: cteConfigResourceID,
		notFoundMsg: "cte config not found",
	}}
}

// MdfeConfigService wraps MdfeConfigRepository.
type MdfeConfigService struct{ fiscalConfigService }

func NewMdfeConfigService(repo *repositories.MdfeConfigRepository, auditRepo *repositories.AuditLogRepository) *MdfeConfigService {
	return &MdfeConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceMdfeConfig, resourceID: mdfeConfigResourceID,
		notFoundMsg: "mdfe config not found",
	}}
}

// NfseConfigService wraps NfseConfigRepository.
type NfseConfigService struct{ fiscalConfigService }

func NewNfseConfigService(repo *repositories.NfseConfigRepository, auditRepo *repositories.AuditLogRepository) *NfseConfigService {
	return &NfseConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceNfseConfig, resourceID: nfseConfigResourceID,
		notFoundMsg: "nfse config not found",
	}}
}
