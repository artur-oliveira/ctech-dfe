package repositories

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// DistributionRepository stores DFe distribution (NF-e/CT-e/MDF-e received) records.
// PK: {env}#{org_pk} — NSU is the numeric sort key (NOT the standard string "sk").
type DistributionRepository struct {
	Base
	// db is kept alongside Base for ListDistributions' custom sort-key query
	// shape, which Base.Query does not cover (Base's db field is unexported
	// in the shared api-commons/dynamo package).
	db *dynamodb.Client
}

func newDistributionRepo(db *dynamodb.Client, cfg *config.Config, table string) DistributionRepository {
	return DistributionRepository{Base: NewBase(db, cfg, table), db: db}
}

// NFeDistributionRepository wraps DistributionRepository for nfe_distributions table.
type NFeDistributionRepository struct{ DistributionRepository }

func NewNfeDistributionRepository(db *dynamodb.Client, cfg *config.Config) *NFeDistributionRepository {
	r := newDistributionRepo(db, cfg, "nfe_distributions")
	return &NFeDistributionRepository{r}
}

// CTeDistributionRepository wraps DistributionRepository for cte_distributions table.
type CTeDistributionRepository struct{ DistributionRepository }

func NewCteDistributionRepository(db *dynamodb.Client, cfg *config.Config) *CTeDistributionRepository {
	r := newDistributionRepo(db, cfg, "cte_distributions")
	return &CTeDistributionRepository{r}
}

// MDFeDistributionRepository wraps DistributionRepository for mdfe_distributions table.
type MDFeDistributionRepository struct{ DistributionRepository }

func NewMdfeDistributionRepository(db *dynamodb.Client, cfg *config.Config) *MDFeDistributionRepository {
	r := newDistributionRepo(db, cfg, "mdfe_distributions")
	return &MDFeDistributionRepository{r}
}

// GetByNSU retrieves a single distribution record by its numeric NSU key.
func (r *DistributionRepository) GetByNSU(ctx context.Context, pk string, nsu int) (map[string]any, error) {
	raw, err := r.GetItemByRawKey(ctx, map[string]types.AttributeValue{
		"pk":  &types.AttributeValueMemberS{Value: pk},
		"nsu": &types.AttributeValueMemberN{Value: strconv.Itoa(nsu)},
	})
	if err != nil || raw == nil {
		return nil, err
	}
	var m map[string]any
	if err := attributevalue.UnmarshalMap(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// DistributionListOpts holds pagination options for listing distribution records.
type DistributionListOpts struct {
	Limit    int
	StartKey map[string]types.AttributeValue
}

// ListDistributions returns distribution records newest-first (ScanIndexForward=false).
func (r *DistributionRepository) ListDistributions(ctx context.Context, pk string, opts DistributionListOpts) (*QueryResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.TableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(false),
	}
	if opts.StartKey != nil {
		input.ExclusiveStartKey = opts.StartKey
	}
	out, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	return &QueryResult{Items: out.Items, LastEvaluatedKey: out.LastEvaluatedKey}, nil
}

// NfseDistributionRepository wraps DistributionRepository for nfse_distributions table.
type NfseDistributionRepository struct{ DistributionRepository }

func NewNfseDistributionRepository(db *dynamodb.Client, cfg *config.Config) *NfseDistributionRepository {
	r := newDistributionRepo(db, cfg, "nfse_distributions")
	return &NfseDistributionRepository{r}
}
