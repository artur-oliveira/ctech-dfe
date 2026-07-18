package services

import (
	"context"
	"regexp"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Plate formats: legacy AAA9999, Mercosul AAA9A99.
var plateRe = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`)

// validOwnerTypes mirrors Python VehicleOwnerType enum.
var validOwnerTypes = map[string]bool{"TAC": true, "ETC": true, "CTC": true}

// VehicleService mirrors api/app/services/vehicles.py.
type VehicleService struct {
	repo      *repositories.VehicleRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
	crud      *CRUDMutationHelper
}

func NewVehicleService(repo *repositories.VehicleRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *VehicleService {
	return &VehicleService{
		repo:      repo,
		auditRepo: auditRepo,
		cache:     c,
		crud:      NewCRUDMutationHelper(auditRepo, c),
	}
}

func ValidatePlate(plate string) error {
	p := strings.ToUpper(strings.TrimSpace(plate))
	if !plateRe.MatchString(p) {
		return problem.BadRequest("invalid vehicle plate: " + plate)
	}
	return nil
}

func validateRenavam(renavam string) error {
	d := strings.TrimSpace(renavam)
	if len(d) < 9 || len(d) > 11 {
		return problem.BadRequest("renavam must be 9–11 digits")
	}
	for _, ch := range d {
		if ch < '0' || ch > '9' {
			return problem.BadRequest("renavam must contain only digits")
		}
	}
	return nil
}

func (s *VehicleService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := BuildItemCacheKey(orgPK, "vehicles", sk)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, sk)
	}, "vehicle not found")
}

func (s *VehicleService) List(ctx context.Context, orgPK string, opts repositories.VehicleListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, "vehicles", opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.List(ctx, orgPK, opts)
	})
}

func (s *VehicleService) ListByRole(ctx context.Context, orgPK, role string, opts repositories.VehicleListOpts) (*repositories.QueryResult, error) {
	type listByRoleOpts struct {
		Role string
		Opts repositories.VehicleListOpts
	}
	return GetCachedList(ctx, s.cache, orgPK, "vehicles", listByRoleOpts{Role: role, Opts: opts}, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.ListByRole(ctx, orgPK, role, opts)
	})
}

func strField(fields map[string]any, key string) string {
	v, _ := fields[key].(string)
	return v
}

func intField(fields map[string]any, key string) int {
	switch v := fields[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// Create builds strongly-typed arguments then writes the vehicle and its
// CREATE audit row atomically.
func (s *VehicleService) Create(ctx context.Context, orgPK string, fields map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	plate, _ := fields["plate"].(string)
	if err := ValidatePlate(plate); err != nil {
		return nil, err
	}
	renavam, _ := fields["renavam"].(string)
	if renavam != "" {
		if err := validateRenavam(renavam); err != nil {
			return nil, err
		}
	}
	ownerType, _ := fields["owner_type"].(string)
	if ownerType != "" && !validOwnerTypes[ownerType] {
		return nil, problem.BadRequest("owner_type must be TAC, ETC, or CTC")
	}
	owner, _ := fields["owner"].(map[string]any)
	if nestedType, _ := owner["type"].(string); nestedType != "" && !validOwnerTypes[nestedType] {
		return nil, problem.BadRequest("owner_type must be TAC, ETC, or CTC")
	}

	f := repositories.VehicleFields{
		Plate:    plate,
		PlateUF:  strField(fields, "plate_uf"),
		Role:     strField(fields, "role"),
		Wheelset: strField(fields, "wheelset"),
		Bodywork: strField(fields, "bodywork"),
		Renavam:  renavam,
		Weight:   intField(fields, "weight"),
		CapKG:    intField(fields, "cap_kg"),
		CapM3:    intField(fields, "cap_m3"),
		Cint:     strField(fields, "cint"),
		Owner:    owner,
	}

	return s.crud.Create(ctx, orgPK, repositories.AuditResourceVehicle, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, f)
		return tx, item, nil
	}, s.repo.TransactWrite)
}

// Update writes the vehicle change and its UPDATE audit row atomically.
func (s *VehicleService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	if plate, ok := updates["plate"].(string); ok {
		if err := ValidatePlate(plate); err != nil {
			return nil, err
		}
	}
	if renavam, ok := updates["renavam"].(string); ok && renavam != "" {
		if err := validateRenavam(renavam); err != nil {
			return nil, err
		}
	}
	if ownerType, ok := updates["owner_type"].(string); ok && ownerType != "" && !validOwnerTypes[ownerType] {
		return nil, problem.BadRequest("owner_type must be TAC, ETC, or CTC")
	}

	return s.crud.Update(ctx, orgPK, sk, repositories.AuditResourceVehicle, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	}, s.repo.TransactWrite)
}

// Delete removes the vehicle and writes its DELETE audit row atomically.
func (s *VehicleService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	return s.crud.Delete(ctx, orgPK, sk, repositories.AuditResourceVehicle, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, sk), nil
	}, s.repo.TransactWrite)
}
