//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
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

// fakeRoles stands in for ctech-account's membership route.
type fakeRoles struct {
	roles map[string]string // userID → role
	err   error
}

func (f fakeRoles) Role(_ context.Context, _, userID string) (string, error) {
	return f.roles[userID], f.err
}
func (f fakeRoles) OrganizationName(context.Context, string, string) string { return "Escritório" }

// managementStub answers what Choose/Change/Cancel need: the catalogue, the
// entitlements, customer and subscription creation. It records creations.
type managementStub struct {
	customers     []map[string]any
	subscriptions int
	entitled      bool
}

func (m *managementStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/products", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"prod_dfe_free","name":"Free","active":true,"prices":[{"id":"price_dfe_free","product_id":"prod_dfe_free","unit_amount":0,"metadata":{"plan":"free","quota_nfe":"3","quota_companies":"1"}}]}]}`))
	})
	// The list carries no prices; billingclient reads each product's detail.
	mux.HandleFunc("/v1.0/products/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"prod_dfe_free","name":"Free","active":true,"prices":[{"id":"price_dfe_free","product_id":"prod_dfe_free","active":true,"unit_amount":0,"metadata":{"plan":"free","quota_nfe":"3","quota_companies":"1"}}]}`))
	})
	mux.HandleFunc("/v1.0/entitlements", func(w http.ResponseWriter, _ *http.Request) {
		if !m.entitled {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"customer_id":"cus_org","entitled":true,"subscriptions":[{"id":"sub_org","status":"ACTIVE","entitled":true,"plan":"free","items":[{"price_id":"price_dfe_free","unit_amount":0,"metadata":{"plan":"free","quota_nfe":"3","quota_companies":"1"}}],"current_period":{"start":"2026-10-10","end":"2026-11-10"}}]}`))
	})
	mux.HandleFunc("/v1.0/customers", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.customers = append(m.customers, body)
		_, _ = w.Write([]byte(`{"id":"cus_org","external_ref":"` + body["external_ref"].(string) + `"}`))
	})
	mux.HandleFunc("/v1.0/subscriptions", func(w http.ResponseWriter, _ *http.Request) {
		m.subscriptions++
		m.entitled = true
		_, _ = w.Write([]byte(`{"subscription":{"id":"sub_org","customer_id":"cus_org","status":"ACTIVE"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// Spec § 5 test 3: member → 403 on choose/change/cancel; admin → allowed; both
// read the plan.
func TestOnlyOwnerAndAdminManageThePlan(t *testing.T) {
	ctx := context.Background()
	stub := &managementStub{}
	svc := chargingBilling(t, stub.server(t)).WithWorkspaceRoles(fakeRoles{roles: map[string]string{
		"usr-admin": services.AccountRoleAdmin, "usr-member": "member",
	}})
	org := "org-manage-" + newCompanyPK(t)
	company := seedCompany(t, org, "usr-owner", "11222333000181", "Gerida Ltda")

	member, err := svc.ScopeFor(ctx, company, "usr-member")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Choose(ctx, member, "", []string{"price_dfe_free"}); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member choose: %v, want 403", err)
	}
	if _, _, err := svc.Change(ctx, member, []string{"price_dfe_free"}); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member change: %v, want 403", err)
	}
	if _, err := svc.Cancel(ctx, member, true); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member cancel: %v, want 403", err)
	}
	if _, err := svc.Invoices(ctx, member, 2026, 10); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member invoices: %v, want 403", err)
	}
	if _, err := svc.SnapshotOf(ctx, member); err != nil {
		t.Fatalf("a member reads the plan: %v", err)
	}

	admin, err := svc.ScopeFor(ctx, company, "usr-admin")
	if err != nil {
		t.Fatal(err)
	}
	snap, _, err := svc.Choose(ctx, admin, "", []string{"price_dfe_free"})
	if err != nil || snap.SubscriptionID != "sub_org" || snap.OrganizationID != org {
		t.Fatalf("admin choose = %+v (%v)", snap, err)
	}
	if len(stub.customers) != 1 || stub.customers[0]["external_ref"] != repositories.OrgBillingPK(org) {
		t.Fatalf("customers = %+v", stub.customers)
	}
	// The organization's billing company names the invoice.
	if stub.customers[0]["tax_id"] != "11222333000181" || stub.customers[0]["name"] != "Gerida Ltda" {
		t.Fatalf("customer identity = %+v", stub.customers[0])
	}
	// Billing refuses user_id on an organization customer (422 not_allowed).
	if uid, has := stub.customers[0]["user_id"]; has && uid != "" {
		t.Fatalf("an ORG_ customer must carry no user_id: %+v", stub.customers[0])
	}
	if ok, _ := svc.CanManage(ctx, member); ok {
		t.Fatal("a member must not be reported manageable")
	}
}

