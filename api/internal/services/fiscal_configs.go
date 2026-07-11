package services

import (
	"context"

	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// resourceID is a fixed constant per config type — these tables hold one item
// per org (no sk), so there's no natural per-item identifier to use instead.
const (
	nfeConfigResourceID  = "nfe_config"
	nfceConfigResourceID = "nfce_config"
	cteConfigResourceID  = "cte_config"
	mdfeConfigResourceID = "mdfe_config"
)

// NfeConfigService wraps NfeConfigRepository.
// Mirrors api/app/services/fiscal_configs.py NfeConfigService.
type NfeConfigService struct {
	repo      *repositories.NfeConfigRepository
	auditRepo *repositories.AuditLogRepository
}

func NewNfeConfigService(repo *repositories.NfeConfigRepository, auditRepo *repositories.AuditLogRepository) *NfeConfigService {
	return &NfeConfigService{repo: repo, auditRepo: auditRepo}
}

func (s *NfeConfigService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	item, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("nfe config not found")
	}
	return item, nil
}

// Upsert writes the config and its CREATE/UPDATE audit row atomically. The
// audit diff compares the pre-existing item against the FINAL merged fields
// (post preserve-field carry-forward), never against the caller's raw input —
// see FiscalConfigRepository.BuildUpsertTxItem.
func (s *NfeConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
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
		orgPK, repositories.AuditResourceNfeConfig, nfeConfigResourceID, action,
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

// NfceConfigService wraps NfceConfigRepository.
type NfceConfigService struct {
	repo      *repositories.NfceConfigRepository
	auditRepo *repositories.AuditLogRepository
}

func NewNfceConfigService(repo *repositories.NfceConfigRepository, auditRepo *repositories.AuditLogRepository) *NfceConfigService {
	return &NfceConfigService{repo: repo, auditRepo: auditRepo}
}

func (s *NfceConfigService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	item, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("nfce config not found")
	}
	return item, nil
}

// Upsert writes the config and its CREATE/UPDATE audit row atomically. See
// NfeConfigService.Upsert for the preserve-field/diff reasoning (identical here).
func (s *NfceConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
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
		orgPK, repositories.AuditResourceNfceConfig, nfceConfigResourceID, action,
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

// CteConfigService wraps CteConfigRepository.
type CteConfigService struct {
	repo      *repositories.CteConfigRepository
	auditRepo *repositories.AuditLogRepository
}

func NewCteConfigService(repo *repositories.CteConfigRepository, auditRepo *repositories.AuditLogRepository) *CteConfigService {
	return &CteConfigService{repo: repo, auditRepo: auditRepo}
}

func (s *CteConfigService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	item, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("cte config not found")
	}
	return item, nil
}

// Upsert writes the config and its CREATE/UPDATE audit row atomically. See
// NfeConfigService.Upsert for the preserve-field/diff reasoning (identical here).
func (s *CteConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
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
		orgPK, repositories.AuditResourceCteConfig, cteConfigResourceID, action,
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

// MdfeConfigService wraps MdfeConfigRepository.
type MdfeConfigService struct {
	repo      *repositories.MdfeConfigRepository
	auditRepo *repositories.AuditLogRepository
}

func NewMdfeConfigService(repo *repositories.MdfeConfigRepository, auditRepo *repositories.AuditLogRepository) *MdfeConfigService {
	return &MdfeConfigService{repo: repo, auditRepo: auditRepo}
}

func (s *MdfeConfigService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	item, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("mdfe config not found")
	}
	return item, nil
}

// Upsert writes the config and its CREATE/UPDATE audit row atomically. See
// NfeConfigService.Upsert for the preserve-field/diff reasoning (identical here).
func (s *MdfeConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
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
		orgPK, repositories.AuditResourceMdfeConfig, mdfeConfigResourceID, action,
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
