package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/api-commons/accountorgs"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

type fakeSnaps struct {
	users  []repositories.AccountSnapshot
	orgs   map[string]*repositories.AccountSnapshot
	copies []string // markers that committed
	tried  int
}

func (f *fakeSnaps) ListUserSnapshots(context.Context) ([]repositories.AccountSnapshot, error) {
	return f.users, nil
}
func (f *fakeSnaps) GetOrg(_ context.Context, id string) (*repositories.AccountSnapshot, error) {
	return f.orgs[id], nil
}
func (f *fakeSnaps) CopyUsageOnce(_ context.Context, _ []repositories.UsageSource, _, _, marker string) (bool, error) {
	f.tried++
	for _, m := range f.copies {
		if m == marker {
			return false, nil
		}
	}
	f.copies = append(f.copies, marker)
	return true, nil
}

type fakeBilling struct {
	ent       map[string]*billingclient.Entitlements // by external ref
	created   []string                               // "customer:prices"
	cancelled []string
	cancelErr error
}

func (f *fakeBilling) GetEntitlements(_ context.Context, ref string) (*billingclient.Entitlements, error) {
	if e, ok := f.ent[ref]; ok {
		return e, nil
	}
	return nil, billingclient.ErrCustomerNotFound
}
func (f *fakeBilling) GetCustomer(context.Context, string) (*billingclient.Customer, error) {
	return &billingclient.Customer{ID: "cus_user", Name: "Dono", Email: "dono@example.com"}, nil
}
func (f *fakeBilling) CreateSubscription(_ context.Context, customerID string, items []billingclient.Item, _ string) (*billingclient.SubscriptionResult, error) {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.PriceID)
	}
	f.created = append(f.created, customerID+":"+strings.Join(ids, ","))
	return &billingclient.SubscriptionResult{}, nil
}
func (f *fakeBilling) CancelSubscription(_ context.Context, id string, _ bool, _ string) (*billingclient.Subscription, error) {
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	f.cancelled = append(f.cancelled, id)
	// Billing now reports the user customer with nothing entitled.
	for ref, e := range f.ent {
		if strings.HasPrefix(ref, "USER_") {
			for i := range e.Subscriptions {
				if e.Subscriptions[i].ID == id {
					e.Subscriptions[i].Entitled = false
				}
			}
		}
	}
	return &billingclient.Subscription{ID: id}, nil
}

type fakeOrgBilling struct{ snaps *fakeSnaps }

func (f fakeOrgBilling) GetOrCreateCustomer(_ context.Context, org string, _ services.CustomerPayer) (string, error) {
	return "cus_" + org, nil
}
func (f fakeOrgBilling) Sync(_ context.Context, org string) (*repositories.AccountSnapshot, error) {
	s := &repositories.AccountSnapshot{OrganizationID: org, SubscriptionID: "sub_" + org, Status: services.StatusActive, Entitled: true, PeriodStart: "2026-10-15"}
	f.snaps.orgs[org] = s
	return s, nil
}

type fakeWorkspaces struct{ orgs []accountorgs.Organization }

func (f fakeWorkspaces) Organizations(context.Context, string) ([]accountorgs.Organization, error) {
	return f.orgs, nil
}

type fakeCompanies struct {
	all        []string
	byOrg      map[string][]repositories.CompanyRef
	gaps       []repositories.CompanyIndexGap
	backfilled []string
	lag        map[string]lagged
	lagReads   int
	owners     map[string]string
}

func (f *fakeCompanies) ListCompaniesOfOrganization(_ context.Context, org string) ([]repositories.CompanyRef, error) {
	out := append([]repositories.CompanyRef(nil), f.byOrg[org]...)
	for pk, l := range f.lag {
		if l.org != org {
			continue
		}
		if l.after > 0 {
			l.after--
			f.lag[pk] = l
			continue
		}
		out = append(out, repositories.CompanyRef{PK: pk})
	}
	return out, nil
}

func (f *fakeCompanies) CompanyOwner(_ context.Context, pk string) (string, error) {
	return f.owners[pk], nil
}
func (f *fakeCompanies) ListOrganizationsWithCompanies(context.Context) ([]string, error) {
	return f.all, nil
}
func (f *fakeCompanies) ListCompanyIndexGaps(context.Context) ([]repositories.CompanyIndexGap, error) {
	return f.gaps, nil
}
func (f *fakeCompanies) BackfillIndexKeys(_ context.Context, pk, org, _ string) error {
	f.backfilled = append(f.backfilled, pk+"="+org)
	if f.lag != nil && org != "" {
		f.lag[pk] = lagged{org: org, after: f.lagReads}
	}
	return nil
}

// lagged is a backfilled company the organization-index shows only after a
// number of reads: the GSI is eventually consistent.
type lagged struct {
	org   string
	after int
}

type fakeLevels struct{ orgs []string }

func (f *fakeLevels) MarkAndFlush(_ context.Context, org string) error {
	f.orgs = append(f.orgs, org)
	return nil
}

type fakeReach map[string]string // company → organization

func (f fakeReach) Reach(_ context.Context, company, _ string) (string, bool, error) {
	org, ok := f[company]
	return org, ok, nil
}

