package services

import (
	"context"
	"fmt"

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

// VehicleSetService owns organization_vehicle_sets. A regra própria: os
// veículos referenciados têm que existir e ter o papel certo — um reboque no
// lugar do trator vira rejeição da SEFAZ, não erro de cadastro.
type VehicleSetService struct {
	OrgEntityService
	vehicleRepo *repositories.VehicleRepository
}

func NewVehicleSetService(
	repo *repositories.VehicleSetRepository, vehicleRepo *repositories.VehicleRepository,
	auditRepo *repositories.AuditLogRepository, c cache.Backend,
) *VehicleSetService {
	return &VehicleSetService{
		OrgEntityService: newOrgEntityService(
			&repo.OrgEntityRepository, auditRepo, c,
			CacheScopeVehicleSets, repositories.AuditResourceVehicleSet, "vehicle set not found",
		),
		vehicleRepo: vehicleRepo,
	}
}

func (s *VehicleSetService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := s.validateMembers(ctx, orgPK, fields); err != nil {
		return nil, err
	}
	return s.OrgEntityService.Create(ctx, orgPK, fields, userID, userName)
}

func (s *VehicleSetService) Update(ctx context.Context, orgPK, id string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	av, err := repositories.MarshalMapOmitNull(updates)
	if err != nil {
		return nil, problem.InternalServer(err.Error())
	}
	if err := s.validateMembers(ctx, orgPK, av); err != nil {
		return nil, err
	}
	return s.OrgEntityService.Update(ctx, orgPK, id, updates, userID, userName)
}

// validateMembers confere que o trator é um tractor e cada reboque é um trailer.
func (s *VehicleSetService) validateMembers(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) error {
	if sk, ok := fields[VehicleSetTractorField].(*types.AttributeValueMemberS); ok && sk.Value != "" {
		if err := s.requireRole(ctx, orgPK, sk.Value, VehicleRoleTractor); err != nil {
			return err
		}
	}
	trailers, ok := fields[VehicleSetTrailersField].(*types.AttributeValueMemberL)
	if !ok {
		return nil
	}
	for _, item := range trailers.Value {
		sk, ok := item.(*types.AttributeValueMemberS)
		if !ok || sk.Value == "" {
			continue
		}
		if err := s.requireRole(ctx, orgPK, sk.Value, VehicleRoleTrailer); err != nil {
			return err
		}
	}
	return nil
}

func (s *VehicleSetService) requireRole(ctx context.Context, orgPK, sk, want string) error {
	vehicle, err := s.vehicleRepo.Get(ctx, orgPK, sk)
	if err != nil {
		return err
	}
	if vehicle == nil {
		return problem.BadRequest("veículo não encontrado: " + sk)
	}
	role, _ := vehicle[VehicleRoleField].(*types.AttributeValueMemberS)
	if role == nil || role.Value != want {
		return problem.BadRequest(fmt.Sprintf("veículo %s não tem o papel %q exigido pela composição", sk, want))
	}
	return nil
}

// Campos e papéis da composição veicular.
const (
	VehicleSetTractorField  = "tractor_sk"
	VehicleSetTrailersField = "trailer_sks"
	VehicleRoleField        = "role"
)

// PaymentTermService owns organization_payment_terms.
type PaymentTermService struct{ OrgEntityService }

func NewPaymentTermService(repo *repositories.PaymentTermRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *PaymentTermService {
	return &PaymentTermService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopePaymentTerms, repositories.AuditResourcePaymentTerm, "payment term not found",
	)}
}

// PaymentTerminalService owns organization_payment_terminals.
type PaymentTerminalService struct{ OrgEntityService }

func NewPaymentTerminalService(repo *repositories.PaymentTerminalRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *PaymentTerminalService {
	return &PaymentTerminalService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopePaymentTerminals, repositories.AuditResourcePaymentTerminal, "payment terminal not found",
	)}
}

// TollProviderService owns organization_toll_providers.
type TollProviderService struct{ OrgEntityService }

func NewTollProviderService(repo *repositories.TollProviderRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *TollProviderService {
	return &TollProviderService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeTollProviders, repositories.AuditResourceTollProvider, "toll provider not found",
	)}
}

// CargoUnitService owns organization_cargo_units.
type CargoUnitService struct{ OrgEntityService }

func NewCargoUnitService(repo *repositories.CargoUnitRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *CargoUnitService {
	return &CargoUnitService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeCargoUnits, repositories.AuditResourceCargoUnit, "cargo unit not found",
	)}
}

// ImportDeclarationService owns organization_import_declarations.
type ImportDeclarationService struct{ OrgEntityService }

func NewImportDeclarationService(repo *repositories.ImportDeclarationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ImportDeclarationService {
	return &ImportDeclarationService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeImportDIs, repositories.AuditResourceImportDI, "import declaration not found",
	)}
}

// FuelPumpService owns organization_fuel_pumps.
type FuelPumpService struct{ OrgEntityService }

func NewFuelPumpService(repo *repositories.FuelPumpRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *FuelPumpService {
	return &FuelPumpService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeFuelPumps, repositories.AuditResourceFuelPump, "fuel pump not found",
	)}
}

// ProductLotService owns organization_product_lots.
type ProductLotService struct{ OrgEntityService }

func NewProductLotService(repo *repositories.ProductLotRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ProductLotService {
	return &ProductLotService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeProductLots, repositories.AuditResourceProductLot, "product lot not found",
	)}
}

// InsurancePolicyService owns organization_insurance_policies.
type InsurancePolicyService struct{ OrgEntityService }

func NewInsurancePolicyService(repo *repositories.InsurancePolicyRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *InsurancePolicyService {
	return &InsurancePolicyService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeInsurancePolicies, repositories.AuditResourceInsurance, "insurance policy not found",
	)}
}

// ServiceLocationService owns organization_service_locations — obra, imóvel e
// local de evento da NFS-e num cadastro só.
type ServiceLocationService struct{ OrgEntityService }

func NewServiceLocationService(repo *repositories.ServiceLocationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ServiceLocationService {
	return &ServiceLocationService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeServiceLocations, repositories.AuditResourceServiceLocation, "service location not found",
	)}
}

// ReferenceDocumentService owns organization_reference_documents — os
// documentos citados em dedução/redução e em reembolso/repasse/ressarcimento.
type ReferenceDocumentService struct{ OrgEntityService }

func NewReferenceDocumentService(repo *repositories.ReferenceDocumentRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ReferenceDocumentService {
	return &ReferenceDocumentService{newOrgEntityService(
		&repo.OrgEntityRepository, auditRepo, c,
		CacheScopeReferenceDocs, repositories.AuditResourceReferenceDoc, "reference document not found",
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
	CacheScopeTaxProfiles       = "tax_profiles"
	CacheScopeOperations        = "operations"
	CacheScopePaymentTerms      = "payment_terms"
	CacheScopeVehicleSets       = "vehicle_sets"
	CacheScopePaymentTerminals  = "payment_terminals"
	CacheScopeTollProviders     = "toll_providers"
	CacheScopeCargoUnits        = "cargo_units"
	CacheScopeImportDIs         = "import_declarations"
	CacheScopeInsurancePolicies = "insurance_policies"
	CacheScopeProductLots       = "product_lots"
	CacheScopeFuelPumps         = "fuel_pumps"
	CacheScopeServiceLocations  = "service_locations"
	CacheScopeReferenceDocs     = "reference_documents"
)
