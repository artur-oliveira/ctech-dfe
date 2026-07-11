package repositories

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/artur-oliveira/ctech-dfe/api/internal/config"
)

// DocumentRepository is the base for NF-e, NFC-e, CT-e, MDF-e repos.
// pk = {env}#{org_pk}  sk = access_key (44-digit chave de acesso)
type DocumentRepository struct {
	Base
}

func (r *DocumentRepository) Create(ctx context.Context, item map[string]types.AttributeValue) error {
	item["created_at"] = &types.AttributeValueMemberS{Value: NowStr()}
	return r.PutItem(ctx, item)
}

func (r *DocumentRepository) Get(ctx context.Context, pk, accessKey string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, pk, accessKey)
}

func (r *DocumentRepository) Update(ctx context.Context, pk, accessKey string, updates map[string]any) (bool, error) {
	return r.UpdateItem(ctx, pk, &accessKey, updates)
}

type DocumentListOpts struct {
	Limit    int
	StartKey map[string]types.AttributeValue
	Sort     string
}

func (r *DocumentRepository) List(ctx context.Context, pk string, opts DocumentListOpts) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{
		PK: pk, ScanIndexForward: opts.Sort != "desc",
		Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

// NfeRepository — nfes table.
type NfeRepository struct {
	DocumentRepository
}

func NewNfeRepository(db *dynamodb.Client, cfg *config.Config) *NfeRepository {
	return &NfeRepository{DocumentRepository: DocumentRepository{Base: NewBase(db, cfg, "nfes")}}
}

// NfceRepository — nfces table. Shares all query/persist logic with NF-e via
// the embedded DocumentRepository (same key schema, same GSIs).
type NfceRepository struct {
	DocumentRepository
}

func NewNfceRepository(db *dynamodb.Client, cfg *config.Config) *NfceRepository {
	return &NfceRepository{DocumentRepository: DocumentRepository{Base: NewBase(db, cfg, "nfces")}}
}

// CteRepository — ctes table. Shares the DocumentRepository key schema; used by
// MDF-e emission to read referenced CT-e records (and their xml_s3_key).
type CteRepository struct {
	DocumentRepository
}

func NewCteRepository(db *dynamodb.Client, cfg *config.Config) *CteRepository {
	return &CteRepository{DocumentRepository: DocumentRepository{Base: NewBase(db, cfg, "ctes")}}
}

// MdfeRepository — mdfes table. Same key schema and GSIs as the other DFe tables.
type MdfeRepository struct {
	DocumentRepository
}

func NewMdfeRepository(db *dynamodb.Client, cfg *config.Config) *MdfeRepository {
	return &MdfeRepository{DocumentRepository: DocumentRepository{Base: NewBase(db, cfg, "mdfes")}}
}

// NFeListOpts mirrors list_nfes parameters in Python NfeRepository.
type NFeListOpts struct {
	Limit    int
	StartKey map[string]types.AttributeValue
	Incoming *int
	Number   *int
	Year     *int
	Month    *int
	Day      *int
	Sort     string
}

func (r *DocumentRepository) ListNFes(ctx context.Context, pk string, opts NFeListOpts) (*QueryResult, error) {
	if opts.Limit == 0 {
		opts.Limit = 50
	}
	if opts.Number != nil {
		return r.queryNumberIndex(ctx, pk, *opts.Number, opts.Limit, opts.StartKey, opts.Sort)
	}
	if opts.Incoming != nil {
		return r.queryDateIndex(ctx, pk, *opts.Incoming, opts.Year, opts.Month, opts.Day, opts.Number, opts.Limit, opts.StartKey, opts.Sort)
	}
	return r.List(ctx, pk, DocumentListOpts{Limit: opts.Limit, StartKey: opts.StartKey, Sort: opts.Sort})
}

func (r *DocumentRepository) queryNumberIndex(ctx context.Context, pk string, number, limit int, startKey map[string]types.AttributeValue, sort string) (*QueryResult, error) {
	input := &dynamodb.QueryInput{
		TableName:                aws.String(r.TableName),
		IndexName:                aws.String("number-index-v2"),
		KeyConditionExpression:   aws.String("pk = :pk AND #num = :number"),
		ExpressionAttributeNames: map[string]string{"#num": "number"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":number": &types.AttributeValueMemberN{Value: strconv.Itoa(number)},
		},
		ScanIndexForward: aws.Bool(sort != "desc"),
		Limit:            aws.Int32(int32(limit)),
	}
	if startKey != nil {
		input.ExclusiveStartKey = startKey
	}
	out, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	return &QueryResult{Items: out.Items, LastEvaluatedKey: out.LastEvaluatedKey}, nil
}

func (r *DocumentRepository) queryDateIndex(ctx context.Context, pk string, incoming int, year, month, day, number *int, limit int, startKey map[string]types.AttributeValue, sort string) (*QueryResult, error) {
	keyExpr := "#pk = :pk AND #incoming = :incoming"
	exprValues := map[string]types.AttributeValue{
		":pk":       &types.AttributeValueMemberS{Value: pk},
		":incoming": &types.AttributeValueMemberN{Value: strconv.Itoa(incoming)},
	}
	exprNames := map[string]string{
		"#pk":       "pk",
		"#incoming": "incoming",
	}

	if year != nil {
		keyExpr += " AND #yr = :yr"
		exprNames["#yr"] = "year"
		exprValues[":yr"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*year)}
	}
	if month != nil {
		keyExpr += " AND #mo = :mo"
		exprNames["#mo"] = "month"
		exprValues[":mo"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*month)}
	}
	if day != nil {
		keyExpr += " AND #dy = :dy"
		exprNames["#dy"] = "day"
		exprValues[":dy"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*day)}
	}
	if number != nil {
		keyExpr += " AND #nb = :nb"
		exprNames["#nb"] = "number"
		exprValues[":nb"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*number)}
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.TableName),
		IndexName:                 aws.String("dfe-index"),
		KeyConditionExpression:    aws.String(keyExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
		ScanIndexForward:          aws.Bool(sort != "desc"),
		Limit:                     aws.Int32(int32(limit)),
	}

	if startKey != nil {
		input.ExclusiveStartKey = startKey
	}

	out, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	return &QueryResult{Items: out.Items, LastEvaluatedKey: out.LastEvaluatedKey}, nil
}