// Review Focus 2: ctech-account down → 403, never a grant; reading still works.
func TestManagingFailsClosedWhenTheRoleCannotBeRead(t *testing.T) {
	ctx := context.Background()
	stub := &managementStub{}
	svc := chargingBilling(t, stub.server(t)).WithWorkspaceRoles(fakeRoles{err: errors.New("down")})
	company := seedCompany(t, "org-down-"+newCompanyPK(t), "usr-owner", "11222333000181", "Fora Ltda")
	scope, err := svc.ScopeFor(ctx, company, "usr-owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Choose(ctx, scope, "", []string{"price_dfe_free"}); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("choose during an outage: %v, want 403", err)
	}
	if stub.subscriptions != 0 {
		t.Fatal("nothing may be created when the role is unknown")
	}
	if ok, err := svc.CanManage(ctx, scope); ok || err == nil {
		t.Fatalf("CanManage = %v, %v", ok, err)
	}
	if _, err := svc.SnapshotOf(ctx, scope); err != nil {
		t.Fatalf("reading must survive the outage: %v", err)
	}
}

// isStatus reports a problem with the given HTTP status.
func isStatus(err error, status int) bool {
	var p *problem.Problem
	return errors.As(err, &p) && p.Status == status
}
func companyQuotaBilling(t *testing.T, calls *[]usageCall) *services.BillingService {
	t.Helper()
	return chargingBilling(t, billingStub(t, calls)).WithEnablement(services.NewFiscalConfigEnablement(
		nfeConfigRepo, nfceConfigRepo, nil, nil, nil))
}

// Spec § 3 O5: the company quota applies when a company becomes enabled.
func TestReserveCompanyRefusesAboveTheLimitAndIgnoresLinkedCompanies(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-reserve-" + newCompanyPK(t)
	enabled := seedCompany(t, org, "owner-r", "11222333000181", "Um Ltda")
	seedNfeConfig(t, enabled)
	// Spec § 5 test 8: linked companies above the limit are fine; they cost nothing.
	for _, tax := range []string{"11222333000262", "11222333000343"} {
		_ = seedCompany(t, org, "owner-r", tax, "Vinculada Ltda")
	}
	snap := proSnapshot("sub_reserve")
	snap.Quotas[services.MeterCompanies] = 1
	seedOrgSnapshot(t, org, snap)
	candidate := seedCompany(t, org, "owner-r", "11222333000424", "Nova Ltda")

	if _, err := svc.ReserveCompany(ctx, org, candidate); !isStatus(err, http.StatusPaymentRequired) {
		t.Fatalf("second enabled company on a 1-company plan: %v, want 402", err)
	}
	// An already-enabled company saving another configuration is not a change.
	r, err := svc.ReserveCompany(ctx, org, enabled)
	if err != nil || len(r.Items) != 0 {
		t.Fatalf("already enabled: %+v %v", r, err)
	}
}

