package repositories

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

type PersonRepository struct {
	CRUDRepository[map[string]any]
}

func NewPersonRepository(db *dynamodb.Client, cfg *config.Config) *PersonRepository {
	return &PersonRepository{
		CRUDRepository: NewCRUDRepository[map[string]any](db, cfg, "organization_persons"),
	}
}

func (r *PersonRepository) Create(ctx context.Context, orgPK, sk string, fields map[string]any) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Create(ctx, orgPK, sk, fields)
}

func (r *PersonRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, sk)
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
	return r.CRUDRepository.Update(ctx, orgPK, sk, updates)
}

func (r *PersonRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, sk)
}

func (r *PersonRepository) BuildCreateTxItem(orgPK, sk string, fields map[string]any) (types.TransactWriteItem, map[string]types.AttributeValue) {
	tx, item, _ := r.CRUDRepository.BuildCreateTxItemIfAbsent(orgPK, sk, fields)
	return tx, item
}

func (r *PersonRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, sk, updates)
}

func (r *PersonRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, sk)
}
