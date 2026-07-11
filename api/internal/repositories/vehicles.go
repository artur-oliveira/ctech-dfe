package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

type VehicleRepository struct {
	Base
}

func NewVehicleRepository(db *dynamodb.Client, cfg *config.Config) *VehicleRepository {
	return &VehicleRepository{Base: NewBase(db, cfg, "organization_vehicles")}
}

func buildVehicleSK(sk string) string {
	if strings.HasPrefix(sk, "VEHICLE_") {
		return sk
	}
	return fmt.Sprintf("VEHICLE_%s", sk)
}

// VehicleFields is the flat set of attributes stored on organization_vehicles.
// Only Plate, PlateUF and Role are ever required; everything else may be the
// zero value (unset) at cadastro time and completed later — see
// services.Missing for what each doc-type/role actually requires at emission.
type VehicleFields struct {
	Plate    string
	PlateUF  string
	Role     string // "tractor" | "trailer"
	Wheelset string
	Bodywork string
	Renavam  string
	Weight   int
	CapKG    int
	CapM3    int
	Cint     string
	Owner    map[string]any
}

// buildItem assembles the item to store. Optional fields are omitted
// entirely (not written as empty-string/zero placeholders) — DynamoDB
// rejects an empty string on any attribute that backs a GSI key (role,
// plate), and omission also keeps "unset" distinguishable from "explicitly
// zero" for services.Missing.
func (r *VehicleRepository) buildItem(orgPK, sk string, f VehicleFields, now string) map[string]types.AttributeValue {
	ownerAV, _ := attributevalue.MarshalMap(f.Owner)
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: orgPK},
		"sk":         &types.AttributeValueMemberS{Value: sk},
		"plate":      &types.AttributeValueMemberS{Value: f.Plate},
		"plate_uf":   &types.AttributeValueMemberS{Value: f.PlateUF},
		"owner":      &types.AttributeValueMemberM{Value: ownerAV},
		"created_at": &types.AttributeValueMemberS{Value: now},
		"updated_at": &types.AttributeValueMemberS{Value: now},
	}
	if f.Role != "" {
		item["role"] = &types.AttributeValueMemberS{Value: f.Role}
	}
	if f.Wheelset != "" {
		item["wheelset"] = &types.AttributeValueMemberS{Value: f.Wheelset}
	}
	if f.Bodywork != "" {
		item["bodywork"] = &types.AttributeValueMemberS{Value: f.Bodywork}
	}
	if f.Renavam != "" {
		item["renavam"] = &types.AttributeValueMemberS{Value: f.Renavam}
	}
	if f.Cint != "" {
		item["cint"] = &types.AttributeValueMemberS{Value: f.Cint}
	}
	if f.Weight != 0 {
		item["weight"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", f.Weight)}
	}
	if f.CapKG != 0 {
		item["cap_kg"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", f.CapKG)}
	}
	if f.CapM3 != 0 {
		item["cap_m3"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", f.CapM3)}
	}
	deleteNulls(item)
	return item
}

func (r *VehicleRepository) Create(ctx context.Context, orgPK string, f VehicleFields) (map[string]types.AttributeValue, error) {
	now := NowStr()
	item := r.buildItem(orgPK, buildVehicleSK(GenerateID()), f, now)
	return item, r.PutItem(ctx, item)
}

func (r *VehicleRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, buildVehicleSK(sk))
}

type VehicleListOpts struct {
	PlatePrefix string
	Sort        string
	Limit       int
	StartKey    map[string]types.AttributeValue
}

// List returns vehicles for an org, optionally filtered by plate prefix.
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

// ListByRole returns vehicles for an org filtered to one role ("tractor" or
// "trailer") via role-index, without a Scan.
func (r *VehicleRepository) ListByRole(ctx context.Context, orgPK, role string, opts VehicleListOpts) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: role,
		IndexName: "role-index", SKField: "role",
		ScanIndexForward: opts.Sort != "desc", Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *VehicleRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	updates["updated_at"] = NowStr()
	return r.UpdateItem(ctx, orgPK, new(buildVehicleSK(sk)), updates)
}

func (r *VehicleRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, buildVehicleSK(sk))
}

// BuildCreateTxItem returns a TransactWriteItem for a new vehicle, mirroring
// Create's key/timestamp/field construction, without writing.
func (r *VehicleRepository) BuildCreateTxItem(orgPK string, f VehicleFields) (types.TransactWriteItem, map[string]types.AttributeValue) {
	now := NowStr()
	item := r.buildItem(orgPK, buildVehicleSK(GenerateID()), f, now)
	return r.BuildPutTxItem(item), item
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// vehicle, mirroring Update's timestamp bump, without writing.
func (r *VehicleRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(orgPK, new(buildVehicleSK(sk)), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a vehicle, without writing.
func (r *VehicleRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, buildVehicleSK(sk))
}