// An enablement carries the guard and the level marker; committing them marks
// the organization's dfe_companies level dirty with a new version.
func TestAnEnablementMarksTheCompaniesLevelDirty(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-level-" + newCompanyPK(t)
	seedOrgSnapshot(t, org, proSnapshot("sub_level"))
	company := seedCompany(t, org, "owner-l", "11222333000181", "Nível Ltda")

	r, err := svc.ReserveCompany(ctx, org, company)
	if err != nil || len(r.Items) != 2 {
		t.Fatalf("enablement items = %+v (%v), want guard + marker", r, err)
	}
	// The configuration and the reservation commit together (Task 11 wires
	// this into the fiscal configuration route).
	tx, _, err := nfeConfigRepo.BuildUpsertTxItem(company, map[string]types.AttributeValue{
		"prod_current_serie": &types.AttributeValueMemberN{Value: "1"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := nfeConfigRepo.TransactWrite(ctx, append([]types.TransactWriteItem{tx}, r.Items...)); err != nil {
		t.Fatal(err)
	}
	m, err := repositories.NewAccountBillingRepository(db, cfg).GetLevelMarker(ctx, org, services.MeterLevelCompanies)
	if err != nil || m == nil || !m.Dirty || m.Version != 1 || m.ChangedAt == "" {
		t.Fatalf("marker = %+v (%v)", m, err)
	}
}

// Levels are reported whatever the plan: a plan with no company quota still
// marks the level, so it is known the day the organization moves to on-demand.
func TestAnUncappedPlanStillMarksTheLevel(t *testing.T) {
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-uncapped-" + newCompanyPK(t)
	snap := proSnapshot("sub_uncapped")
	snap.Quotas[services.MeterCompanies] = services.QuotaUnlimited
	seedOrgSnapshot(t, org, snap)
	company := seedCompany(t, org, "owner-u", "11222333000181", "Livre Ltda")
	r, err := svc.ReserveCompany(context.Background(), org, company)
	if err != nil || len(r.Items) != 1 {
		t.Fatalf("items = %+v (%v), want only the marker", r, err)
	}
}

func TestClearLevelDirtyKeepsANewerChange(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	org := "org-clear-" + newCompanyPK(t)
	for range 2 {
		if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
			t.Fatal(err)
		}
	}
	// Clearing the version a reporter read before the second change must not
	// hide the second change.
	if err := repo.ClearLevelDirty(ctx, org, services.MeterLevelCompanies, 1); err != nil {
		t.Fatal(err)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); !m.Dirty {
		t.Fatal("a stale clear removed a newer change")
	}
	if err := repo.ClearLevelDirty(ctx, org, services.MeterLevelCompanies, 2); err != nil {
		t.Fatal(err)
	}
	m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies)
	if m.Dirty {
		t.Fatal("the current version was not cleared")
	}
	dirty, err := repo.ListDirtyLevels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirty {
		if d.OrganizationID == org {
			t.Fatal("a clean marker is still listed")
		}
	}
}

// Review Focus 1.
func TestReserveCompanyRefusesACompanyWithNoOrganization(t *testing.T) {
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	if _, err := svc.ReserveCompany(context.Background(), "", newCompanyPK(t)); !isStatus(err, http.StatusConflict) {
		t.Fatalf("err = %v, want 409", err)
	}
}
// levelSinkStub records level reports and can fail.
type levelSinkStub struct {
	reports []billingclient.LevelReport
	fail    error
}

func (l *levelSinkStub) ReportLevel(_ context.Context, in billingclient.LevelReport) error {
	if l.fail != nil {
		return l.fail
	}
	l.reports = append(l.reports, in)
	return nil
}

func TestFlushReportsTheWholeCountOnceAndClears(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	sink := &levelSinkStub{}
	rep := services.NewLevelReporter(repo, sink, svc)
	org := "org-flush-" + newCompanyPK(t)
	a := seedCompany(t, org, "owner-f", "11222333000181", "A Ltda")
	b := seedCompany(t, org, "owner-f", "11222333000262", "B Ltda")
	seedNfeConfig(t, a)
	seedNfeConfig(t, b)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := rep.Flush(ctx, org); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.reports) != 1 {
		t.Fatalf("reports = %+v, want one (the second flush finds nothing dirty)", sink.reports)
	}
	got := sink.reports[0]
	if got.CustomerRef != repositories.OrgBillingPK(org) || got.Meter != services.MeterLevelCompanies || got.Value != 2 ||
		got.IdempotencyKey != services.LevelKey(org, 1) || got.OccurredAt == "" {
		t.Fatalf("report = %+v", got)
	}
}

