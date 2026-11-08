package repositories

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Table names, keys and index names of the CIOT tables (docs/specs/2026-11-04-ciot-antt-pef-integracao.md).
const (
	TableCiots      = "ciots"
	TableCiotAudit  = "ciot_audit"
	CiotSKPrefix    = "CIOT_"
	CiotCodeIndex   = "code-index"
	CiotMdfeIndex   = "mdfe-index"
	ciotCachePKFmt  = "CARRIER_STATUS#%s#%s"
	ciotCacheSK     = "STATUS"
	AttrCiotCode    = "code"
	AttrCiotMdfeKey = "mdfe_access_key"
)

// CiotRepository stores CIOT operations (pk {env}#{org_pk}, sk CIOT_{id}) and the carrier-status cache
// (pk CARRIER_STATUS#{env}#{doc}, TTL expires_at). The cache rows are public RNTRC data and never org rows.
type CiotRepository struct{ Base }

func NewCiotRepository(db *dynamodb.Client, cfg *config.Config) *CiotRepository {
	return &CiotRepository{Base: NewBase(db, cfg, TableCiots)}
}

// CiotSK builds the sort key from the request id.
func CiotSK(id string) string { return CiotSKPrefix + id }

func (r *CiotRepository) Get(ctx context.Context, pk, id string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, pk, CiotSK(id))
}

// List pages the organization's CIOTs newest first (ULID ids sort by creation time).
func (r *CiotRepository) List(ctx context.Context, pk string, limit int, start map[string]types.AttributeValue) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{PK: pk, SKPrefix: CiotSKPrefix, ScanIndexForward: false, Limit: limit, ExclusiveStartKey: start})
}

// ByMdfe lists the CIOTs linked to an MDF-e access key.
func (r *CiotRepository) ByMdfe(ctx context.Context, pk, accessKey string) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{PK: pk, SKPrefix: accessKey, IndexName: CiotMdfeIndex, SKField: AttrCiotMdfeKey, ScanIndexForward: true})
}

// ByCode finds a CIOT by its 12-character code.
func (r *CiotRepository) ByCode(ctx context.Context, pk, code string) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{PK: pk, SKPrefix: code, IndexName: CiotCodeIndex, SKField: AttrCiotCode, ScanIndexForward: true})
}

// CreateTx is the conditional insert of a new CIOT row, for a transaction with its outbox and audit items.
func (r *CiotRepository) CreateTx(item map[string]types.AttributeValue) types.TransactWriteItem {
	return r.BuildPutTxItemIfAbsent(item)
}

// QueueOperationTx marks the row as having an operation in flight, only when none is and the status allows it.
func (r *CiotRepository) QueueOperationTx(pk, id, op string, allowed []string) types.TransactWriteItem {
	names := map[string]string{"#st": "status", "#po": "pending_operation", "#ua": "updated_at"}
	values := map[string]types.AttributeValue{
		":op": &types.AttributeValueMemberS{Value: op},
		":ua": &types.AttributeValueMemberS{Value: NowStr()},
	}
	cond := "attribute_exists(pk) AND attribute_not_exists(#po) AND #st IN ("
	for i, s := range allowed {
		k := fmt.Sprintf(":s%d", i)
		values[k] = &types.AttributeValueMemberS{Value: s}
		if i > 0 {
			cond += ", "
		}
		cond += k
	}
	sk := CiotSK(id)
	return r.BuildRawUpdateTxItem(pk, &sk, "SET #po = :op, #ua = :ua", cond+")", names, values)
}

// LinkMdfeTx links an open CIOT with no operation in flight to an MDF-e access key, in the MDF-e emission
// transaction. Re-linking the same key is a no-op; a CIOT linked to another MDF-e fails the condition.
func (r *CiotRepository) LinkMdfeTx(pk, id, accessKey string, allowed []string) types.TransactWriteItem {
	names := map[string]string{"#st": "status", "#po": "pending_operation", "#mk": AttrCiotMdfeKey}
	values := map[string]types.AttributeValue{
		":k":  &types.AttributeValueMemberS{Value: accessKey},
		":ua": &types.AttributeValueMemberS{Value: NowStr()},
	}
	cond := "attribute_exists(pk) AND attribute_not_exists(#po) AND (attribute_not_exists(#mk) OR #mk = :k) AND #st IN ("
	for i, s := range allowed {
		k := fmt.Sprintf(":s%d", i)
		values[k] = &types.AttributeValueMemberS{Value: s}
		if i > 0 {
			cond += ", "
		}
		cond += k
	}
	sk := CiotSK(id)
	return r.BuildRawUpdateTxItem(pk, &sk, "SET #mk = :k, updated_at = :ua", cond+")", names, values)
}

// GetCarrierStatus returns the cached ConsultarSituacaoTransportador body, or "" when absent or expired
// (DynamoDB TTL deletion is lazy, so expiry is checked here).
func (r *CiotRepository) GetCarrierStatus(ctx context.Context, env, doc string, now time.Time) (string, error) {
	item, err := r.GetItem(ctx, fmt.Sprintf(ciotCachePKFmt, env, doc), ciotCacheSK)
	if err != nil || item == nil {
		return "", err
	}
	exp, _ := item["expires_at"].(*types.AttributeValueMemberN)
	if exp == nil {
		return "", nil
	}
	if at, err := strconv.ParseInt(exp.Value, 10, 64); err != nil || at <= now.Unix() {
		return "", nil
	}
	if b, ok := item["body"].(*types.AttributeValueMemberS); ok {
		return b.Value, nil
	}
	return "", nil
}

// PutCarrierStatus caches a response body for ttl.
func (r *CiotRepository) PutCarrierStatus(ctx context.Context, env, doc, body string, now time.Time, ttl time.Duration) error {
	return r.PutItem(ctx, map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: fmt.Sprintf(ciotCachePKFmt, env, doc)},
		"sk":         &types.AttributeValueMemberS{Value: ciotCacheSK},
		"body":       &types.AttributeValueMemberS{Value: body},
		"expires_at": &types.AttributeValueMemberN{Value: fmt.Sprint(now.Add(ttl).Unix())},
	})
}

// CiotAuditRepository is the append-only evidence table: pk = the CIOT row's sk, sk = {RFC3339 nano}#{operation}.
type CiotAuditRepository struct{ Base }

func NewCiotAuditRepository(db *dynamodb.Client, cfg *config.Config) *CiotAuditRepository {
	return &CiotAuditRepository{Base: NewBase(db, cfg, TableCiotAudit)}
}

// AppendTx builds the insert-only audit item. bodyHash is the sha256 of the body; the body itself holds
// documents and is never logged.
func (r *CiotAuditRepository) AppendTx(ciotSK, operation, bodyHash, body string, at time.Time) types.TransactWriteItem {
	return r.BuildPutTxItemIfAbsent(map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: ciotSK},
		"sk":         &types.AttributeValueMemberS{Value: at.UTC().Format(time.RFC3339Nano) + "#" + operation},
		"operation":  &types.AttributeValueMemberS{Value: operation},
		"body_hash":  &types.AttributeValueMemberS{Value: bodyHash},
		"body":       &types.AttributeValueMemberS{Value: body},
		"created_at": &types.AttributeValueMemberS{Value: NowStr()},
	})
}

// List returns the evidence of one CIOT, oldest first.
func (r *CiotAuditRepository) List(ctx context.Context, ciotSK string) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{PK: ciotSK, ScanIndexForward: true})
}
