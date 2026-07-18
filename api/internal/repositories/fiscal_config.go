package repositories

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

const (
	distNSURateLimitHours = 1
	consQuotaMax          = 20
)

// FiscalConfigRepository is the base for singleton fiscal config repos
// (one record per org, keyed only by pk). Mirrors _fiscal_config.py.
type FiscalConfigRepository struct {
	Base
	// preserve lists fields that must survive an Upsert (updated by internal processes).
	preserve map[string]any
}

func newFiscalConfigBase(db *dynamodb.Client, cfg *config.Config, table string, preserve map[string]any) FiscalConfigRepository {
	return FiscalConfigRepository{
		Base:     NewBase(db, cfg, table),
		preserve: preserve,
	}
}

func (r *FiscalConfigRepository) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK)
}

// Upsert writes the fiscal config, preserving internal-process fields (e.g. NSU cursors).
func (r *FiscalConfigRepository) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	existing, err := r.GetItem(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	_, finalItem, err := r.BuildUpsertTxItem(orgPK, fields, existing)
	if err != nil {
		return nil, err
	}
	return finalItem, r.PutItem(ctx, finalItem)
}

// BuildUpsertTxItem returns a TransactWriteItem for an upsert, mirroring Upsert's
// preserve-merge logic, without writing. `existing` MUST be the caller's own
// pre-fetch of the current item for this orgPK (or nil if none exists) — it is
// NOT re-fetched here. Callers that also need a pre-write snapshot for an audit
// diff (beforeMap) MUST reuse that SAME fetch for both purposes: two independent
// reads of the same item, moments apart, can straddle a concurrent internal-process
// write (e.g. an NSU cursor increment) and misattribute that change to the acting
// user. It also returns the final merged field map exactly as it will be stored
// (i.e. AFTER preserved fields have been carried forward from `existing`) —
// callers that need to audit-diff this write MUST diff against this returned
// map, not the caller-supplied `fields`, otherwise preserved fields that were
// silently carried forward would be misreported as user-initiated changes.
func (r *FiscalConfigRepository) BuildUpsertTxItem(orgPK string, fields map[string]types.AttributeValue, existing map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue, error) {
	fields["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	fields["updated_at"] = &types.AttributeValueMemberS{Value: NowStr()}

	for field, defVal := range r.preserve {
		if existing != nil {
			if v, ok := existing[field]; ok {
				fields[field] = v
				continue
			}
		}
		// set default if not yet present
		if _, alreadySet := fields[field]; !alreadySet {
			if defVal == nil {
				continue // omit null defaults
			}
			av, _ := attributevalue.Marshal(defVal)
			fields[field] = av
		}
	}

	return r.BuildPutTxItem(fields), fields, nil
}

// IncrementNumber atomically increments the emission counter for the environment.
func (r *FiscalConfigRepository) IncrementNumber(ctx context.Context, orgPK, envPrefix string) (int64, error) {
	return r.AtomicIncrement(ctx, orgPK, nil, fmt.Sprintf("%s_current_number", envPrefix))
}

// ClaimDistNSUSlot atomically claims the 1-hour distNSU call slot.
// Returns true if the slot was claimed; false if rate-limited.
func (r *FiscalConfigRepository) ClaimDistNSUSlot(ctx context.Context, orgPK, envPrefix string) (bool, error) {
	field := fmt.Sprintf("%s_last_dist_nsu_at", envPrefix)
	now := time.Now().UTC().Truncate(time.Second)
	// Use RFC3339 (second precision, fixed-width UTC) so DynamoDB string < comparison is lexicographically correct.
	nowStr := now.Format(time.RFC3339)
	thresholdStr := now.Add(-distNSURateLimitHours * time.Hour).Format(time.RFC3339)

	input := &dynamodb.UpdateItemInput{
		Key:                      map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
		UpdateExpression:         aws.String("SET #f = :now"),
		ConditionExpression:      aws.String("attribute_not_exists(#f) OR #f < :threshold"),
		ExpressionAttributeNames: map[string]string{"#f": field},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":       &types.AttributeValueMemberS{Value: nowStr},
			":threshold": &types.AttributeValueMemberS{Value: thresholdStr},
		},
	}
	_, err := r.UpdateItemRaw(ctx, input)
	if err != nil {
		if IsConditionFailed(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdateNSU persists the latest fetched NSU cursor.
func (r *FiscalConfigRepository) UpdateNSU(ctx context.Context, orgPK, envPrefix string, nsu int64) error {
	field := fmt.Sprintf("%s_nsu", envPrefix)
	input := &dynamodb.UpdateItemInput{
		Key:                      map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
		UpdateExpression:         aws.String("SET #f = :nsu"),
		ExpressionAttributeNames: map[string]string{"#f": field},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":nsu": &types.AttributeValueMemberN{Value: strconv.FormatInt(nsu, 10)},
		},
	}
	_, err := r.UpdateItemRaw(ctx, input)
	return err
}

// IncrementConsQuota atomically increments the per-hour quota counter.
// Returns new call count; returns consQuotaMax+1 on error to block the call.
func (r *FiscalConfigRepository) IncrementConsQuota(ctx context.Context, orgPK, envPrefix string) int {
	windowField := fmt.Sprintf("%s_cons_quota_window_start", envPrefix)
	callsField := fmt.Sprintf("%s_cons_quota_calls", envPrefix)
	now := NowStr()

	input := &dynamodb.UpdateItemInput{
		Key: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
		UpdateExpression: aws.String(fmt.Sprintf(
			"SET #w = if_not_exists(#w, :now), #c = if_not_exists(#c, :zero) + :one",
		)),
		ExpressionAttributeNames: map[string]string{"#w": windowField, "#c": callsField},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":  &types.AttributeValueMemberS{Value: now},
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":one":  &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueAllNew,
	}
	out, err := r.UpdateItemRaw(ctx, input)
	if err != nil {
		return consQuotaMax + 1
	}
	if av, ok := out.Attributes[callsField]; ok {
		if nv, ok := av.(*types.AttributeValueMemberN); ok {
			n, _ := strconv.Atoi(nv.Value)
			return n
		}
	}
	return consQuotaMax + 1
}

// ResetConsQuota resets the hourly quota window.
func (r *FiscalConfigRepository) ResetConsQuota(ctx context.Context, orgPK, envPrefix string) error {
	windowField := fmt.Sprintf("%s_cons_quota_window_start", envPrefix)
	callsField := fmt.Sprintf("%s_cons_quota_calls", envPrefix)
	input := &dynamodb.UpdateItemInput{
		Key:                      map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
		UpdateExpression:         aws.String("SET #c = :zero, #w = :now"),
		ExpressionAttributeNames: map[string]string{"#w": windowField, "#c": callsField},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":now":  &types.AttributeValueMemberS{Value: NowStr()},
		},
	}
	_, err := r.UpdateItemRaw(ctx, input)
	return err
}
