package repositories

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Tabelas de NFS-e, criadas na fase F1 do módulo.
const (
	TableNfses      = "nfses"
	TableNfseEvents = "nfse_events"
)

// accessKeyIndexName é a GSI que resolve chave de acesso -> item. Existe
// porque a SK de nfses é o id_dps: a chave de acesso só passa a existir na
// resposta do fisco (spec §3.4).
const accessKeyIndexName = "access-key-index"

// NfseRepository reusa toda a mecânica de DocumentRepository — inclusive
// TransactReserveAndCreate, que a emissão usa para reservar número, criar o
// documento e enfileirar o comando do worker numa transação só.
type NfseRepository struct {
	DocumentRepository
}

func NewNfseRepository(db *dynamodb.Client, cfg *config.Config) *NfseRepository {
	return &NfseRepository{DocumentRepository: DocumentRepository{Base: NewBase(db, cfg, TableNfses), db: db}}
}

// GetByAccessKey resolve a chave de acesso pela GSI. Devolve nil quando não
// existe — o serviço traduz para 404.
func (r *NfseRepository) GetByAccessKey(ctx context.Context, pk, accessKey string) (map[string]types.AttributeValue, error) {
	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.TableName),
		IndexName:              aws.String(accessKeyIndexName),
		KeyConditionExpression: aws.String("pk = :pk AND access_key = :ak"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":ak": &types.AttributeValueMemberS{Value: accessKey},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", accessKeyIndexName, err)
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	return out.Items[0], nil
}

// NfseListOpts filtra a listagem. Status e competência são os filtros que a
// tela de NFS-e oferece (spec §7).
type NfseListOpts struct {
	Limit    int
	StartKey map[string]types.AttributeValue
	Status   *string
	Number   *int
	Year     *int
	Month    *int
	Sort     string
}

func normalizeSort(s string) string {
	if s == "desc" {
		return "desc"
	}
	return "asc"
}

// ListNfses lista por pk, opcionalmente por número (number-index) ou
// competência (date-index). Sem scan.
func (r *NfseRepository) ListNfses(ctx context.Context, pk string, opts NfseListOpts) (*QueryResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	sort := normalizeSort(opts.Sort)

	if opts.Number != nil {
		return r.queryNumberIndex(ctx, pk, *opts.Number, opts.Limit, opts.StartKey, sort)
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.TableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		Limit:             aws.Int32(int32(opts.Limit)),
		ScanIndexForward:  aws.Bool(sort == "asc"),
		ExclusiveStartKey: opts.StartKey,
	}
	if opts.Status != nil {
		input.FilterExpression = aws.String("#st = :st")
		input.ExpressionAttributeNames = map[string]string{"#st": "status"}
		input.ExpressionAttributeValues[":st"] = &types.AttributeValueMemberS{Value: *opts.Status}
	}

	out, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", r.TableName, err)
	}
	return &QueryResult{Items: out.Items, LastEvaluatedKey: out.LastEvaluatedKey}, nil
}
