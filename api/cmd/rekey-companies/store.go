package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/dynamo"
)

// store is every DynamoDB call this tool makes, behind one type so the planner
// and the verifier stay testable without a database.
type store struct {
	db           *dynamodb.Client
	dfePrefix    string
	accountTable string
}

// readMapping loads the legacy-key → company mapping out of ctech-account.
//
// It reads the sparse lookup index that the company migration already writes
// (lookup_pk = SOURCE#dfe#{legacy pk}), so the mapping is recoverable at any
// time by anybody — there is no file to keep, and no ordering between the two
// tools beyond "pass one first".
func (s *store) readMapping(ctx context.Context, legacyPK string) (*mapping, error) {
	base := dynamo.NewBase(s.db, "", s.accountTable)
	res, err := base.QueryGSI(ctx, "lookup-index", "lookup_pk", "SOURCE#"+sourceSystem+"#"+legacyPK, 1, nil)
	if err != nil {
		return nil, fmt.Errorf("reading the company for %s: %w", legacyPK, err)
	}
	if len(res.Items) == 0 {
		return nil, nil
	}
	item := res.Items[0]
	m := &mapping{
		LegacyPK:  legacyPK,
		TaxID:     attrS(item, "tax_id"),
		TaxIDKind: attrS(item, "tax_id_kind"),
		LegalName: attrS(item, "legal_name"),
	}
	// The company's own ids live in its keys: pk = ORG#{organization_id},
	// sk = COMPANY#{company_id}.
	if pk := attrS(item, "pk"); len(pk) > 4 {
		m.OrganizationID = pk[4:]
	}
	if sk := attrS(item, "sk"); len(sk) > 8 {
		m.CompanyID = sk[8:]
	}
	return m, nil
}

// listLegacyOrganizations scans the dfe organizations table.
//
// A Scan, and it is the right call: this table holds one row per company and
// the whole point is to visit every one. A Query would need a key nobody has.
func (s *store) listLegacyOrganizations(ctx context.Context) ([]string, error) {
	base := dynamo.NewBase(s.db, s.dfePrefix, "organizations")
	var out []string
	var start map[string]types.AttributeValue
	for {
		res, err := base.ScanRaw(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(base.TableName),
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning organizations: %w", err)
		}
		for _, item := range res.Items {
			if pk := attrS(item, "pk"); pk != "" {
				out = append(out, pk)
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// readPartition returns every row of one partition, keyed by sort key and
// rendered as a stable string so two rows can be compared.
//
// The organizations table has no sort key, which is why the empty string is a
// valid key here rather than a bug.
func (s *store) readPartition(ctx context.Context, tableName, pk string) (map[string]string, error) {
	base := dynamo.NewBase(s.db, s.dfePrefix, tableName)
	out := map[string]string{}
	var start map[string]types.AttributeValue
	for {
		res, err := base.QueryRaw(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(base.TableName),
			KeyConditionExpression:    aws.String("pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
			ExclusiveStartKey:         start,
		})
		if err != nil {
			return nil, fmt.Errorf("reading %s/%s: %w", tableName, pk, err)
		}
		for _, item := range res.Items {
			body, err := canonicalBody(item)
			if err != nil {
				return nil, err
			}
			out[attrS(item, "sk")] = body
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// copyPartition writes one partition's rows under the new key.
//
// Every write is conditional on absence, so a re-run skips what landed and
// completes what did not: a run that dies mid-table is finished by the next
// one rather than restarted. The old partition is never touched — that is the
// rollback, and deleting it is a separate decision taken later.
func (s *store) copyPartition(ctx context.Context, tableName, fromPK, toPK string, extra map[string]types.AttributeValue) (int, error) {
	base := dynamo.NewBase(s.db, s.dfePrefix, tableName)
	written := 0
	var start map[string]types.AttributeValue
	for {
		res, err := base.QueryRaw(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(base.TableName),
			KeyConditionExpression:    aws.String("pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: fromPK}},
			ExclusiveStartKey:         start,
		})
		if err != nil {
			return written, fmt.Errorf("reading %s/%s: %w", tableName, fromPK, err)
		}
		for _, item := range res.Items {
			copied := make(map[string]types.AttributeValue, len(item)+len(extra))
			for k, v := range item {
				copied[k] = v
			}
			copied["pk"] = &types.AttributeValueMemberS{Value: toPK}
			for k, v := range extra {
				copied[k] = v
			}
			_, err := base.PutItemRaw(ctx, &dynamodb.PutItemInput{
				TableName:           aws.String(base.TableName),
				Item:                copied,
				ConditionExpression: aws.String("attribute_not_exists(pk)"),
			})
			if dynamo.IsConditionFailed(err) {
				// Already copied by an earlier run. Not an error: this is what
				// makes the tool resumable.
				continue
			}
			if err != nil {
				return written, fmt.Errorf("writing %s/%s: %w", tableName, toPK, err)
			}
			written++
		}
		if len(res.LastEvaluatedKey) == 0 {
			return written, nil
		}
		start = res.LastEvaluatedKey
	}
}

// identityAttrs are the fields the company record gains on top of its copy: the
// platform identity, which is what every issuer document is read from after the
// flip.
func identityAttrs(m *mapping) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"organization_id": &types.AttributeValueMemberS{Value: m.OrganizationID},
		"tax_id":          &types.AttributeValueMemberS{Value: m.TaxID},
		"tax_id_kind":     &types.AttributeValueMemberS{Value: m.TaxIDKind},
		"legal_name":      &types.AttributeValueMemberS{Value: m.LegalName},
	}
}

// canonicalBody renders an item for comparison, with pk removed — it is the one
// attribute the copy is supposed to change.
func canonicalBody(item map[string]types.AttributeValue) (string, error) {
	trimmed := make(map[string]types.AttributeValue, len(item))
	for k, v := range item {
		if k == "pk" {
			continue
		}
		trimmed[k] = v
	}
	// json.Marshal sorts map keys, so two items with the same attributes render
	// the same regardless of the order DynamoDB returned them in.
	b, err := json.Marshal(trimmed)
	if err != nil {
		return "", fmt.Errorf("rendering an item for comparison: %w", err)
	}
	return string(b), nil
}

func attrS(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
