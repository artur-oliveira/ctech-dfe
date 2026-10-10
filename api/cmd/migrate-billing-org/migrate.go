package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Idempotency keys and markers. Derived from the user and organization so a
// re-run repeats the same intent, which billing and the marker row recognise.
func migrateSubscriptionKey(sub, org string) string { return "migrate:" + sub + ":" + org }
func migrateUsageMarker(sub, org string) string     { return "migrate-usage:" + sub + ":" + org }
func migrateCancelKey(subscriptionID string) string { return "migrate-cancel:" + subscriptionID }

// calendarPeriodLayout matches services.usagePeriod's fallback.
const calendarPeriodLayout = "2006-01"

type snapshotStore interface {
	ListUserSnapshots(ctx context.Context) ([]repositories.AccountSnapshot, error)
	GetOrg(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error)
	CopyUsageOnce(ctx context.Context, sources []repositories.UsageSource, toAccount, toPeriod, marker string) (bool, error)
}

type billingAPI interface {
	GetEntitlements(ctx context.Context, externalRef string) (*billingclient.Entitlements, error)
	GetCustomer(ctx context.Context, customerID string) (*billingclient.Customer, error)
	CreateSubscription(ctx context.Context, customerID string, items []billingclient.Item, key string) (*billingclient.SubscriptionResult, error)
	CancelSubscription(ctx context.Context, subscriptionID string, atPeriodEnd bool, key string) (*billingclient.Subscription, error)
}

type orgBilling interface {
	GetOrCreateCustomer(ctx context.Context, organizationID string, payer services.CustomerPayer) (string, error)
	Sync(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error)
}

type workspaceLister interface {
	Organizations(ctx context.Context, userID string) ([]accountclient.Workspace, error)
}

type companyStore interface {
	ListOrganizationsWithCompanies(ctx context.Context) ([]string, error)
	ListCompaniesOfOrganization(ctx context.Context, organizationID string) ([]repositories.CompanyRef, error)
	ListCompanyIndexGaps(ctx context.Context) ([]repositories.CompanyIndexGap, error)
	BackfillIndexKeys(ctx context.Context, companyPK, organizationID, createdAt string) error
	// CompanyOwner is the company's owner_user_id: whose USER_ snapshot the
	// dual read serves it today.
	CompanyOwner(ctx context.Context, companyPK string) (string, error)
}

type levelReporter interface {
	MarkAndFlush(ctx context.Context, organizationID string) error
}

type reacher interface {
	Reach(ctx context.Context, companyID, userID string) (string, bool, error)
}

type deps struct {
	snaps      snapshotStore
	billing    billingAPI
	orgs       orgBilling
	workspaces workspaceLister
	companies  companyStore
	levels     levelReporter
	reach      reacher // nil: nothing can be backfilled, gaps are listed
	// reportLevelsAll is -report-levels-all.
	reportLevelsAll bool
	// sleep waits between reads of the organization-index after a backfill
	// (nil: time.Sleep).
	sleep func(time.Duration)
	now   func() time.Time
}

// Report is what the run did (or, dry, would do).
type Report struct {
	Apply      bool
	Backfilled []string
	Unresolved []string
	Migrated   []string
	Review     []string
	Levels     []string // organizations whose level was (or, dry, would be) reported by -report-levels-all

	// assigned is organization → the user whose subscription the migration
	// gives it (dry or applied).
	assigned map[string]string
}

func (r *Report) NeedsReview() bool { return len(r.Review) > 0 || len(r.Unresolved) > 0 }

func (r *Report) Print(w io.Writer) {
	mode := "DRY RUN (nothing written; pass -apply)"
	if r.Apply {
		mode = "APPLIED"
	}
	_, _ = fmt.Fprintf(w, "migrate-billing-org: %s\n", mode)
	sections := []struct {
		title string
		lines []string
	}{
		{"index backfill", r.Backfilled}, {"index unresolved", r.Unresolved},
		{"migrated", r.Migrated}, {"levels reported", r.Levels}, {"needs review", r.Review},
	}
	for _, sec := range sections {
		_, _ = fmt.Fprintf(w, "\n%s (%d)\n", sec.title, len(sec.lines))
		for _, l := range sec.lines {
			_, _ = fmt.Fprintf(w, "  %s\n", l)
		}
	}
}

