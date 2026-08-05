package repositories

import (
	"context"
	"errors"
	"fmt"
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

// distributionCursorSK é o item de cursor por organização. O ADN pagina por
// NSU sequencial, não por ultNSU+maxNSU como o DistDFe da NF-e (spec §3.6) —
// por isso o cursor é um único inteiro monotônico.
const distributionCursorSK = "CURSOR"

// NfseDistributionRepository guarda os documentos recebidos do ADN.
type NfseDistributionRepository struct{ DistributionRepository }

func NewNfseDistributionRepository(db *dynamodb.Client, cfg *config.Config) *NfseDistributionRepository {
	return &NfseDistributionRepository{newDistributionRepo(db, cfg, TableNfses+"_distributions")}
}

func isConditionalCheckFailed(err error) bool {
	var ccf *types.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

// GetLastNSU devolve o último NSU consumido por pk. Zero quando nunca rodou.
func (r *DistributionRepository) GetLastNSU(ctx context.Context, pk string) (int64, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: distributionCursorSK},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("get cursor: %w", err)
	}
	n, ok := out.Item["last_nsu"].(*types.AttributeValueMemberN)
	if !ok {
		return 0, nil
	}
	var v int64
	if _, err := fmt.Sscanf(n.Value, "%d", &v); err != nil {
		return 0, fmt.Errorf("parse last_nsu %q: %w", n.Value, err)
	}
	return v, nil
}

// SetLastNSU avança o cursor. A condição impede regressão: uma entrega
// duplicada de SQS não pode fazer o cursor voltar e reprocessar o lote.
func (r *DistributionRepository) SetLastNSU(ctx context.Context, pk string, nsu int64) error {
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: distributionCursorSK},
		},
		UpdateExpression:    aws.String("SET last_nsu = :n"),
		ConditionExpression: aws.String("attribute_not_exists(last_nsu) OR last_nsu < :n"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":n": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", nsu)},
		},
	})
	if err != nil && !isConditionalCheckFailed(err) {
		return fmt.Errorf("set cursor: %w", err)
	}
	return nil
}
