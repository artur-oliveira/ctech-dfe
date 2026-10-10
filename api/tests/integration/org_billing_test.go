//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// seedOrgSnapshot files an organization's own snapshot.
func seedOrgSnapshot(t *testing.T, organizationID string, snap *repositories.AccountSnapshot) {
	t.Helper()
	snap.OrganizationID = organizationID
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(context.Background(), snap); err != nil {
		t.Fatal(err)
	}
}

func proSnapshot(subID string) *repositories.AccountSnapshot {
	return &repositories.AccountSnapshot{
		SubscriptionID: subID, Status: services.StatusActive, Plan: "pro", Entitled: true,
		PeriodStart: "2026-10-01", PeriodEnd: "2026-11-01",
		Quotas: map[string]int64{services.MeterNFe: 3, services.MeterCompanies: 10},
	}
}

// Spec § 5 test 1: two companies of one organization share one snapshot and
// one counter.
func TestTwoCompaniesOfOneOrganizationShareSnapshotAndCounter(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))
	org := "org-share-" + newCompanyPK(t)
	a := seedCompany(t, org, "owner-share", "11222333000181", "A Ltda")
	b := seedCompany(t, org, "owner-share", "11222333000262", "B Ltda")
	seedOrgSnapshot(t, org, proSnapshot("sub_share"))

	snapA, err := svc.SnapshotForOrg(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	snapB, err := svc.SnapshotForOrg(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if snapA.SubscriptionID != "sub_share" || snapB.SubscriptionID != "sub_share" {
		t.Fatalf("snapshots = %+v / %+v", snapA, snapB)
	}

	for _, company := range []string{a, b, a} {
		if err := svc.Reserve(ctx, company, services.MeterNFe); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Reserve(ctx, b, services.MeterNFe); err == nil {
		t.Fatal("the fourth NF-e of the organization must be refused, whichever company asks")
	}
	usage, err := svc.Usage(ctx, org, a)
	if err != nil || usage[services.MeterNFe].Used != 3 {
		t.Fatalf("usage = %+v (%v)", usage, err)
	}
}

// Spec § 5 test 2: two organizations of the same owner have separate quotas and
// subscriptions.
func TestTwoOrganizationsOfOneOwnerAreSeparate(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))
	org1, org2 := "org-sep1-"+newCompanyPK(t), "org-sep2-"+newCompanyPK(t)
	c1 := seedCompany(t, org1, "owner-sep", "11222333000181", "Um Ltda")
	c2 := seedCompany(t, org2, "owner-sep", "44555666000105", "Dois Ltda")
	seedOrgSnapshot(t, org1, proSnapshot("sub_one"))
	seedOrgSnapshot(t, org2, proSnapshot("sub_two"))

	for range 3 {
		if err := svc.Reserve(ctx, c1, services.MeterNFe); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Reserve(ctx, c2, services.MeterNFe); err != nil {
		t.Fatalf("the second organization has its own quota: %v", err)
	}
	s2, _ := svc.SnapshotForOrg(ctx, c2)
	if s2.SubscriptionID != "sub_two" {
		t.Fatalf("snapshot = %+v", s2)
	}
}

// Spec § 5 test 6: dual read. Only the owner's USER_ snapshot exists → it is
// served; once the organization has its own, the ORG_ one wins.
func TestDualReadFallsBackToTheOwnerUntilTheOrganizationHasItsOwn(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls)).WithUserFallback()
	org := "org-dual-" + newCompanyPK(t)
	company := seedCompany(t, org, "owner-dual", "11222333000181", "Dual Ltda")
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(ctx, &repositories.AccountSnapshot{
		UserID: "owner-dual", SubscriptionID: "sub_user", Status: services.StatusActive, Plan: "unlimited",
		Entitled: true, PeriodStart: "2026-10-01", Quotas: map[string]int64{services.MeterNFe: -1},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "sub_user" || !got.InheritedFromUser || got.OrganizationID != org {
		t.Fatalf("fallback = %+v (%v)", got, err)
	}
	// Writes go to ORG_ only.
	if err := svc.Reserve(ctx, company, services.MeterNFe); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewAccountBillingRepository(db, cfg)
	if u, _ := repo.GetUsage(ctx, org, "2026-10-01"); u[services.MeterNFe] != 1 {
		t.Fatalf("org counter = %+v", u)
	}
	if u, _ := repo.GetUsage(ctx, "owner-dual", "2026-10-01"); u[services.MeterNFe] != 0 {
		t.Fatalf("the USER_ counter moved: %+v", u)
	}

	seedOrgSnapshot(t, org, proSnapshot("sub_org"))
	svc.Invalidate(ctx, org)
	got, err = svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "sub_org" || got.InheritedFromUser {
		t.Fatalf("ORG_ must win: %+v (%v)", got, err)
	}
}

// Review Focus 1: a company with no organization_id falls back to its owner
// for reading, and refuses every counter write.
func TestACompanyWithNoOrganizationFallsBackToItsOwner(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls)).WithUserFallback()
	company := newCompanyPK(t)
	if err := orgRepo.CreateOrganization(ctx, company, map[string]types.AttributeValue{
		repositories.AttrOwnerUserID: &types.AttributeValueMemberS{Value: "owner-noorg"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(ctx, &repositories.AccountSnapshot{
		UserID: "owner-noorg", SubscriptionID: "sub_noorg", Status: services.StatusActive, Entitled: true,
		Quotas: map[string]int64{services.MeterNFe: -1},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "sub_noorg" || got.OrganizationID != "" {
		t.Fatalf("snapshot = %+v (%v)", got, err)
	}
	if _, err := svc.PrepareUsageReservation(ctx, company, services.MeterNFe, true); err == nil {
		t.Fatal("a company with no organization must not reserve anywhere")
	}
}

// companiesUsed counts enabled companies of the organization only.
func TestCompaniesUsedCountsEnabledCompaniesOfTheOrganization(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls)).WithEnablement(services.NewFiscalConfigEnablement(
		nfeConfigRepo, nfceConfigRepo, nil, nil, nil))
	org := "org-count-" + newCompanyPK(t)
	enabled := seedCompany(t, org, "owner-count", "11222333000181", "Habilitada Ltda")
	_ = seedCompany(t, org, "owner-count", "11222333000262", "Vinculada Ltda") // linked, never configured
	_ = seedCompany(t, "org-other-"+newCompanyPK(t), "owner-count", "44555666000105", "Outra Ltda")
	seedNfeConfig(t, enabled)
	seedOrgSnapshot(t, org, proSnapshot("sub_count"))

	usage, err := svc.Usage(ctx, org, enabled)
	if err != nil {
		t.Fatal(err)
	}
	if usage[services.MeterCompanies].Used != 1 {
		t.Fatalf("companies used = %d, want 1 (linked-only companies cost nothing)", usage[services.MeterCompanies].Used)
	}
}

// seedNfeConfig gives a company an NF-e configuration, which is what makes it
// enabled (services.FiscalConfigEnablement).
func seedNfeConfig(t *testing.T, companyPK string) {
	t.Helper()
	tx, _, err := nfeConfigRepo.BuildUpsertTxItem(companyPK, map[string]types.AttributeValue{
		"prod_current_serie": &types.AttributeValueMemberN{Value: "1"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := nfeConfigRepo.TransactWrite(context.Background(), []types.TransactWriteItem{tx}); err != nil {
		t.Fatal(err)
	}
}