// TransactReserveAndCreate atomically increments the NF-e counter and inserts the NF-e record.
// Mirrors _transact_reserve_and_create in Python NfeService.
// Uses attribute_not_exists fallback so the condition works on first emission.
func (r *DocumentRepository) TransactReserveAndCreate(ctx context.Context, configTable, orgPK, envPrefix string, currentNumber int, nfeItem map[string]types.AttributeValue) error {
	nextNumber := currentNumber + 1
	now := NowStr()
	counterField := fmt.Sprintf("%s_current_number", envPrefix)

	condExpr := fmt.Sprintf("attribute_not_exists(#num) OR #num = :current")
	updateExpr := "SET #num = :next, updated_at = :ts"

	items := []types.TransactWriteItem{
		{
			Update: &types.Update{
				TableName:                aws.String(configTable),
				Key:                      map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
				UpdateExpression:         aws.String(updateExpr),
				ConditionExpression:      aws.String(condExpr),
				ExpressionAttributeNames: map[string]string{"#num": counterField},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":current": &types.AttributeValueMemberN{Value: strconv.Itoa(currentNumber)},
					":next":    &types.AttributeValueMemberN{Value: strconv.Itoa(nextNumber)},
					":ts":      &types.AttributeValueMemberS{Value: now},
				},
			},
		},
		{
			Put: &types.Put{
				TableName:           aws.String(r.TableName),
				Item:                nfeItem,
				ConditionExpression: aws.String("attribute_not_exists(pk) AND attribute_not_exists(sk)"),
			},
		},
	}

	_, err := r.db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
	})
	return wrapDynamoErr(err)
}

// EncodeItem encodes a map[string]any into DynamoDB attribute values, omitting nulls.
func EncodeItem(item map[string]any) (map[string]types.AttributeValue, error) {
	return MarshalMapOmitNull(item)
}
