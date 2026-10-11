//go:build integration

package integration_test

// DynamoDB applies a Query's Limit to the items it *evaluates*, before the
// FilterExpression. Every listing below filters a partition, so a match that is
// not among the first Limit rows read used to come back as an empty or short
// page. These tests put the match past the first page.

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

func strAV(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

// batchPut writes items 25 at a time, retrying unprocessed ones.
func batchPut(t *testing.T, table string, items []map[string]types.AttributeValue) {
	t.Helper()
	ctx := context.Background()
	for start := 0; start < len(items); start += 25 {
		var reqs []types.WriteRequest
		for _, it := range items[start:min(start+25, len(items))] {
			reqs = append(reqs, types.WriteRequest{PutRequest: &types.PutRequest{Item: it}})
		}
		pending := map[string][]types.WriteRequest{tablePrefix + "_" + table: reqs}
		for len(pending) > 0 {
			out, err := db.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{RequestItems: pending})
			if err != nil {
				t.Fatalf("batch write %s: %v", table, err)
			}
			pending = out.UnprocessedItems
		}
	}
}

func skOf(t *testing.T, items []map[string]types.AttributeValue, attr string) []string {
	t.Helper()
	out := make([]string, 0, len(items))
	for _, it := range items {
		v, ok := it[attr].(*types.AttributeValueMemberS)
		if !ok {
			t.Fatalf("item without %s: %v", attr, it)
		}
		out = append(out, v.Value)
	}
	return out
}

// An NFS-e list filtered by competence: the only matching notes sort after 30
// notes of another year, and the page size is 5.
func TestListNfsesFilterFindsNotesPastTheFirstPage(t *testing.T) {
	ctx := context.Background()
	pk := "CNPJ_" + randomCNPJ() + "#2"
	var items []map[string]types.AttributeValue
	var want []string
	for i := range 33 {
		year := "2025"
		if i >= 30 {
			year = "2026"
		}
		sk := fmt.Sprintf("DPS%042d", i)
		if year == "2026" {
			want = append(want, sk)
		}
		items = append(items, map[string]types.AttributeValue{
			"pk": strAV(pk), "sk": strAV(sk), "status": strAV("AUTHORIZED"),
			"year": &types.AttributeValueMemberN{Value: year}, "month": &types.AttributeValueMemberN{Value: "3"},
		})
	}
	batchPut(t, repositories.TableNfses, items)

	year := 2026
	res, err := nfseRepo.ListNfses(ctx, pk, repositories.NfseListOpts{Year: &year, Limit: 5})
	if err != nil {
		t.Fatalf("ListNfses: %v", err)
	}
	if got := skOf(t, res.Items, "sk"); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if res.LastEvaluatedKey != nil {
		t.Fatalf("partition exhausted, cursor must be nil")
	}

	// Paging two at a time visits every match once. A page can come back short
	// (here: empty, with a cursor) when one request's read budget — 10 calls of
	// Limit rows — runs out before the matches; the cursor carries on from there.
	var got []string
	opts := repositories.NfseListOpts{Year: &year, Limit: 2}
	for range 100 {
		page, err := nfseRepo.ListNfses(ctx, pk, opts)
		if err != nil {
			t.Fatalf("ListNfses: %v", err)
		}
		if len(page.Items) > 2 {
			t.Fatalf("page of %d exceeds the limit", len(page.Items))
		}
		got = append(got, skOf(t, page.Items, "sk")...)
		if page.LastEvaluatedKey == nil {
			break
		}
		opts.StartKey = page.LastEvaluatedKey
	}
	if !slices.Equal(got, want) {
		t.Fatalf("paged got %v, want %v", got, want)
	}
}

// The organization's only pending invitation is older than more accepted ones
// than one filtered Query reads in its page budget (Limit 200 x 10 pages).
func TestListPendingInvitationsFindsOneBehindManyAccepted(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()
	items := []map[string]types.AttributeValue{{
		"pk": strAV(repositories.InvitationPK("pending-" + orgPK)), "org_pk": strAV(orgPK),
		"created_at": strAV("2020-01-01T00:00:00Z"), "status": strAV(repositories.InvitationPending),
	}}
	for i := range 2100 {
		items = append(items, map[string]types.AttributeValue{
			"pk": strAV(repositories.InvitationPK(fmt.Sprintf("acc-%s-%d", orgPK, i))), "org_pk": strAV(orgPK),
			"created_at": strAV(fmt.Sprintf("2021-01-01T00:00:%04dZ", i)), "status": strAV(repositories.InvitationAccepted),
		})
	}
	batchPut(t, "organization_invitations", items)

	got, err := invRepo.ListPendingByOrg(ctx, orgPK)
	if err != nil {
		t.Fatalf("ListPendingByOrg: %v", err)
	}
	if want := []string{repositories.InvitationPK("pending-" + orgPK)}; !slices.Equal(skOf(t, got, "pk"), want) {
		t.Fatalf("got %v, want %v", skOf(t, got, "pk"), want)
	}
}

// One user's activity in this org is older than their activity in another
// org; the per-user feed filters back to this org.
func TestAuditByUserFindsEntriesPastTheFirstPage(t *testing.T) {
	ctx := context.Background()
	orgPK, otherPK := "CNPJ_"+randomCNPJ(), "CNPJ_"+randomCNPJ()
	user := "user-" + orgPK
	var items []map[string]types.AttributeValue
	var want []string
	for i := range 3 {
		sk := fmt.Sprintf("PRODUCT#P%d#2020-01-01T00:00:0%dZ", i, i)
		want = append([]string{sk}, want...) // newest first
		items = append(items, map[string]types.AttributeValue{
			"pk": strAV(orgPK), "sk": strAV(sk), "user_id": strAV(user), "created_at": strAV(fmt.Sprintf("2020-01-01T00:00:0%dZ", i)),
		})
	}
	for i := range 60 {
		items = append(items, map[string]types.AttributeValue{
			"pk": strAV(otherPK), "sk": strAV(fmt.Sprintf("PRODUCT#X%d", i)), "user_id": strAV(user),
			"created_at": strAV(fmt.Sprintf("2021-01-01T00:00:%02dZ", i)),
		})
	}
	batchPut(t, "audit_logs", items)

	res, err := services.NewAuditLogService(auditRepo).List(ctx, orgPK, services.AuditLogQueryOpts{UserID: user, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := skOf(t, res.Items, "sk"); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
