package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

type ProductRepository struct {
	CRUDRepository[map[string]types.AttributeValue]
}

func NewProductRepository(db *dynamodb.Client, cfg *config.Config) *ProductRepository {
	return &ProductRepository{
		CRUDRepository: NewCRUDRepository[map[string]types.AttributeValue](db, cfg, "organization_products"),
	}
}

func buildProductSK(sk string) string {
	if strings.HasPrefix(sk, "PRODUCT_") {
		return sk
	}
	return fmt.Sprintf("PRODUCT_%s", sk)
}

func (r *ProductRepository) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	id := GenerateID()
	return r.CRUDRepository.Create(ctx, orgPK, buildProductSK(id), fields)
}

func (r *ProductRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, buildProductSK(sk))
}

type ProductListOpts struct {
	DescriptionPrefix string
	CodePrefix        string
	OrderBy           string
	Sort              string
	Limit             int
	StartKey          map[string]types.AttributeValue
}

func (r *ProductRepository) List(ctx context.Context, orgPK string, opts ProductListOpts) (*QueryResult, error) {
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
		PK: orgPK, SKPrefix: "PRODUCT_",
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *ProductRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, buildProductSK(sk), updates)
}

func (r *ProductRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, buildProductSK(sk))
}

// BuildCreateTxItem returns a TransactWriteItem for a new product, mirroring
// Create's key/timestamp construction, without writing.
func (r *ProductRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	id := GenerateID()
	tx, item, _ := r.CRUDRepository.BuildCreateTxItem(orgPK, buildProductSK(id), fields)
	return tx, item
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// product, mirroring Update's timestamp bump, without writing.
func (r *ProductRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, buildProductSK(sk), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a product, without writing.
func (r *ProductRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, buildProductSK(sk))
}
