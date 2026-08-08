package repositories

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// batchGetLimit is DynamoDB's hard cap on keys per BatchGetItem request.
const batchGetLimit = 100

// Reusable registry entities — the decisions that used to be retyped on every
// issuance, stored once and referenced by id. They all share one shape:
// pk = {org_pk}, sk = {PREFIX}_{uuid}, plus a `name-index` GSI for prefix
// search, so they share one repository instead of four near-identical copies.
const (
	TableTaxProfiles  = "organization_tax_profiles"
	TableOperations   = "organization_operations"
	TablePaymentTerms = "organization_payment_terms"
	TableVehicleSets  = "organization_vehicle_sets"

	SKPrefixTaxProfile  = "TAXPROFILE_"
	SKPrefixOperation   = "OPERATION_"
	SKPrefixPaymentTerm = "PAYMENTTERM_"
	SKPrefixVehicleSet  = "VEHICLESET_"

	// OrgEntityNameIndex is the GSI created for every registry table (see
	// getOrgEntityTable in cdk/lib/dynamodb-stack.ts).
	OrgEntityNameIndex = "name-index"
	OrgEntityNameField = "name"
)

// OrgEntityListOpts configures a registry listing.
type OrgEntityListOpts struct {
	NamePrefix string
	Sort       string
	Limit      int
	StartKey   map[string]types.AttributeValue
}

// OrgEntityRepository is the shared persistence for every reusable registry
// entity. Concrete repositories embed it so fx can inject them by distinct type
// while the CRUD body stays defined exactly once.
type OrgEntityRepository struct {
	CRUDRepository[map[string]types.AttributeValue]
	skPrefix string
}

func newOrgEntityRepository(db *dynamodb.Client, cfg *config.Config, table, skPrefix string) OrgEntityRepository {
	return OrgEntityRepository{
		CRUDRepository: NewCRUDRepository[map[string]types.AttributeValue](db, cfg, table),
		skPrefix:       skPrefix,
	}
}

// SK accepts either a bare id or an already-prefixed sk, so routes may take
// either from the path without the caller having to know which.
func (r *OrgEntityRepository) SK(id string) string {
	if strings.HasPrefix(id, r.skPrefix) {
		return id
	}
	return r.skPrefix + id
}

func (r *OrgEntityRepository) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Create(ctx, orgPK, r.SK(GenerateID()), fields)
}

func (r *OrgEntityRepository) Get(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, r.SK(id))
}

func (r *OrgEntityRepository) List(ctx context.Context, orgPK string, opts OrgEntityListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.NamePrefix != "" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.NamePrefix,
			IndexName: OrgEntityNameIndex, SKField: OrgEntityNameField,
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: r.skPrefix,
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

// BatchGet fetches many registry rows in one round trip, keyed by the id the
// caller asked for. Missing ids are simply absent from the result — the caller
// decides whether that is an error. Used by issuance, which must never do one
// GetItem per line item inside a loop.
func (r *OrgEntityRepository) BatchGet(ctx context.Context, orgPK string, ids []string) (map[string]map[string]types.AttributeValue, error) {
	out := make(map[string]map[string]types.AttributeValue, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	// De-duplicate: several products commonly reference the same profile.
	skToID := make(map[string]string, len(ids))
	keys := make([]map[string]types.AttributeValue, 0, len(ids))
	for _, id := range ids {
		sk := r.SK(id)
		if _, seen := skToID[sk]; seen {
			continue
		}
		skToID[sk] = id
		keys = append(keys, map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
			"sk": &types.AttributeValueMemberS{Value: sk},
		})
	}

	for start := 0; start < len(keys); start += batchGetLimit {
		end := min(start+batchGetLimit, len(keys))
		unprocessed := map[string]types.KeysAndAttributes{
			r.TableName: {Keys: keys[start:end]},
		}
		// BatchGetItem may return UnprocessedKeys under throttling; retrying
		// them is the documented contract, not an optional optimization.
		for len(unprocessed) > 0 {
			res, err := r.BatchGetItemRaw(ctx, &dynamodb.BatchGetItemInput{RequestItems: unprocessed})
			if err != nil {
				return nil, err
			}
			for _, item := range res.Responses[r.TableName] {
				sk, _ := item["sk"].(*types.AttributeValueMemberS)
				if sk == nil {
					continue
				}
				if id, ok := skToID[sk.Value]; ok {
					out[id] = item
				}
			}
			unprocessed = res.UnprocessedKeys
		}
	}
	return out, nil
}

func (r *OrgEntityRepository) Update(ctx context.Context, orgPK, id string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, r.SK(id), updates)
}

func (r *OrgEntityRepository) Delete(ctx context.Context, orgPK, id string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, r.SK(id))
}

// BuildCreateTxItem mirrors Create's key/timestamp construction without writing.
func (r *OrgEntityRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	// marshalEntity never errors for T = map[string]types.AttributeValue (base.go).
	tx, item, _ := r.CRUDRepository.BuildCreateTxItem(orgPK, r.SK(GenerateID()), fields)
	return tx, item
}

func (r *OrgEntityRepository) BuildUpdateTxItem(orgPK, id string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, r.SK(id), updates)
}

func (r *OrgEntityRepository) BuildDeleteTxItem(orgPK, id string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, r.SK(id))
}

// ── Concrete registries ──────────────────────────────────────────────────────

// TaxProfileRepository — organization_tax_profiles. A profile is one tax
// treatment applied to a set of CFOPs, shared by many products.
type TaxProfileRepository struct{ OrgEntityRepository }

func NewTaxProfileRepository(db *dynamodb.Client, cfg *config.Config) *TaxProfileRepository {
	return &TaxProfileRepository{newOrgEntityRepository(db, cfg, TableTaxProfiles, SKPrefixTaxProfile)}
}
