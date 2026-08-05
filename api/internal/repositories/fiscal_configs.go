package repositories

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// NfeConfigRepository — organization_nfe_configs.
// preserve: prod_nsu, hom_nsu, prod_last_dist_nsu_at, hom_last_dist_nsu_at
type NfeConfigRepository struct {
	FiscalConfigRepository
}

func NewNfeConfigRepository(db *dynamodb.Client, cfg *config.Config) *NfeConfigRepository {
	return &NfeConfigRepository{
		FiscalConfigRepository: newFiscalConfigBase(db, cfg, "organization_nfe_configs", map[string]any{
			"prod_nsu":              0,
			"hom_nsu":               0,
			"prod_last_dist_nsu_at": nil,
			"hom_last_dist_nsu_at":  nil,
		}),
	}
}

// NfceConfigRepository — organization_nfce_configs (no preserve fields).
type NfceConfigRepository struct {
	FiscalConfigRepository
}

func NewNfceConfigRepository(db *dynamodb.Client, cfg *config.Config) *NfceConfigRepository {
	return &NfceConfigRepository{
		FiscalConfigRepository: newFiscalConfigBase(db, cfg, "organization_nfce_configs", nil),
	}
}

// CteConfigRepository — organization_cte_configs.
type CteConfigRepository struct {
	FiscalConfigRepository
}

func NewCteConfigRepository(db *dynamodb.Client, cfg *config.Config) *CteConfigRepository {
	return &CteConfigRepository{
		FiscalConfigRepository: newFiscalConfigBase(db, cfg, "organization_cte_configs", map[string]any{
			"prod_nsu":              0,
			"hom_nsu":               0,
			"prod_last_dist_nsu_at": nil,
			"hom_last_dist_nsu_at":  nil,
		}),
	}
}

// MdfeConfigRepository — organization_mdfe_configs.
type MdfeConfigRepository struct {
	FiscalConfigRepository
}

func NewMdfeConfigRepository(db *dynamodb.Client, cfg *config.Config) *MdfeConfigRepository {
	return &MdfeConfigRepository{
		FiscalConfigRepository: newFiscalConfigBase(db, cfg, "organization_mdfe_configs", map[string]any{
			"prod_nsu":              0,
			"hom_nsu":               0,
			"prod_last_dist_nsu_at": nil,
			"hom_last_dist_nsu_at":  nil,
		}),
	}
}

// NfseConfigRepository — organization_nfse_configs.
// preserve: contadores de numeração da DPS/RPS, atualizados pela emissão, e cursor NSU da distribuição ADN.
type NfseConfigRepository struct {
	FiscalConfigRepository
}

func NewNfseConfigRepository(db *dynamodb.Client, cfg *config.Config) *NfseConfigRepository {
	return &NfseConfigRepository{
		FiscalConfigRepository: newFiscalConfigBase(db, cfg, "organization_nfse_configs", map[string]any{
			"prod_current_number":   0,
			"hom_current_number":    0,
			"prod_nsu":              0,
			"hom_nsu":               0,
			"prod_last_dist_nsu_at": nil,
			"hom_last_dist_nsu_at":  nil,
		}),
	}
}