// Gap 12 of the planning amendment: a failed delivery is not forgotten. The
// marker stays dirty and the sweeper delivers it later, with the same key.
func TestAFailedReportStaysDirtyAndTheSweeperDeliversIt(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	sink := &levelSinkStub{fail: errors.New("billing down")}
	rep := services.NewLevelReporter(repo, sink, svc)
	org := "org-sweep-" + newCompanyPK(t)
	c := seedCompany(t, org, "owner-s", "11222333000181", "S Ltda")
	seedNfeConfig(t, c)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}

	if err := rep.Flush(ctx, org); err == nil {
		t.Fatal("a failed report must surface")
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); !m.Dirty {
		t.Fatal("a failed report cleared the marker")
	}
	sink.fail = nil
	if err := rep.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	var delivered bool
	for _, r := range sink.reports {
		if r.CustomerRef == repositories.OrgBillingPK(org) && r.Value == 1 && r.IdempotencyKey == services.LevelKey(org, 1) {
			delivered = true
		}
	}
	if !delivered {
		t.Fatalf("the sweeper did not deliver: %+v", sink.reports)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); m.Dirty {
		t.Fatal("still dirty after delivery")
	}
}

// Billing answers 409 idempotency_key_reused for a key it holds with another
// body: the version was already recorded (by another instance), so the marker
// is cleared, not retried forever.
func TestAConflictingReportCountsAsDelivered(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	rep := services.NewLevelReporter(repo, &levelSinkStub{fail: billingclient.ErrLevelAlreadyRecorded}, svc)
	org := "org-409-" + newCompanyPK(t)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}
	if err := rep.Flush(ctx, org); err != nil {
		t.Fatal(err)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); m.Dirty {
		t.Fatal("a 409 must clear the version")
	}
}
// Billing answers 409 concurrent_update when nothing was recorded: the marker
// stays dirty for the sweeper.
func TestAConcurrentUpdateReportStaysDirty(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	rep := services.NewLevelReporter(repo, &levelSinkStub{fail: problem.Conflict("concurrent")}, svc)
	org := "org-409c-" + newCompanyPK(t)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}
	if err := rep.Flush(ctx, org); err == nil {
		t.Fatal("a concurrent_update must surface")
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); !m.Dirty {
		t.Fatal("nothing was recorded; the marker must stay dirty")
	}
}

