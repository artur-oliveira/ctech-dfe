package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

type ProductRepository struct {
	Base
}

func NewProductRepository(db *dynamodb.Client, cfg *config.Config) *ProductRepository {
	return &ProductRepository{Base: NewBase(db, cfg, "organization_products")}
}

func buildProductSK(sk string) string {
	if strings.HasPrefix(sk, "PRODUCT_") {
		return sk
	}
	return fmt.Sprintf("PRODUCT_%s", sk)
}

func (r *ProductRepository) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	now := NowStr()
	id := GenerateID()
	fields["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	fields["sk"] = &types.AttributeValueMemberS{Value: buildProductSK(id)}
	fields["created_at"] = &types.AttributeValueMemberS{Value: now}
	fields["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return fields, r.PutItem(ctx, fields)
}

func (r *ProductRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, buildProductSK(sk))
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
	updates["updated_at"] = NowStr()
	return r.UpdateItem(ctx, orgPK, new(buildProductSK(sk)), updates)
}

func (r *ProductRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, buildProductSK(sk))
}

// BuildCreateTxItem returns a TransactWriteItem for a new product, mirroring
// Create's key/timestamp construction, without writing.
func (r *ProductRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	now := NowStr()
	id := GenerateID()
	fields["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	fields["sk"] = &types.AttributeValueMemberS{Value: buildProductSK(id)}
	fields["created_at"] = &types.AttributeValueMemberS{Value: now}
	fields["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.BuildPutTxItem(fields), fields
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// product, mirroring Update's timestamp bump, without writing.
func (r *ProductRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(orgPK, new(buildProductSK(sk)), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a product, without writing.
func (r *ProductRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, buildProductSK(sk))
}

func strPtr(s string) *string { return &s }