func run(ctx context.Context, d deps, apply bool) (*Report, error) {
	rep := &Report{Apply: apply, assigned: map[string]string{}}
	if err := checkCompanyIndex(ctx, d, apply, rep); err != nil {
		return rep, fmt.Errorf("company index: %w", err)
	}
	users, err := d.snaps.ListUserSnapshots(ctx)
	if err != nil {
		return rep, err
	}
	for _, u := range users {
		if u.SubscriptionID == "" {
			continue
		}
		if err := migrateUser(ctx, d, apply, u, rep); err != nil {
			return rep, fmt.Errorf("%s: %w", repositories.AccountBillingPK(u.UserID), err)
		}
	}
	if err := listInheritanceLosses(ctx, d, users, rep); err != nil {
		return rep, fmt.Errorf("inheritance check: %w", err)
	}
	if d.reportLevelsAll {
		if err := reportAllLevels(ctx, d, apply, rep); err != nil {
			return rep, fmt.Errorf("levels: %w", err)
		}
	}
	return rep, nil
}

// listInheritanceLosses lists every organization with no subscription of its
// own whose companies run today on a user's plan through the dual read
// (owner_user_id → that user's USER_ snapshot), when the migration does not give
// it that user's subscription. Closing the dual-read window would silently
// leave it without a plan, so a person must look at it first.
func listInheritanceLosses(ctx context.Context, d deps, users []repositories.AccountSnapshot, rep *Report) error {
	subscribed := map[string]bool{}
	for _, u := range users {
		if u.SubscriptionID != "" {
			subscribed[repositories.RawUserID(u.UserID)] = true
		}
	}
	orgs, err := d.companies.ListOrganizationsWithCompanies(ctx)
	if err != nil {
		return err
	}
	for _, org := range orgs {
		own, err := d.snaps.GetOrg(ctx, org)
		if err != nil {
			return err
		}
		if own != nil && own.SubscriptionID != "" {
			continue // it has its own plan: the dual read never served it
		}
		refs, err := d.companies.ListCompaniesOfOrganization(ctx, org)
		if err != nil {
			return err
		}
		listed := map[string]bool{}
		for _, ref := range refs {
			owner, err := d.companies.CompanyOwner(ctx, ref.PK)
			if err != nil {
				return err
			}
			owner = repositories.RawUserID(owner)
			if owner == "" || !subscribed[owner] || rep.assigned[org] == owner || listed[owner] {
				continue
			}
			listed[owner] = true
			target := "no subscription"
			if a := rep.assigned[org]; a != "" {
				target = repositories.AccountBillingPK(a) + "'s subscription"
			}
			rep.Review = append(rep.Review, fmt.Sprintf(
				"%s: company %s runs today on %s's plan (dual read); the migration gives the organization %s, so it loses that plan when the dual read closes",
				repositories.OrgBillingPK(org), ref.PK, repositories.AccountBillingPK(owner), target))
		}
	}
	return nil
}

// reportAllLevels marks (and flushes) the companies level of every
// organization holding DF-e companies, so an organization that never
// subscribed still has its level known. Dry: list only.
func reportAllLevels(ctx context.Context, d deps, apply bool, rep *Report) error {
	orgs, err := d.companies.ListOrganizationsWithCompanies(ctx)
	if err != nil {
		return err
	}
	for _, org := range orgs {
		if apply {
			if err := d.levels.MarkAndFlush(ctx, org); err != nil {
				return fmt.Errorf("%s: %w", org, err)
			}
		}
		rep.Levels = append(rep.Levels, org)
	}
	return nil
}