func entitled(subID string, unitAmount int64) *billingclient.Entitlements {
	return &billingclient.Entitlements{CustomerID: "cus_user", Subscriptions: []billingclient.EntitlementSubscription{{
		ID: subID, Status: services.StatusActive, Entitled: true,
		Period: billingclient.Period{Start: "2026-10-01"},
		Items:  []billingclient.EntitlementItem{{PriceID: "price_dfe_unlimited_internal_monthly", UnitAmount: unitAmount}},
	}}}
}

func fixture(unitAmount int64) (deps, *fakeSnaps, *fakeBilling, *fakeCompanies) {
	d, snaps, bill, comps, _ := fixtureWithLevels(unitAmount)
	return d, snaps, bill, comps
}

func fixtureWithLevels(unitAmount int64) (deps, *fakeSnaps, *fakeBilling, *fakeCompanies, *fakeLevels) {
	snaps := &fakeSnaps{
		users: []repositories.AccountSnapshot{{UserID: "u1", SubscriptionID: "sub_u1", Entitled: true, PeriodStart: "2026-09-01"}},
		orgs:  map[string]*repositories.AccountSnapshot{},
	}
	bill := &fakeBilling{ent: map[string]*billingclient.Entitlements{"USER_u1": entitled("sub_u1", unitAmount)}}
	levels := &fakeLevels{}
	comps := &fakeCompanies{byOrg: map[string][]repositories.CompanyRef{"org_a": {{PK: "cmp_a"}}, "org_b": {{PK: "cmp_b"}}}}
	d := deps{
		snaps:   snaps,
		billing: bill,
		orgs:    fakeOrgBilling{snaps: snaps},
		workspaces: fakeWorkspaces{orgs: []accountorgs.Organization{
			{ID: "org_a", Role: "owner", Kind: "organization"},
			{ID: "org_b", Role: "owner"},
			{ID: "org_admin", Role: "admin", Kind: "organization"}, // not owned
			{ID: "sp_1", Role: "owner", Kind: "personal"},          // a space
			{ID: "org_empty", Role: "owner", Kind: "organization"}, // no DF-e companies
		}},
		companies: comps,
		levels:    levels,
		reach:     fakeReach{},
		now:       func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		sleep:     func(time.Duration) {},
	}
	return d, snaps, bill, comps, levels
}

// Spec § 5 test 5: --dry-run writes nothing.
func TestDryRunWritesNothing(t *testing.T) {
	d, snaps, bill, _ := fixture(0)
	rep, err := run(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.created) != 0 || len(bill.cancelled) != 0 || snaps.tried != 0 || len(snaps.orgs) != 0 {
		t.Fatalf("dry run wrote: created=%v cancelled=%v copies=%d", bill.created, bill.cancelled, snaps.tried)
	}
	if len(rep.Migrated) != 2 {
		t.Fatalf("plan = %v, want org_a and org_b", rep.Migrated)
	}
}

func TestApplyMigratesEachOwnedOrganizationAndCancelsTheUserSubscription(t *testing.T) {
	d, snaps, bill, _, levels := fixtureWithLevels(0)
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	want := []string{"cus_org_a:price_dfe_unlimited_internal_monthly", "cus_org_b:price_dfe_unlimited_internal_monthly"}
	if strings.Join(bill.created, "|") != strings.Join(want, "|") {
		t.Fatalf("created = %v, want %v (same price id)", bill.created, want)
	}
	if len(snaps.copies) != 2 {
		t.Fatalf("counter copies = %v", snaps.copies)
	}
	if len(bill.cancelled) != 1 || bill.cancelled[0] != "sub_u1" {
		t.Fatalf("cancelled = %v", bill.cancelled)
	}
	// The initial dfe_companies level of each migrated organization.
	if strings.Join(levels.orgs, ",") != "org_a,org_b" {
		t.Fatalf("levels = %v", levels.orgs)
	}
}

func TestReportLevelsAllMarksEveryOrganizationWithCompanies(t *testing.T) {
	d, _, _, comps, levels := fixtureWithLevels(0)
	comps.all = []string{"org_a", "org_b", "org_unmigrated"}
	d.reportLevelsAll = true

	rep, err := run(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels.orgs) != 0 || strings.Join(rep.Levels, ",") != "org_a,org_b,org_unmigrated" {
		t.Fatalf("dry run: levels=%v listed=%v", levels.orgs, rep.Levels)
	}

	levels.orgs = nil
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	// Migrated organizations are marked by the migration and again by the
	// flag: harmless, a level is the whole count.
	if !strings.Contains(strings.Join(levels.orgs, ","), "org_unmigrated") {
		t.Fatalf("levels = %v, want every organization including the unmigrated one", levels.orgs)
	}
}

func TestDryRunReportsNoLevel(t *testing.T) {
	d, _, _, _, levels := fixtureWithLevels(0)
	if _, err := run(context.Background(), d, false); err != nil {
		t.Fatal(err)
	}
	if len(levels.orgs) != 0 {
		t.Fatalf("dry run reported levels: %v", levels.orgs)
	}
}

