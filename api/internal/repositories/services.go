package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// ServiceRepository — organization_services, o catálogo de serviços que a
// emissão de NFS-e consome (análogo a organization_products para NF-e).
type ServiceRepository struct {
	CRUDRepository[map[string]types.AttributeValue]
}

func NewServiceRepository(db *dynamodb.Client, cfg *config.Config) *ServiceRepository {
	return &ServiceRepository{
		CRUDRepository: NewCRUDRepository[map[string]types.AttributeValue](db, cfg, "organization_services"),
	}
}

func buildServiceSK(sk string) string {
	if strings.HasPrefix(sk, "SERVICE_") {
		return sk
	}
	return fmt.Sprintf("SERVICE_%s", sk)
}

func (r *ServiceRepository) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	id := GenerateID()
	return r.CRUDRepository.Create(ctx, orgPK, buildServiceSK(id), fields)
}

func (r *ServiceRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, buildServiceSK(sk))
}

type ServiceListOpts struct {
	DescriptionPrefix string
	CodePrefix        string
	OrderBy           string
	Sort              string
	Limit             int
	StartKey          map[string]types.AttributeValue
}

func (r *ServiceRepository) List(ctx context.Context, orgPK string, opts ServiceListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.DescriptionPrefix != "" || opts.OrderBy == "description" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.DescriptionPrefix,
			IndexName: "description-index", SKField: "description",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	if opts.CodePrefix != "" || opts.OrderBy == "code" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.CodePrefix,
			IndexName: "code-index", SKField: "code",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: "SERVICE_",
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *ServiceRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, buildServiceSK(sk), updates)
}

func (r *ServiceRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, buildServiceSK(sk))
}

// BuildCreateTxItem returns a TransactWriteItem for a new service, mirroring
// Create's key/timestamp construction, without writing.
func (r *ServiceRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	id := GenerateID()
	// marshalEntity never errors for T = map[string]types.AttributeValue (base.go) — safe to discard.
	tx, item, _ := r.CRUDRepository.BuildCreateTxItem(orgPK, buildServiceSK(id), fields)
	return tx, item
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// service, mirroring Update's timestamp bump, without writing.
func (r *ServiceRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, buildServiceSK(sk), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a service, without writing.
func (r *ServiceRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, buildServiceSK(sk))
}
