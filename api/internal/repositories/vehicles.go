package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

type VehicleRepository struct {
	CRUDRepository[VehicleFields]
}

func NewVehicleRepository(db *dynamodb.Client, cfg *config.Config) *VehicleRepository {
	return &VehicleRepository{
		CRUDRepository: NewCRUDRepository[VehicleFields](db, cfg, "organization_vehicles"),
	}
}

func buildVehicleSK(sk string) string {
	if strings.HasPrefix(sk, "VEHICLE_") {
		return sk
	}
	return fmt.Sprintf("VEHICLE_%s", sk)
}

type VehicleFields struct {
	Plate    string         `dynamodbav:"plate"`
	PlateUF  string         `dynamodbav:"plate_uf"`
	Role     string         `dynamodbav:"role,omitempty"` // "tractor" | "trailer"
	Wheelset string         `dynamodbav:"wheelset,omitempty"`
	Bodywork string         `dynamodbav:"bodywork,omitempty"`
	Renavam  string         `dynamodbav:"renavam,omitempty"`
	Weight   int            `dynamodbav:"weight,omitempty"`
	CapKG    int            `dynamodbav:"cap_kg,omitempty"`
	CapM3    int            `dynamodbav:"cap_m3,omitempty"`
	Cint     string         `dynamodbav:"cint,omitempty"`
	Owner    map[string]any `dynamodbav:"owner"`
}

func (r *VehicleRepository) Create(ctx context.Context, orgPK string, f VehicleFields) (map[string]types.AttributeValue, error) {
	id := GenerateID()
	return r.CRUDRepository.Create(ctx, orgPK, buildVehicleSK(id), f)
}

func (r *VehicleRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, buildVehicleSK(sk))
}

type VehicleListOpts struct {
	PlatePrefix string
	Sort        string
	Limit       int
	StartKey    map[string]types.AttributeValue
}

func (r *VehicleRepository) List(ctx context.Context, orgPK string, opts VehicleListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.PlatePrefix != "" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.PlatePrefix,
			IndexName: "plate-index", SKField: "plate",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: "VEHICLE_",
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *VehicleRepository) ListByRole(ctx context.Context, orgPK, role string, opts VehicleListOpts) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: role,
		IndexName: "role-index", SKField: "role",
		ScanIndexForward: opts.Sort != "desc", Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *VehicleRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, buildVehicleSK(sk), updates)
}

func (r *VehicleRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, buildVehicleSK(sk))
}

func (r *VehicleRepository) BuildCreateTxItem(orgPK string, f VehicleFields) (types.TransactWriteItem, map[string]types.AttributeValue) {
	id := GenerateID()
	tx, item, _ := r.CRUDRepository.BuildCreateTxItem(orgPK, buildVehicleSK(id), f)
	return tx, item
}

func (r *VehicleRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, buildVehicleSK(sk), updates)
}

func (r *VehicleRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, buildVehicleSK(sk))
}