func checkCompanyIndex(ctx context.Context, d deps, apply bool, rep *Report) error {
	gaps, err := d.companies.ListCompanyIndexGaps(ctx)
	if err != nil {
		return err
	}
	for _, g := range gaps {
		org := ""
		if g.MissingOrganization {
			if d.reach == nil || g.OwnerUserID == "" {
				rep.Unresolved = append(rep.Unresolved, g.PK+": no organization_id and nothing to resolve it with")
				continue
			}
			got, ok, err := d.reach.Reach(ctx, g.PK, g.OwnerUserID)
			if err != nil {
				return err
			}
			if !ok || got == "" {
				rep.Unresolved = append(rep.Unresolved, g.PK+": ctech-account does not place its owner on it")
				continue
			}
			org = got
		}
		createdAt := ""
		if g.MissingCreatedAt {
			createdAt = d.now().UTC().Format(time.RFC3339)
		}
		if apply {
			if err := d.companies.BackfillIndexKeys(ctx, g.PK, org, createdAt); err != nil {
				return err
			}
			// The index is a GSI and eventually consistent: what follows (the
			// owned organizations, the initial companies level) reads it, so
			// the backfilled company must be visible first.
			if org != "" {
				visible, err := waitIndexed(ctx, d, org, g.PK)
				if err != nil {
					return err
				}
				if !visible {
					rep.Review = append(rep.Review, fmt.Sprintf("%s: backfilled into %s but not yet visible in organization-index; rerun with -report-levels-all once it is", g.PK, org))
				}
			}
		}
		rep.Backfilled = append(rep.Backfilled, g.PK)
	}
	return nil
}

// Waiting for the organization-index after a backfill: GSI propagation is
// usually well under a second.
const (
	indexWaitAttempts = 20
	indexWaitInterval = 500 * time.Millisecond
)

// waitIndexed reads the organization-index until it lists companyPK under
// organizationID, a bounded number of times.
func waitIndexed(ctx context.Context, d deps, organizationID, companyPK string) (bool, error) {
	sleep := d.sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	for attempt := 0; attempt < indexWaitAttempts; attempt++ {
		refs, err := d.companies.ListCompaniesOfOrganization(ctx, organizationID)
		if err != nil {
			return false, err
		}
		for _, r := range refs {
			if r.PK == companyPK {
				return true, nil
			}
		}
		sleep(indexWaitInterval)
	}
	return false, nil
}

