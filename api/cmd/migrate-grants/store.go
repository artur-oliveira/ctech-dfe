package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/dynamo"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// store is every DynamoDB call this tool makes, so the planner stays testable
// without a database.
type store struct {
	db           *dynamodb.Client
	dfePrefix    string
	accountTable string
}

// listOverlays reads ctech-dfe's authorization rows.
//
// Only company-keyed ones. A legacy CNPJ_ partition has not been through the
// re-key, so there is no company id for an edge to name — and granting reach for
// a key that is about to be retired would write a row nobody reads.
func (s *store) listOverlays(ctx context.Context) ([]overlay, error) {
	base := dynamo.NewBase(s.db, s.dfePrefix, "organization_users")
	var out []overlay
	var start map[string]types.AttributeValue
	for {
		res, err := base.ScanRaw(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(base.TableName),
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning organization_users: %w", err)
		}
		for _, item := range res.Items {
			pk := attrS(item, "pk")
			if !repositories.IsCompanyKey(pk) {
				continue
			}
			out = append(out, overlay{
				CompanyID:   pk,
				UserID:      attrS(item, "sk"),
				Role:        attrS(item, "role"),
				Permissions: attrSS(item, "permissions"),
			})
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// companyOrganizations maps each company to its organization, read from
// ctech-account.
//
// The edge is keyed by (organization, company, user) and this tool is given
// neither — so it reads the companies table rather than guessing, and a company
// it cannot place is a refusal rather than a guessed key.
func (s *store) companyOrganizations(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	var start map[string]types.AttributeValue
	for {
		res, err := s.db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(s.accountTable),
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", s.accountTable, err)
		}
		for _, item := range res.Items {
			pk, sk := attrS(item, "pk"), attrS(item, "sk")
			companyID, isCompany := strings.CutPrefix(sk, "COMPANY#")
			orgID, isOrg := strings.CutPrefix(pk, "ORG#")
			if isCompany && isOrg {
				out[companyID] = orgID
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// reaches reports which (company, user) pairs ctech-account already records.
//
// One GetItem per row rather than a scan of the edges: a person acts for a
// handful of companies, the overlay list is the bound, and a scan would read
// every edge of every organization to answer about a few.
func (s *store) reaches(ctx context.Context, overlays []overlay) (map[string]reach, error) {
	out := make(map[string]reach, len(overlays))
	orgs, err := s.companyOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	for _, o := range overlays {
		orgID := orgs[o.CompanyID]
		if orgID == "" {
			continue
		}
		res, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(s.accountTable),
			Key: map[string]types.AttributeValue{
				"pk": &types.AttributeValueMemberS{Value: "ORG#" + orgID},
				"sk": &types.AttributeValueMemberS{Value: actorSK(o.CompanyID, o.UserID)},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("reading the edge for %s/%s: %w", o.CompanyID, o.UserID, err)
		}
		if res.Item != nil {
			out[edgeKey(o.CompanyID, o.UserID)] = reach{HasEdge: true, OrganizationID: orgID}
		}
	}
	return out, nil
}

// grantEdge writes one edge in ctech-account.
//
// Conditional on absence: a run that races a grant made through the product must
// not overwrite a granted_by somebody will later be asked about. Already-there
// is success, which is what makes the tool resumable.
func (s *store) grantEdge(ctx context.Context, orgID, companyID, userID string) error {
	_, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.accountTable),
		Item: map[string]types.AttributeValue{
			"pk":         &types.AttributeValueMemberS{Value: "ORG#" + orgID},
			"sk":         &types.AttributeValueMemberS{Value: actorSK(companyID, userID)},
			"lookup_pk":  &types.AttributeValueMemberS{Value: "USER#" + bareUserID(userID)},
			"granted_by": &types.AttributeValueMemberS{Value: "migrate-grants"},
			"created_at": &types.AttributeValueMemberS{Value: dynamo.NowStr()},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if dynamo.IsConditionFailed(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("granting the edge for %s/%s: %w", companyID, userID, err)
	}
	return nil
}

// actorSK mirrors ctech-account's own key layout. Duplicated here rather than
// imported: this tool must not depend on that repository's Go module, and the
// layout is pinned by a test so the copy cannot drift silently.
func actorSK(companyID, userID string) string {
	return "ACTOR#" + companyID + "#" + bareUserID(userID)
}

// bareUserID strips ctech-dfe's USER_ sort-key prefix. ctech-account keys edges
// by the bare subject, and an edge written with the prefix would be a row the
// platform cannot find.
func bareUserID(userID string) string {
	if after, ok := strings.CutPrefix(userID, "USER_"); ok {
		return after
	}
	return userID
}

func attrS(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func attrSS(item map[string]types.AttributeValue, key string) []string {
	v, ok := item[key].(*types.AttributeValueMemberL)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(v.Value))
	for _, e := range v.Value {
		if s, ok := e.(*types.AttributeValueMemberS); ok {
			out = append(out, s.Value)
		}
	}
	return out
}
