package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const vehicleCacheTTL = 300

// Plate formats: legacy AAA9999, Mercosul AAA9A99.
var plateRe = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`)

// validOwnerTypes mirrors Python VehicleOwnerType enum.
var validOwnerTypes = map[string]bool{"TAC": true, "ETC": true, "CTC": true}

// VehicleService mirrors api/app/services/vehicles.py.
type VehicleService struct {
	repo      *repositories.VehicleRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewVehicleService(repo *repositories.VehicleRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *VehicleService {
	return &VehicleService{repo: repo, auditRepo: auditRepo, cache: c}
}

func vehicleCacheKey(orgPK, sk string) string {
	return fmt.Sprintf("res:%s:vehicles:%s", orgPK, sk)
}

func vehicleListCachePrefix(orgPK string) string {
	return fmt.Sprintf("res:%s:vehicles:", orgPK)
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
	key := vehicleCacheKey(orgPK, sk)
	if v, ok := cacheGetItem(ctx, s.cache, key); ok {
		return v, nil
	}
	item, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("vehicle not found")
	}
	cacheSetItem(ctx, s.cache, key, item, vehicleCacheTTL)
	return item, nil
}

func (s *VehicleService) List(ctx context.Context, orgPK string, opts repositories.VehicleListOpts) (*repositories.QueryResult, error) {
	return s.repo.List(ctx, orgPK, opts)
}

func (s *VehicleService) ListByRole(ctx context.Context, orgPK, role string, opts repositories.VehicleListOpts) (*repositories.QueryResult, error) {
	return s.repo.ListByRole(ctx, orgPK, role, opts)
}

// strField/intField pull a plain string/int out of the untyped fields map
// produced by structToMap, defaulting to the zero value when absent —
// matching the existing "zero value means unset" convention already used by
// resolveVehicle for weight/wheelset/bodywork.
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

	vehicleTx, finalItem := s.repo.BuildCreateTxItem(orgPK, f)

	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceVehicle, attrStrAV(finalItem, "sk"), repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{vehicleTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.DeletePrefix(ctx, vehicleListCachePrefix(orgPK))
	return finalItem, nil
}

// Update writes the vehicle change and its UPDATE audit row atomically.
// Fetches the current item first so only actually-changed fields are logged.
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

	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("vehicle not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}

	vehicleTx, err := s.repo.BuildUpdateTxItem(orgPK, sk, updates)
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
		orgPK, repositories.AuditResourceVehicle, attrStrAV(current, "sk"), repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{vehicleTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.Delete(ctx, vehicleCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, vehicleListCachePrefix(orgPK))
	return s.repo.Get(ctx, orgPK, sk)
}

// Delete removes the vehicle and writes its DELETE audit row atomically.
func (s *VehicleService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return err
	}
	if current == nil {
		return problem.NotFound("vehicle not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return err
	}

	vehicleTx := s.repo.BuildDeleteTxItem(orgPK, sk)
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceVehicle, attrStrAV(current, "sk"), repositories.AuditActionDelete,
		userID, userName, Diff(beforeMap, nil),
	)
	if err != nil {
		return err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{vehicleTx, auditTx}); err != nil {
		return err
	}
	_ = s.cache.Delete(ctx, vehicleCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, vehicleListCachePrefix(orgPK))
	return nil
}