func migrateUser(ctx context.Context, d deps, apply bool, u repositories.AccountSnapshot, rep *Report) error {
	userRef := repositories.AccountBillingPK(u.UserID)
	ent, err := d.billing.GetEntitlements(ctx, userRef)
	if errors.Is(err, billingclient.ErrCustomerNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	sub := firstEntitled(ent.Subscriptions)
	if sub == nil {
		return nil // already cancelled: migrated by an earlier run, or never live
	}
	for _, it := range sub.Items {
		if it.UnitAmount != 0 {
			rep.Review = append(rep.Review, fmt.Sprintf("%s: subscription %s has a paid price (%s); not migrated, nothing duplicated", userRef, sub.ID, it.PriceID))
			return nil
		}
	}
	orgs, err := ownedOrganizationsWithCompanies(ctx, d, u.UserID)
	if err != nil {
		return err
	}
	if len(orgs) == 0 {
		rep.Review = append(rep.Review, fmt.Sprintf("%s: owns no organization with DF-e companies; subscription %s left as is", userRef, sub.ID))
		return nil
	}
	for _, org := range orgs {
		rep.assigned[org] = u.UserID
	}
	priceIDs := make([]string, 0, len(sub.Items))
	items := make([]billingclient.Item, 0, len(sub.Items))
	for _, it := range sub.Items {
		priceIDs = append(priceIDs, it.PriceID)
		items = append(items, billingclient.Item{PriceID: it.PriceID})
	}
	if !apply {
		for _, org := range orgs {
			rep.Migrated = append(rep.Migrated, fmt.Sprintf("%s → %s [%s]", userRef, repositories.OrgBillingPK(org), strings.Join(priceIDs, ",")))
		}
		return nil
	}

	customer, err := d.billing.GetCustomer(ctx, ent.CustomerID)
	if err != nil {
		return err
	}
	payer := services.CustomerPayer{UserID: u.UserID, Name: customer.Name, Email: customer.Email}
	allLive := true
	for _, org := range orgs {
		target, err := d.snaps.GetOrg(ctx, org)
		if err != nil {
			return err
		}
		if target == nil || !services.GrantsService(target) {
			customerID, err := d.orgs.GetOrCreateCustomer(ctx, org, payer)
			if err != nil {
				return err
			}
			if _, err := d.billing.CreateSubscription(ctx, customerID, items, migrateSubscriptionKey(u.UserID, org)); err != nil {
				return err
			}
			if target, err = d.orgs.Sync(ctx, org); err != nil {
				return err
			}
		}
		if !services.GrantsService(target) {
			allLive = false
			rep.Review = append(rep.Review, fmt.Sprintf("%s: %s is %q after creation; USER_ subscription kept", userRef, org, target.Status))
			continue
		}
		toPeriod := target.PeriodStart
		if toPeriod == "" {
			toPeriod = d.now().Format(calendarPeriodLayout)
		}
		if _, err := d.snaps.CopyUsageOnce(ctx, usageSources(u, sub, org), org, toPeriod, migrateUsageMarker(u.UserID, org)); err != nil {
			return err
		}
		// The organization's initial companies level (whole count). The
		// marker is durable: if billing does not take it now, the API's
		// sweeper delivers it.
		if err := d.levels.MarkAndFlush(ctx, org); err != nil {
			return err
		}
		rep.Migrated = append(rep.Migrated, fmt.Sprintf("%s → %s [%s]", userRef, repositories.OrgBillingPK(org), strings.Join(priceIDs, ",")))
	}
	if !allLive {
		return nil
	}
	_, err = d.billing.CancelSubscription(ctx, sub.ID, false, migrateCancelKey(sub.ID))
	return err
}

// usageSources are the counters the organization inherits: the user's for the
// current period, and whatever the dual read already wrote to the organization
// under the user's period. Both the subscription's period and the possibly
// stale USER_ row's period are read; CopyUsageOnce skips the target itself.
func usageSources(u repositories.AccountSnapshot, sub *billingclient.EntitlementSubscription, org string) []repositories.UsageSource {
	periods := []string{sub.Period.Start}
	if u.PeriodStart != "" && u.PeriodStart != sub.Period.Start {
		periods = append(periods, u.PeriodStart)
	}
	out := []repositories.UsageSource{}
	for _, p := range periods {
		if p == "" {
			continue
		}
		out = append(out,
			repositories.UsageSource{AccountID: u.UserID, Period: p},
			repositories.UsageSource{AccountID: org, Period: p},
		)
	}
	return out
}

func firstEntitled(subs []billingclient.EntitlementSubscription) *billingclient.EntitlementSubscription {
	for i := range subs {
		if subs[i].Entitled {
			return &subs[i]
		}
	}
	return nil
}

// ownedOrganizationsWithCompanies is spec § 4 step 2: organizations (not
// spaces) the user owns that hold at least one DF-e company.
func ownedOrganizationsWithCompanies(ctx context.Context, d deps, userID string) ([]string, error) {
	all, err := d.workspaces.Organizations(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, w := range all {
		if w.Role != services.AccountRoleOwner || !w.IsOrganization() {
			continue
		}
		refs, err := d.companies.ListCompaniesOfOrganization(ctx, w.ID)
		if err != nil {
			return nil, err
		}
		if len(refs) > 0 {
			out = append(out, w.ID)
		}
	}
	return out, nil
}