// Review Focus 4 / spec § 5 test 5: --apply twice creates nothing the second
// time, including after a run that failed to cancel.
func TestApplyTwiceCreatesNothingTheSecondTime(t *testing.T) {
	d, snaps, bill, _ := fixture(0)
	bill.cancelErr = errors.New("billing down")
	if _, err := run(context.Background(), d, true); err == nil {
		t.Fatal("the failed cancel must surface")
	}
	bill.cancelErr = nil
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	if len(bill.created) != 2 {
		t.Fatalf("created = %v, want one subscription per organization in total", bill.created)
	}
	if len(snaps.copies) != 2 {
		t.Fatalf("counters copied %d times, want 2 (once per organization)", len(snaps.copies))
	}
	if len(bill.cancelled) != 1 {
		t.Fatalf("cancelled = %v", bill.cancelled)
	}
}

// Spec § 5 test 5: a paid subscription is listed and skipped.
func TestAPaidSubscriptionIsListedAndSkipped(t *testing.T) {
	d, _, bill, _ := fixture(35000)
	rep, err := run(context.Background(), d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.created) != 0 || len(bill.cancelled) != 0 {
		t.Fatalf("a paid subscription was touched: %v %v", bill.created, bill.cancelled)
	}
	if !rep.NeedsReview() || !strings.Contains(strings.Join(rep.Review, "\n"), "USER_u1") {
		t.Fatalf("review = %v", rep.Review)
	}
}

func TestIndexGapsAreBackfilledThroughReach(t *testing.T) {
	d, _, _, comps := fixture(0)
	comps.gaps = []repositories.CompanyIndexGap{
		{PK: "cmp_gap", OwnerUserID: "u1", MissingOrganization: true},
		{PK: "cmp_orphan", OwnerUserID: "u9", MissingOrganization: true},
		{PK: "cmp_nodate", MissingCreatedAt: true},
	}
	d.reach = fakeReach{"cmp_gap": "org_a"}
	rep, err := run(context.Background(), d, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(comps.backfilled, ",") != "cmp_gap=org_a,cmp_nodate=" {
		t.Fatalf("backfilled = %v", comps.backfilled)
	}
	if len(rep.Unresolved) != 1 || !strings.HasPrefix(rep.Unresolved[0], "cmp_orphan") {
		t.Fatalf("unresolved = %v", rep.Unresolved)
	}
}

// Review round 2, important 2: a company backfilled into the organization-index
// is waited for before the organization's level is reported, so the initial
// level does not miss it.
func TestBackfilledCompaniesAreWaitedForBeforeTheLevel(t *testing.T) {
	d, _, _, comps, _ := fixtureWithLevels(0)
	comps.gaps = []repositories.CompanyIndexGap{{PK: "cmp_gap", OwnerUserID: "u1", MissingOrganization: true}}
	comps.lag = map[string]lagged{}
	comps.lagReads = 2
	d.reach = fakeReach{"cmp_gap": "org_a"}
	var sleeps int
	d.sleep = func(time.Duration) { sleeps++ }
	rep, err := run(context.Background(), d, true)
	if err != nil {
		t.Fatal(err)
	}
	if sleeps == 0 {
		t.Fatal("the run did not wait for the index to show the backfilled company")
	}
	if rep.NeedsReview() {
		t.Fatalf("review = %v %v", rep.Review, rep.Unresolved)
	}
}

func TestACompanyTheIndexNeverShowsIsListedForReview(t *testing.T) {
	d, _, _, comps, _ := fixtureWithLevels(0)
	comps.gaps = []repositories.CompanyIndexGap{{PK: "cmp_gap", OwnerUserID: "u1", MissingOrganization: true}}
	comps.lag = map[string]lagged{}
	comps.lagReads = 1000
	d.reach = fakeReach{"cmp_gap": "org_a"}
	d.sleep = func(time.Duration) {}
	rep, err := run(context.Background(), d, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(rep.Review, "\n"), "cmp_gap") {
		t.Fatalf("review = %v, want cmp_gap listed", rep.Review)
	}
}

// Review round 2, minor 7: an organization whose companies run today on a
// user's plan through the dual read, and that the migration does not assign to
// that user, is listed: closing the window would silently downgrade it.
func TestOrganizationsThatWouldLoseAnInheritedPlanAreListed(t *testing.T) {
	d, _, _, comps, _ := fixtureWithLevels(0)
	comps.all = []string{"org_a", "org_b", "org_x"}
	comps.byOrg["org_x"] = []repositories.CompanyRef{{PK: "cmp_x"}}
	comps.owners = map[string]string{"cmp_a": "u1", "cmp_b": "u1", "cmp_x": "u1"}

	rep, err := run(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	review := strings.Join(rep.Review, "\n")
	if !strings.Contains(review, "ORG_org_x") || !strings.Contains(review, "USER_u1") {
		t.Fatalf("review = %v, want org_x listed as inheriting from USER_u1", rep.Review)
	}
	if strings.Contains(review, "ORG_org_a") || strings.Contains(review, "ORG_org_b") {
		t.Fatalf("review lists an organization the migration assigns to its owner: %v", rep.Review)
	}
}
