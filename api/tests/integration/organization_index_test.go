//go:build integration

package integration_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// newCompanyPK returns a lowercase canonical UUID, the shape IsCompanyKey
// accepts. Random so tests never share a partition.
func newCompanyPK(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// seedCompany writes a re-keyed company record under organizationID, the way
// Link and the re-key leave it.
func seedCompany(t *testing.T, organizationID, ownerSub, taxID, legalName string) string {
	t.Helper()
	pk := newCompanyPK(t)
	item := map[string]types.AttributeValue{
		repositories.AttrOrganizationID: &types.AttributeValueMemberS{Value: organizationID},
		repositories.AttrOwnerUserID:    &types.AttributeValueMemberS{Value: ownerSub},
		repositories.AttrTaxID:          &types.AttributeValueMemberS{Value: taxID},
		repositories.AttrTaxIDKind:      &types.AttributeValueMemberS{Value: repositories.TaxKindCNPJ},
		repositories.AttrLegalName:      &types.AttributeValueMemberS{Value: legalName},
	}
	if err := orgRepo.CreateOrganization(context.Background(), pk, item); err != nil {
		t.Fatal(err)
	}
	return pk
}

func TestListCompaniesOfOrganizationIsOldestFirstAndCompanyKeyedOnly(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := "org-idx-a-"+newCompanyPK(t), "org-idx-b-"+newCompanyPK(t)

	first := seedCompany(t, orgA, "owner-idx", "11222333000181", "Primeira Ltda")
	second := seedCompany(t, orgA, "owner-idx", "11222333000262", "Segunda Ltda")
	_ = seedCompany(t, orgB, "owner-idx", "44555666000105", "Outra Ltda")

	// A legacy partition that somehow carries an organization_id must not be
	// listed: it is a rollback copy, not a company.
	legacy := "CNPJ_" + randomCNPJ()
	if err := orgRepo.CreateOrganization(ctx, legacy, map[string]types.AttributeValue{
		repositories.AttrOrganizationID: &types.AttributeValueMemberS{Value: orgA},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := orgRepo.ListCompaniesOfOrganization(ctx, orgA)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PK != first || got[1].PK != second {
		t.Fatalf("got %+v, want [%s %s] in creation order", got, first, second)
	}
	if empty, err := orgRepo.ListCompaniesOfOrganization(ctx, ""); err != nil || len(empty) != 0 {
		t.Fatalf("an empty organization id must list nothing, got %+v (%v)", empty, err)
	}
}

func TestIndexGapsAreListedAndBackfilledWithoutOverwriting(t *testing.T) {
	ctx := context.Background()
	pk := newCompanyPK(t)
	// A re-keyed row the GSI cannot see: no organization_id.
	if err := orgRepo.CreateOrganization(ctx, pk, map[string]types.AttributeValue{
		repositories.AttrOwnerUserID: &types.AttributeValueMemberS{Value: "gap-owner"},
	}); err != nil {
		t.Fatal(err)
	}

	gaps, err := orgRepo.ListCompanyIndexGaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found *repositories.CompanyIndexGap
	for i := range gaps {
		if gaps[i].PK == pk {
			found = &gaps[i]
		}
	}
	if found == nil || !found.MissingOrganization || found.MissingCreatedAt || found.OwnerUserID != "gap-owner" {
		t.Fatalf("gap = %+v", found)
	}

	if err := orgRepo.BackfillIndexKeys(ctx, pk, "org-gap", "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := orgRepo.ListCompaniesOfOrganization(ctx, "org-gap")
	if err != nil || len(got) != 1 || got[0].PK != pk {
		t.Fatalf("after backfill: %+v (%v)", got, err)
	}
	// created_at existed and must be kept, not replaced by the fallback.
	if got[0].CreatedAt == "2000-01-01T00:00:00Z" {
		t.Fatal("backfill overwrote an existing created_at")
	}
}
