package repositories

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

type PersonRepository struct {
	Base
}

func NewPersonRepository(db *dynamodb.Client, cfg *config.Config) *PersonRepository {
	return &PersonRepository{Base: NewBase(db, cfg, "organization_persons")}
}

func (r *PersonRepository) Create(ctx context.Context, orgPK, sk string, fields map[string]any) (map[string]types.AttributeValue, error) {
	now := NowStr()
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: orgPK},
		"sk":         &types.AttributeValueMemberS{Value: sk},
		"created_at": &types.AttributeValueMemberS{Value: now},
		"updated_at": &types.AttributeValueMemberS{Value: now},
	}
	for k, v := range fields {
		if _, exists := item[k]; exists {
			continue
		}
		if v == nil {
			continue // omit null attributes
		}
		av, err := attributevalue.Marshal(v)
		if err == nil {
			item[k] = av
		}
	}
	return item, r.PutItem(ctx, item)
}

func (r *PersonRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, sk)
}

type PersonListOpts struct {
	NamePrefix string
	Sort       string
	Limit      int
	StartKey   map[string]types.AttributeValue
}

func (r *PersonRepository) List(ctx context.Context, orgPK string, opts PersonListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.NamePrefix != "" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.NamePrefix,
			IndexName: "org-name-index", SKField: "name",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, ScanIndexForward: forward,
		Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *PersonRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	updates["updated_at"] = NowStr()
	return r.UpdateItem(ctx, orgPK, &sk, updates)
}

func (r *PersonRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, sk)
}

// BuildCreateTxItem returns a TransactWriteItem for a new person, mirroring
// Create's key/timestamp/field construction, without writing.
func (r *PersonRepository) BuildCreateTxItem(orgPK, sk string, fields map[string]any) (types.TransactWriteItem, map[string]types.AttributeValue) {
	now := NowStr()
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: orgPK},
		"sk":         &types.AttributeValueMemberS{Value: sk},
		"created_at": &types.AttributeValueMemberS{Value: now},
		"updated_at": &types.AttributeValueMemberS{Value: now},
	}
	for k, v := range fields {
		if _, exists := item[k]; exists {
			continue
		}
		if v == nil {
			continue // omit null attributes
		}
		av, err := attributevalue.Marshal(v)
		if err == nil {
			item[k] = av
		}
	}
	return r.BuildPutTxItem(item), item
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// person, mirroring Update's timestamp bump, without writing.
func (r *PersonRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(orgPK, &sk, updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a person, without writing.
func (r *PersonRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, sk)
}