// Any plan selection makes the organization's level known: subscribing marks
// it dirty and flushes it, migrated or not.
func TestChoosingAPlanReportsTheCompaniesLevel(t *testing.T) {
	ctx := context.Background()
	stub := &managementStub{}
	svc := chargingBilling(t, stub.server(t)).
		WithWorkspaceRoles(fakeRoles{roles: map[string]string{"usr-admin": services.AccountRoleAdmin}}).
		WithEnablement(services.NewFiscalConfigEnablement(nfeConfigRepo, nfceConfigRepo, nil, nil, nil))
	repo := repositories.NewAccountBillingRepository(db, cfg)
	sink := &levelSinkStub{}
	svc.WithLevelFlush(services.NewLevelReporter(repo, sink, svc).Flush)
	org := "org-choose-level-" + newCompanyPK(t)
	company := seedCompany(t, org, "usr-owner", "11222333000181", "Escolhe Ltda")
	seedNfeConfig(t, company)

	scope, err := svc.ScopeFor(ctx, company, "usr-admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Choose(ctx, scope, "", []string{"price_dfe_free"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.reports) != 1 || sink.reports[0].CustomerRef != repositories.OrgBillingPK(org) || sink.reports[0].Value != 1 {
		t.Fatalf("reports = %+v, want the level 1 for the organization", sink.reports)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); m == nil || m.Dirty {
		t.Fatalf("marker = %+v, want written and delivered", m)
	}
}
func TestWebhookSyncsAnOrganizationAndIgnoresAUser(t *testing.T) {
	ctx := context.Background()
	org := "org-hook-" + newCompanyPK(t)
	ref := repositories.OrgBillingPK(org)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/subscriptions/sub_org", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sub_org","customer_id":"cus_org"}`))
	})
	mux.HandleFunc("/v1.0/subscriptions/sub_user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sub_user","customer_id":"cus_user"}`))
	})
	mux.HandleFunc("/v1.0/customers/cus_org", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cus_org","external_ref":"` + ref + `"}`))
	})
	mux.HandleFunc("/v1.0/customers/cus_user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cus_user","external_ref":"USER_hook-owner"}`))
	})
	mux.HandleFunc("/v1.0/entitlements", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("customer_ref") != ref {
			t.Errorf("entitlements asked for %q", r.URL.Query().Get("customer_ref"))
		}
		_, _ = w.Write([]byte(`{"customer_id":"cus_org","subscriptions":[{"id":"sub_org","status":"PAST_DUE","entitled":true,"plan":"pro","current_period":{"start":"2026-10-01","end":"2026-11-01"}}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	svc := chargingBilling(t, srv)

	if err := svc.SyncBySubscription(ctx, "sub_org"); err != nil {
		t.Fatal(err)
	}
	got, err := repositories.NewAccountBillingRepository(db, cfg).GetOrg(ctx, org)
	if err != nil || got == nil || got.Status != "PAST_DUE" {
		t.Fatalf("ORG_ snapshot = %+v (%v)", got, err)
	}
	if err := svc.SyncBySubscription(ctx, "sub_user"); err != nil {
		t.Fatalf("a USER_ customer is ignored, not an error: %v", err)
	}
	if u, _ := repositories.NewAccountBillingRepository(db, cfg).Get(ctx, "hook-owner"); u != nil {
		t.Fatalf("a USER_ webhook wrote a snapshot: %+v", u)
	}
}

// Two companies of one organization enabled together at limit-1: exactly one
// configuration commits.
func TestTwoEnablementsRaceForTheLastSlot(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-race-" + newCompanyPK(t)
	snap := proSnapshot("sub_race")
	snap.Quotas[services.MeterCompanies] = 1
	seedOrgSnapshot(t, org, snap)
	a := seedCompany(t, org, "owner-race", "11222333000181", "A Ltda")
	b := seedCompany(t, org, "owner-race", "11222333000262", "B Ltda")

	ra, err := svc.ReserveCompany(ctx, org, a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := svc.ReserveCompany(ctx, org, b)
	if err != nil {
		t.Fatal(err) // both read 0 of 1: the guard decides
	}
	nfe := services.NewNfeConfigService(nfeConfigRepo, auditRepo)
	fields := map[string]types.AttributeValue{"prod_current_serie": &types.AttributeValueMemberN{Value: "1"}}
	_, errA := nfe.Upsert(ctx, a, fields, "owner-race", "Dono", ra.Items...)
	_, errB := nfe.Upsert(ctx, b, fields, "owner-race", "Dono", rb.Items...)
	if (errA == nil) == (errB == nil) {
		t.Fatalf("exactly one must commit: errA=%v errB=%v", errA, errB)
	}
}

// O6: a company already enabled before it had an organization (a legacy
// record) keeps saving its configuration; only enabling is refused.
func TestAnEnabledCompanyWithNoOrganizationStillSaves(t *testing.T) {
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	company := newCompanyPK(t)
	seedNfeConfig(t, company)
	r, err := svc.ReserveCompany(context.Background(), "", company)
	if err != nil || len(r.Items) != 0 {
		t.Fatalf("already enabled, no organization: %+v %v", r, err)
	}
}
