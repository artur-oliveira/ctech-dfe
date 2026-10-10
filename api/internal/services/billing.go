package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// The account's billing standing: reading it, keeping it fresh, and acting on it.
//
// One rule runs through the whole file: **billing owns the subscription, this
// service owns a snapshot of it.** Every write to the snapshot comes from
// re-reading billing — never from a webhook body, never from what a mutation
// intended to happen. That is why Choose/Change/Cancel all end in Sync rather
// than patching the row with the answer they already have.

// snapshotCacheTTL is short because this backs an authorization decision.
//
// 60s, matching membershipCacheTTL, and for the same reason: it is the window in
// which a cancelled subscription still issues documents. Webhooks invalidate it
// on every real change, so the TTL only covers the case where the webhook itself
// is late or lost.
const snapshotCacheTTL = 60

// catalogCacheTTL is generous because the catalogue is a handful of rows that
// change when somebody edits a seed file, which is to say almost never.
const catalogCacheTTL = 300

// Price-metadata keys billing carries and this service reads. Billing does not
// look inside metadata by decision (ADR 0008); these names are the contract
// between the seed file (`ctech-billing/api/tenants/ctech.json`) and the quota
// enforcement here, and they exist as constants so a typo is a compile error in
// one place rather than a quota that silently never applies.
const (
	metadataKeyPlan      = "plan"
	metadataKeyMeter     = "meter"
	metadataKeyMDFEScope = "mdfe_scope"
	metadataQuotaPref    = "quota_"
	// metadataKeyVisibility marks a price nobody may subscribe to through this
	// API. See visibilityInternal.
	metadataKeyVisibility = "visibility"
)

const mdfeScopeOwnFleet = "frota_propria"

// visibilityInternal names a price that exists to be granted, never sold: the
// R$ 0 unlimited the CTech team and the first two customers run on.
//
// It is honoured in two places here, and both are needed. Plans hides it, so no
// client can discover it; Choose and Change refuse it, so knowing the id is not
// enough. Hiding alone would be a price list as an access control, which is the
// same mistake as an unlisted URL — `price_dfe_unlimited_internal_monthly` is
// written in this repository's plan document.
//
// Granting it is deliberately an operation nobody can perform from a browser:
// it goes through billing directly, with the M2M credential.
const visibilityInternal = "internal"

// QuotaUnlimited is the limit value meaning "no ceiling". It is -1 rather than
// a missing key because absent and unlimited are opposite answers: the Free
// plan's `quota_cte: 0` grants none, and omitting the key entirely would be
// indistinguishable from granting every one.
const QuotaUnlimited int64 = -1

// PlanUnlimited is the plan key reported when billing is switched off.
const PlanUnlimited = "unlimited"

// StatusActive and StatusTrialing are the two billing statuses that grant
// service in the DF-e.
//
// The list is deliberately narrower than billing's own `entitled`, which counts
// PAST_DUE as entitled — that is billing's answer for a customer whose dunning
// has not run out, and the DF-e's answer is different by decision (D2). Keeping
// both means the disagreement is visible rather than resolved by whoever read
// the field last.
const (
	StatusActive   = "ACTIVE"
	StatusTrialing = "TRIALING"
)

// orgBillingCacheKey caches an organization's snapshot. Keyed by organization,
// never by company: two companies of one organization must read one answer.
func orgBillingCacheKey(organizationID string) string {
	return "dfe:billing:org:" + organizationID
}

const catalogCacheKey = "dfe:billing:catalog"

// BillingService is the DF-e's view of ctech-billing.
type BillingService struct {
	repo   *repositories.AccountBillingRepository
	client *billingclient.Client
	users  *UserService
	orgs   *OrganizationService
	cache  cache.Backend
	// enablement reports which companies can actually emit, which is what the
	// company quota applies to (ctech-billing ADR 0021). Optional: without it
	// the quota falls back to counting owned companies, which is what it always
	// did and what ADR 0021 says is wrong.
	enablement enablementSource
	// roles answers who may manage an organization's plan; nil refuses.
	roles workspaceRoles
	// levelFlush delivers a dirty companies level (LevelReporter.Flush).
	levelFlush func(ctx context.Context, organizationID string) error
}

// WithEnablement makes the company quota count what is enabled rather than what
// is owned.
//
// Wired rather than required so the change is one line to turn off if it
// misbehaves in production — the previous count is stricter, never looser, so
// falling back cannot let somebody past a limit.
func (s *BillingService) WithEnablement(e enablementSource) *BillingService {
	s.enablement = e
	return s
}

// ErrNoOrganization refuses a counter write for a company whose record names
// no ctech-account organization: there is no ORG_ row to write to, and
// writing to the owner's would split one organization's usage across keys.
var ErrNoOrganization = problem.Conflict(
	"esta empresa não está vinculada a uma organização da conta CTech; vincule-a pela conta CTech para continuar")

// WithLevelFlush wires the level delivery (LevelReporter.Flush). Set after
// both exist, since the reporter counts through this service.
func (s *BillingService) WithLevelFlush(flush func(ctx context.Context, organizationID string) error) *BillingService {
	s.levelFlush = flush
	return s
}

// markCompaniesLevel makes the organization's dfe_companies level known to
// billing at a plan selection, so an organization that subscribes (migrated or
// not) has its level from day one. The marker is durable; a failed flush is
// left for the sweeper and never fails the plan selection.
func (s *BillingService) markCompaniesLevel(ctx context.Context, organizationID string) {
	if err := s.repo.MarkLevelDirty(ctx, organizationID, MeterLevelCompanies); err != nil {
		slog.ErrorContext(ctx, "billing: could not mark the companies level", "organization_id", organizationID, "error", err)
		return
	}
	if s.levelFlush == nil {
		return
	}
	if err := s.levelFlush(ctx, organizationID); err != nil {
		slog.WarnContext(ctx, "billing: companies level left for the sweeper", "organization_id", organizationID, "error", err)
	}
}

func NewBillingService(
	repo *repositories.AccountBillingRepository,
	client *billingclient.Client,
	users *UserService,
	orgs *OrganizationService,
	c cache.Backend,
) *BillingService {
	return &BillingService{repo: repo, client: client, users: users, orgs: orgs, cache: c}
}

// Enabled reports whether this deployment charges for anything.
func (s *BillingService) Enabled() bool { return s.client.Enabled() }

// noChargeSnapshot is what every account looks like when billing is switched
// off: unlimited, and honest about why.
func noChargeSnapshot(organizationID string) *repositories.AccountSnapshot {
	return &repositories.AccountSnapshot{
		OrganizationID: organizationID,
		Status:         StatusActive,
		Plan:           PlanUnlimited,
		Entitled:       true,
		NoCharge:       true,
	}
}

// Snapshot returns an organization's own billing standing, cache-first. It
// never calls billing: an organization with no row is "sem assinatura", which
// DynamoDB answers in a millisecond.
func (s *BillingService) Snapshot(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(organizationID), nil
	}
	if organizationID == "" {
		return &repositories.AccountSnapshot{}, nil
	}
	key := orgBillingCacheKey(organizationID)
	if v, ok := CacheGet[repositories.AccountSnapshot](ctx, s.cache, key); ok {
		return v, nil
	}
	snap, err := s.repo.GetOrg(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		snap = &repositories.AccountSnapshot{OrganizationID: organizationID}
	}
	CacheSet(ctx, s.cache, key, *snap, snapshotCacheTTL)
	return snap, nil
}

// Sync re-reads billing for ORG_{organizationID} and rewrites the snapshot.
// The only writer of ORG_ rows.
func (s *BillingService) Sync(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(organizationID), nil
	}
	ent, err := s.client.GetEntitlements(ctx, repositories.OrgBillingPK(organizationID))
	switch {
	case err == nil:
	case errors.Is(err, billingclient.ErrCustomerNotFound):
		ent = &billingclient.Entitlements{}
	default:
		return nil, err
	}
	snap := SnapshotFrom(organizationID, ent)
	if err := s.repo.Put(ctx, snap); err != nil {
		return nil, err
	}
	s.Invalidate(ctx, organizationID)
	return snap, nil
}

// Invalidate drops an organization's cached snapshot.
func (s *BillingService) Invalidate(ctx context.Context, organizationID string) {
	cacheDelete(ctx, s.cache, orgBillingCacheKey(organizationID))
}

// SnapshotFrom derives the snapshot from billing's entitlements answer.
//
// Pure, and separated from the I/O around it because this is where the product's
// rules actually live: which of several subscriptions governs the account, what
// a quota means, and which meters the worker must report. Those are worth
// testing without a network or a database.
//
// **The first live subscription wins.** An account is meant to have one, and
// billing's list is ordered oldest-first; taking the first that grants service
// means a cancelled subscription lying beside a new one cannot shadow it.
func SnapshotFrom(organizationID string, ent *billingclient.Entitlements) *repositories.AccountSnapshot {
	snap := &repositories.AccountSnapshot{OrganizationID: organizationID}
	if ent == nil {
		return snap
	}
	snap.CustomerID = ent.CustomerID

	chosen := pickSubscription(ent.Subscriptions)
	if chosen == nil {
		return snap
	}

	snap.SubscriptionID = chosen.ID
	snap.Status = chosen.Status
	snap.Plan = chosen.Plan
	snap.Entitled = chosen.Entitled
	snap.CancelAtPeriodEnd = chosen.CancelAtPeriodEnd
	snap.PeriodStart, snap.PeriodEnd = chosen.Period.Start, chosen.Period.End

	for _, it := range chosen.Items {
		if snap.Plan == "" {
			snap.Plan = it.Metadata[metadataKeyPlan]
		}
		for k, v := range it.Metadata {
			if !strings.HasPrefix(k, metadataQuotaPref) {
				continue
			}
			limit, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				// A quota that cannot be read is not a quota of zero, which would
				// block every emission on the plan, nor unlimited, which would
				// give the product away. It is dropped and logged: the plan then
				// grants none of that meter, which is the same as the key being
				// absent, and the log is what gets the seed fixed.
				slog.Warn("billing: unreadable quota in price metadata",
					"price_id", it.PriceID, "key", k, "value", v)
				continue
			}
			if snap.Quotas == nil {
				snap.Quotas = map[string]int64{}
			}
			snap.Quotas[strings.TrimPrefix(k, metadataQuotaPref)] = limit
		}
		// Only a metered price has a meter, and only its presence tells the
		// worker to report an emission at all.
		if meter := it.Metadata[metadataKeyMeter]; meter != "" {
			if snap.Meters == nil {
				snap.Meters = map[string]string{}
			}
			snap.Meters[meter] = it.PriceID
		}
		if scope := it.Metadata[metadataKeyMDFEScope]; scope != "" {
			if snap.Features == nil {
				snap.Features = map[string]string{}
			}
			snap.Features[metadataKeyMDFEScope] = scope
		}
	}

	if inv := chosen.OpenInvoice; inv != nil {
		snap.OpenInvoice = &repositories.OpenInvoice{
			ID:          inv.ID,
			TotalCents:  inv.TotalCents,
			DueDate:     inv.DueDate,
			CheckoutURL: inv.CheckoutURL,
		}
	}
	return snap
}

// pickSubscription chooses which subscription governs the account.
//
// The first one that still grants service, and failing that the last one seen —
// so an account whose only subscription is CANCELED still reports that status
// and its open invoice, rather than looking like an account that never
// subscribed. Those are different situations and the screens for them differ.
func pickSubscription(subs []billingclient.EntitlementSubscription) *billingclient.EntitlementSubscription {
	if len(subs) == 0 {
		return nil
	}
	for i := range subs {
		if subs[i].Entitled {
			return &subs[i]
		}
	}
	return &subs[len(subs)-1]
}

// GrantsService reports whether a snapshot lets the account use the product.
//
// This is the one answer, so the blocking middleware, the quota reservation and
// the UI cannot disagree about it. INCOMPLETE is **not** service: it is
// precisely "chose the paid plan and never paid", and treating it as a grace
// period would make the first month free for anyone who abandons the checkout.
func GrantsService(s *repositories.AccountSnapshot) bool {
	if s == nil {
		return false
	}
	return s.Status == StatusActive || s.Status == StatusTrialing
}

// Quota returns the account's limit for a meter and whether the plan grants it
// at all.
func Quota(s *repositories.AccountSnapshot, meter string) (int64, bool) {
	if s == nil {
		return 0, false
	}
	if s.NoCharge {
		return QuotaUnlimited, true
	}
	limit, ok := s.Quotas[meter]
	return limit, ok
}

// workspaceRoles is the role check ScopeFor's callers are authorized by.
// *WorkspaceRoleService satisfies it; a nil one refuses (fail closed).
type workspaceRoles interface {
	Role(ctx context.Context, organizationID, userID string) (string, error)
	OrganizationName(ctx context.Context, organizationID, userID string) string
}

// WithWorkspaceRoles wires who may manage an organization's plan.
func (s *BillingService) WithWorkspaceRoles(r workspaceRoles) *BillingService {
	s.roles = r
	return s
}

// BillingScope is what a /billing route acts on: the organization of the
// selected company, on behalf of one person.
type BillingScope struct {
	CompanyPK      string
	OrganizationID string
	UserID         string
}

// ScopeFor resolves the selected company to its organization.
func (s *BillingService) ScopeFor(ctx context.Context, companyPK, userID string) (*BillingScope, error) {
	organizationID, err := s.OrganizationOf(ctx, companyPK)
	if err != nil {
		return nil, err
	}
	if organizationID == "" {
		return nil, ErrNoOrganization
	}
	return &BillingScope{CompanyPK: companyPK, OrganizationID: organizationID, UserID: repositories.RawUserID(userID)}, nil
}

// manageDenied is the single answer to every refusal to manage, outage
// included: like reach, the cause goes to the log, not to the caller.
const manageDenied = "apenas proprietários e administradores da organização podem gerenciar o plano"

// CanManage reports whether the person is owner or admin of the organization
// in ctech-account. An error is an outage; the caller must read it as "no".
func (s *BillingService) CanManage(ctx context.Context, scope *BillingScope) (bool, error) {
	if s.roles == nil {
		return false, fmt.Errorf("no workspace role check is wired")
	}
	role, err := s.roles.Role(ctx, scope.OrganizationID, scope.UserID)
	if err != nil {
		return false, err
	}
	return MayManageBilling(role), nil
}

func (s *BillingService) requireManager(ctx context.Context, scope *BillingScope) error {
	ok, err := s.CanManage(ctx, scope)
	if err != nil {
		slog.WarnContext(ctx, "billing: could not read the role; refusing", "organization_id", scope.OrganizationID, "error", err)
		return problem.Forbidden(manageDenied)
	}
	if !ok {
		return problem.Forbidden(manageDenied)
	}
	return nil
}

// OrganizationName is the display name for the plan screen, "" when unknown.
func (s *BillingService) OrganizationName(ctx context.Context, scope *BillingScope) string {
	if s.roles == nil {
		return ""
	}
	return s.roles.OrganizationName(ctx, scope.OrganizationID, scope.UserID)
}

// SnapshotOf is the organization's standing as the routes read it.
func (s *BillingService) SnapshotOf(ctx context.Context, scope *BillingScope) (*repositories.AccountSnapshot, error) {
	return s.snapshotFor(ctx, scope.OrganizationID, scope.CompanyPK)
}

// CustomerPayer is who acts when the organization's customer is created: the
// fallback name and the e-mail billing writes to.
type CustomerPayer struct {
	UserID string
	Name   string
	Email  string
}

// GetOrCreateCustomer returns ORG_{organizationID}'s billing customer,
// registering it on first use with the organization's billing company (its
// first linked company: legal name and tax id) and, failing that, the payer's
// name. Billing writes the CUSTOMER_ORG# pointer (its spec § 8).
func (s *BillingService) GetOrCreateCustomer(ctx context.Context, organizationID string, payer CustomerPayer) (string, error) {
	if !s.Enabled() {
		return "", billingclient.ErrNotConfigured
	}
	externalRef := repositories.OrgBillingPK(organizationID)
	if snap, err := s.Snapshot(ctx, organizationID); err == nil && snap.CustomerID != "" {
		return snap.CustomerID, nil
	}
	switch existing, err := s.client.GetEntitlements(ctx, externalRef); {
	case err == nil && existing.CustomerID != "":
		return existing.CustomerID, nil
	case err != nil && !errors.Is(err, billingclient.ErrCustomerNotFound):
		return "", err
	}

	name, taxID, err := s.billingCompany(ctx, organizationID)
	if err != nil {
		return "", err
	}
	if name == "" {
		name = payer.Name
	}
	customer, err := s.client.CreateCustomer(ctx, billingclient.CreateCustomerInput{
		ExternalRef: externalRef,
		Name:        name,
		Email:       payer.Email,
		TaxID:       taxID,
	})
	if err != nil {
		return "", err
	}
	return customer.ID, nil
}

// billingCompany is the organization's first linked company, as this product
// knows it (organization-index, oldest first).
func (s *BillingService) billingCompany(ctx context.Context, organizationID string) (name, taxID string, err error) {
	refs, err := s.orgs.CompaniesOf(ctx, organizationID)
	if err != nil || len(refs) == 0 {
		return "", "", err
	}
	company, err := s.orgs.Company(ctx, refs[0].PK)
	if err != nil || company == nil {
		return "", "", err
	}
	return company.LegalName, company.TaxID, nil
}

// Plans returns the catalogue: the products, their prices and the quota
// metadata, cached.
//
// Published rather than mirrored in a constant here, which the landing page
// still does and this deliberately does not: a plan picker built from a local
// list is a plan picker that offers a price billing will refuse.
func (s *BillingService) Plans(ctx context.Context) ([]billingclient.Product, error) {
	if !s.Enabled() {
		return nil, nil
	}
	if v, ok := CacheGet[[]billingclient.Product](ctx, s.cache, catalogCacheKey); ok {
		return *v, nil
	}
	products, err := s.client.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	products = sellable(products)
	CacheSet(ctx, s.cache, catalogCacheKey, products, catalogCacheTTL)
	return products, nil
}

// sellable drops what no customer may subscribe to: archived prices, products
// billing deactivated, and anything marked internal.
//
// Filtered here rather than in each client, because "the catalogue" and "what
// may be subscribed to" have to be the same list — validatePrices below checks
// against exactly this, so a price that cannot be shown cannot be bought either.
// The previous arrangement filtered in the browser, which meant the API happily
// published the R$ 0 internal price to anyone who called /plans.
func sellable(products []billingclient.Product) []billingclient.Product {
	out := make([]billingclient.Product, 0, len(products))
	for _, p := range products {
		if !p.Active {
			continue
		}
		prices := make([]billingclient.Price, 0, len(p.Prices))
		for _, price := range p.Prices {
			if price.Archived || price.Metadata[metadataKeyVisibility] == visibilityInternal {
				continue
			}
			prices = append(prices, price)
		}
		if len(prices) == 0 {
			continue
		}
		p.Prices = prices
		out = append(out, p)
	}
	return out
}

// validatePrices refuses a set of price ids that must not become a subscription.
//
// Three rules, and each one is a way a subscription goes wrong that billing
// itself would accept:
//
//   - **Unknown price.** Not in the catalogue means archived, internal, or
//     another tenant's. Billing validates ownership; it does not know that this
//     product hides prices.
//   - **Mixed plans.** The usage-based plan is six prices that share
//     `plan: ondemand`; any other combination — Free plus Pro, one plan's
//     document meter beside another's — produces a subscription whose quotas are
//     whatever the merge happened to yield, and SnapshotFrom would report the
//     first item's plan for it.
//   - **Empty.** Handled by the callers, which say so in their own words.
func (s *BillingService) validatePrices(ctx context.Context, priceIDs []string) error {
	products, err := s.Plans(ctx)
	if err != nil {
		return err
	}
	return ValidatePriceSelection(products, priceIDs)
}

// ValidatePriceSelection is validatePrices without the catalogue fetch — pure,
// and separated for the same reason SnapshotFrom is: the rules are worth testing
// without a network.
func ValidatePriceSelection(products []billingclient.Product, priceIDs []string) error {
	type offeredPrice struct {
		price     billingclient.Price
		productID string
	}
	known := map[string]offeredPrice{}
	productPrices := map[string]map[string]bool{}
	for _, product := range products {
		productPrices[product.ID] = map[string]bool{}
		for _, price := range product.Prices {
			known[price.ID] = offeredPrice{price: price, productID: product.ID}
			productPrices[product.ID][price.ID] = true
		}
	}

	plan := ""
	productID := ""
	selected := map[string]bool{}
	for _, id := range priceIDs {
		offered, ok := known[id]
		if !ok {
			// Deliberately the same message for "does not exist" and "exists but
			// is not for sale": a caller probing ids learns nothing from it.
			return problem.BadRequest(fmt.Sprintf("preço indisponível: %s", id))
		}
		if selected[id] {
			return problem.BadRequest(fmt.Sprintf("preço repetido: %s", id))
		}
		selected[id] = true
		price := offered.price
		itemPlan := price.Metadata[metadataKeyPlan]
		if plan == "" {
			plan = itemPlan
			productID = offered.productID
		}
		if itemPlan != plan || offered.productID != productID {
			return problem.BadRequest("todos os preços devem ser do mesmo plano")
		}
	}
	if len(selected) != len(productPrices[productID]) {
		return problem.BadRequest("selecione todos os preços que compõem o plano")
	}
	return nil
}

// Choose puts the organization on a plan for the first time.
//
// It refuses when a subscription already grants service, and that refusal is the
// difference between this and Change: subscribing twice would leave two
// subscriptions billing the same organization, and neither billing nor this
// service has a rule for which of them wins.
func (s *BillingService) Choose(ctx context.Context, scope *BillingScope, accessToken string, priceIDs []string) (*repositories.AccountSnapshot, *billingclient.Invoice, error) {
	if !s.Enabled() {
		return nil, nil, problem.NotImplemented("a cobrança está desativada nesta instalação")
	}
	if len(priceIDs) == 0 {
		return nil, nil, problem.BadRequest("informe ao menos um preço")
	}
	if err := s.requireManager(ctx, scope); err != nil {
		return nil, nil, err
	}
	snap, err := s.SnapshotOf(ctx, scope)
	if err != nil {
		return nil, nil, err
	}
	if snap.SubscriptionID != "" && snap.Status != "" && snap.Status != "CANCELED" {
		return nil, nil, problem.Conflict("esta organização já tem uma assinatura; use a troca de plano")
	}
	if err := s.validatePrices(ctx, priceIDs); err != nil {
		return nil, nil, err
	}
	payer := CustomerPayer{UserID: scope.UserID}
	// users is nil only where no person is acting (tests, the migration
	// command, which calls GetOrCreateCustomer with its own payer).
	if s.users != nil {
		profile, err := s.users.GetUserInfo(ctx, accessToken)
		if err != nil {
			return nil, nil, problem.InternalServer("não foi possível ler o perfil da conta para iniciar a assinatura")
		}
		payer.Name, payer.Email = actorNameFromProfile(profile), profile.Email
	}
	customerID, err := s.GetOrCreateCustomer(ctx, scope.OrganizationID, payer)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.client.CreateSubscription(ctx, customerID, itemsOf(priceIDs),
		subscribeIdempotencyKey(repositories.OrgBillingPK(scope.OrganizationID), priceIDs))
	if err != nil {
		return nil, nil, err
	}
	// Synced rather than derived from `res`: the response says what was created,
	// and the snapshot must say what is true — which for a paid plan is
	// INCOMPLETE with an invoice outstanding, not the plan the user picked.
	fresh, err := s.Sync(ctx, scope.OrganizationID)
	if err != nil {
		return nil, nil, err
	}
	s.markCompaniesLevel(ctx, scope.OrganizationID)
	return fresh, res.Invoice, nil
}

// Change moves the organization to a different plan, billing the prorated
// difference.
func (s *BillingService) Change(ctx context.Context, scope *BillingScope, priceIDs []string) (*repositories.AccountSnapshot, *billingclient.Invoice, error) {
	if !s.Enabled() {
		return nil, nil, problem.NotImplemented("a cobrança está desativada nesta instalação")
	}
	if len(priceIDs) == 0 {
		return nil, nil, problem.BadRequest("informe ao menos um preço")
	}
	if err := s.requireManager(ctx, scope); err != nil {
		return nil, nil, err
	}
	snap, err := s.SnapshotOf(ctx, scope)
	if err != nil {
		return nil, nil, err
	}
	if snap.SubscriptionID == "" {
		return nil, nil, problem.Conflict("esta organização ainda não tem assinatura; escolha um plano primeiro")
	}
	// Same guard as Choose: an organization already on the internal plan must
	// not be able to move a second one onto it, and a downgrade must not smuggle
	// in an archived price.
	if err := s.validatePrices(ctx, priceIDs); err != nil {
		return nil, nil, err
	}
	res, err := s.client.ChangeSubscription(ctx, snap.SubscriptionID, itemsOf(priceIDs), changeIdempotencyKey(snap.SubscriptionID, priceIDs))
	if err != nil {
		return nil, nil, err
	}
	fresh, err := s.Sync(ctx, scope.OrganizationID)
	if err != nil {
		return nil, nil, err
	}
	s.markCompaniesLevel(ctx, scope.OrganizationID)
	return fresh, res.Invoice, nil
}

// Cancel ends the organization's subscription.
func (s *BillingService) Cancel(ctx context.Context, scope *BillingScope, atPeriodEnd bool) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return nil, problem.NotImplemented("a cobrança está desativada nesta instalação")
	}
	if err := s.requireManager(ctx, scope); err != nil {
		return nil, err
	}
	snap, err := s.SnapshotOf(ctx, scope)
	if err != nil {
		return nil, err
	}
	if snap.SubscriptionID == "" {
		return nil, problem.NotFound("esta organização não tem assinatura")
	}
	key := fmt.Sprintf("cancel:%s:%t", snap.SubscriptionID, atPeriodEnd)
	if _, err := s.client.CancelSubscription(ctx, snap.SubscriptionID, atPeriodEnd, key); err != nil {
		return nil, err
	}
	return s.Sync(ctx, scope.OrganizationID)
}

// Invoices lists the organization's invoices, newest month first. Owner/admin
// only: paying is managing.
//
// Billing's M2M invoice list is **tenant-wide** — it has no customer filter —
// so the result is narrowed here by subscription id. Publishing it unfiltered
// would show every CTech customer's invoices to whoever asked.
func (s *BillingService) Invoices(ctx context.Context, scope *BillingScope, year, month int) ([]billingclient.Invoice, error) {
	if !s.Enabled() {
		return nil, nil
	}
	if err := s.requireManager(ctx, scope); err != nil {
		return nil, err
	}
	snap, err := s.SnapshotOf(ctx, scope)
	if err != nil {
		return nil, err
	}
	if snap.SubscriptionID == "" {
		return nil, nil
	}
	all, err := s.client.ListInvoices(ctx, year, month)
	if err != nil {
		return nil, err
	}
	out := make([]billingclient.Invoice, 0, 4)
	for _, inv := range all {
		if inv.SubscriptionID == snap.SubscriptionID {
			out = append(out, inv)
		}
	}
	return out, nil
}

// Meter names. They are the suffix of the `quota_*` keys in the price metadata
// and the `meter` value on a usage-based price, so these constants are the
// contract between the seed file, the quota enforcement and the worker's usage
// report — three places that must agree on one spelling.
const (
	MeterNFe  = "nfe"
	MeterNFCe = "nfce"
	// MeterCTe has a table and a quota but no emission path yet. When CT-e
	// emission is built it must call PrepareUsageReservation(…, MeterCTe, …)
	// like the other four; a Free plan (quota_cte: 0) then refuses it with 402.
	MeterCTe       = "cte"
	MeterMDFe      = "mdfe"
	MeterNFSe      = "nfse"
	MeterCompanies = "companies"
)

// DocumentMeters are the meters counted per issuance. `companies` is absent on
// purpose: it is a current-state count derived by counting rows, not a running
// total — disabling a company gives the slot back, and a counter would have to
// be decremented by every path that removes one.
var DocumentMeters = []string{MeterNFe, MeterNFCe, MeterCTe, MeterMDFe, MeterNFSe}

// MeterForTable maps a document table to the meter its issuance consumes.
//
// The results consumer knows the table the worker wrote to and nothing else
// about the document, and the mapping is spelled out rather than derived from
// the plural so that a table added later has to be declared here to be billed.
// Silence is the safe direction: an undeclared table reports no usage, which
// costs a sale; a guessed one would charge a customer for a meter their plan
// never mentioned.
var MeterForTable = map[string]string{
	"nfes":                  MeterNFe,
	"nfces":                 MeterNFCe,
	"ctes":                  MeterCTe,
	"mdfes":                 MeterMDFe,
	repositories.TableNfses: MeterNFSe,
}

// UsageMeter reports the usage of one meter against its limit.
type UsageMeter struct {
	Used int64 `json:"used"`
	// Limit is -1 for unlimited. A meter the plan does not grant is absent from
	// the map entirely rather than published as zero, so "not included in your
	// plan" and "you have used your last one" stay distinguishable.
	Limit int64 `json:"limit"`
}

// UsageReservation is both the atomic quota write and the immutable billing
// identity attached to the worker command. Terminal settlement must use this
// context, not whatever plan happens to be active minutes later.
type UsageReservation struct {
	Tx             *types.TransactWriteItem
	Exempt         bool
	OrganizationID string
	Period         string
	SubscriptionID string
	PriceID        string
	Meter          string
}

// usagePeriod is the key the counters are filed under.
//
// The subscription's own period start, so a plan anchored on the 10th resets on
// the 10th. Falling back to the calendar month covers the two cases with no
// period: no-charge mode, and an account with no subscription — neither of which
// can issue anything, but both of which have a usage screen to render.
func usagePeriod(s *repositories.AccountSnapshot) string {
	if s != nil && s.PeriodStart != "" {
		return s.PeriodStart
	}
	return time.Now().Format("2006-01")
}

// Reserve claims one unit of a meter for the account that pays for an
// organization, refusing when the plan has no headroom left.
//
// **It is called when the document is requested, not when SEFAZ authorises it**,
// and that ordering is the control rather than a convenience. A limit enforced on
// the authorised count is a limit two concurrent requests both pass — each reads
// three of three used, each issues a fourth. The cost is that a document SEFAZ
// rejects has consumed a slot; giving it back is Refund's job, on the worker's
// terminal-rejection path.
//
// A meter the plan does not mention is refused rather than allowed. That is what
// makes the Free plan's silence about CT-e mean "no CT-e" instead of "unlimited
// CT-e", and it is the safe direction: a wrongly refused emission is a support
// message, a wrongly allowed one is revenue given away.
func (s *BillingService) Reserve(ctx context.Context, orgPK, meter string) error {
	if !s.Enabled() {
		return nil
	}
	snap, err := s.SnapshotForOrg(ctx, orgPK)
	if err != nil {
		return err
	}
	if !GrantsService(snap) {
		return BlockedProblem(snap)
	}
	limit, ok := Quota(snap, meter)
	if !ok {
		return problem.QuotaExceeded(meter, snap.Plan, 0, 0,
			"seu plano não inclui a emissão de "+strings.ToUpper(meter))
	}

	account, err := counterAccount(snap)
	if err != nil {
		return err
	}
	if _, err := s.repo.ReserveUsage(ctx, account, usagePeriod(snap), meter, limit); err != nil {
		if errors.Is(err, repositories.ErrQuotaExceeded) {
			return problem.QuotaExceeded(meter, snap.Plan, limit, limit,
				fmt.Sprintf("o limite de %d %s do plano já foi atingido neste período", limit, strings.ToUpper(meter)))
		}
		return err
	}
	return nil
}

// PrepareUsageReservation validates headroom and returns a conditional update
// that the issuance repository commits atomically with the document and outbox.
func (s *BillingService) PrepareUsageReservation(ctx context.Context, orgPK, meter string, billable bool) (*UsageReservation, error) {
	if !billable || !s.Enabled() {
		return &UsageReservation{Exempt: true}, nil
	}
	snap, err := s.SnapshotForOrg(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if !GrantsService(snap) {
		return nil, BlockedProblem(snap)
	}
	limit, ok := Quota(snap, meter)
	if !ok {
		return nil, problem.QuotaExceeded(meter, snap.Plan, 0, 0,
			"seu plano não inclui a emissão de "+strings.ToUpper(meter))
	}
	account, err := counterAccount(snap)
	if err != nil {
		return nil, err
	}
	period := usagePeriod(snap)
	if limit >= 0 {
		used, getErr := s.repo.GetUsage(ctx, account, period)
		if getErr != nil {
			return nil, getErr
		}
		if used[meter] >= limit {
			return nil, problem.QuotaExceeded(meter, snap.Plan, limit, used[meter],
				fmt.Sprintf("o limite de %d %s do plano já foi atingido neste período", limit, strings.ToUpper(meter)))
		}
	}
	tx := s.repo.BuildReserveUsageTx(account, period, meter, limit)
	return &UsageReservation{
		Tx: &tx, OrganizationID: account, Period: period,
		SubscriptionID: snap.SubscriptionID, PriceID: snap.Meters[meter], Meter: meter,
	}, nil
}

// Refund gives one unit of a meter back.
//
// Best-effort and never fatal to its caller: it runs after something else
// already failed, and turning a rejected document into a failed request as well
// would replace a clear error with a confusing one. A lost refund costs the
// customer one slot out of a monthly allowance, which is recoverable; a lost
// error message is not.
func (s *BillingService) Refund(ctx context.Context, orgPK, meter string) {
	if !s.Enabled() {
		return
	}
	snap, err := s.SnapshotForOrg(ctx, orgPK)
	if err != nil || snap.OrganizationID == "" {
		slog.WarnContext(ctx, "billing: could not refund a quota unit", "org_pk", orgPK, "meter", meter, "error", err)
		return
	}
	if err := s.repo.RefundUsage(ctx, snap.OrganizationID, usagePeriod(snap), meter); err != nil {
		slog.WarnContext(ctx, "billing: refund failed", "org_pk", orgPK, "meter", meter, "error", err)
	}
}

// refundMarker names the once-only claim for a document's refund. The meter is
// part of it because the same access key can never appear under two meters, and
// spelling that out costs nothing while a shared key would silently swallow the
// second refund if it ever did.
func refundMarker(meter, docKey string) string { return "refund:" + meter + ":" + docKey }

// RefundOnce gives a quota unit back exactly once per document. The marker and
// decrement are one transaction, so a transient failure remains retryable.
func (s *BillingService) RefundOnce(ctx context.Context, orgPK, meter, docKey string) error {
	if !s.Enabled() {
		return nil
	}
	snap, err := s.SnapshotForOrg(ctx, orgPK)
	if err != nil {
		return err
	}
	if snap.OrganizationID == "" {
		return nil
	}
	return s.repo.RefundUsageOnce(ctx, snap.OrganizationID, usagePeriod(snap), meter, refundMarker(meter, docKey))
}

// ReportUsage records one authorised document against the account's metered
// price, so a usage-based plan is invoiced for what it actually issued.
//
// A fixed plan reports nothing, and that is not an omission: its price carries
// no meter, the quota counter already recorded the emission for the usage
// screen, and billing would have no per-unit price to charge it against.
//
// docKey is the document's access key (the id_dps for NFS-e), which is what
// makes a redelivered result report the same emission instead of a second one —
// billing answers `duplicate: true`, which the client treats as success.
func (s *BillingService) ReportUsage(ctx context.Context, orgPK, meter, docKey string) error {
	if !s.Enabled() {
		return nil
	}
	snap, err := s.SnapshotForOrg(ctx, orgPK)
	if err != nil {
		return err
	}
	priceID := snap.Meters[meter]
	if priceID == "" || snap.SubscriptionID == "" {
		return nil
	}
	return s.client.ReportUsage(ctx, snap.SubscriptionID, priceID, 1, docKey)
}

// ReportReservedUsage settles against the subscription and price captured when
// the quota was reserved, so a plan change while SEFAZ is processing cannot
// move revenue between plans.
func (s *BillingService) ReportReservedUsage(ctx context.Context, subscriptionID, priceID, docKey string) error {
	if !s.Enabled() || subscriptionID == "" || priceID == "" {
		return nil
	}
	return s.client.ReportUsage(ctx, subscriptionID, priceID, 1, docKey)
}

// RefundReservedUsage refunds the exact account period captured at admission.
func (s *BillingService) RefundReservedUsage(ctx context.Context, accountID, period, meter, docKey string) error {
	if !s.Enabled() || accountID == "" || period == "" || meter == "" {
		return nil
	}
	return s.repo.RefundUsageOnce(ctx, accountID, period, meter, refundMarker(meter, docKey))
}

// Usage reports what an organization has consumed this period against what it
// may. Document meters from the counters; companies counted live.
func (s *BillingService) Usage(ctx context.Context, organizationID, companyPK string) (map[string]UsageMeter, error) {
	snap, err := s.snapshotFor(ctx, organizationID, companyPK)
	if err != nil {
		return nil, err
	}
	out := map[string]UsageMeter{}
	if organizationID == "" {
		return out, nil
	}
	counters, err := s.repo.GetUsage(ctx, organizationID, usagePeriod(snap))
	if err != nil {
		return nil, err
	}
	for _, meter := range DocumentMeters {
		if limit, ok := Quota(snap, meter); ok {
			out[meter] = UsageMeter{Used: counters[meter], Limit: limit}
		}
	}
	if limit, ok := Quota(snap, MeterCompanies); ok {
		used, err := s.companiesUsed(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		out[MeterCompanies] = UsageMeter{Used: used, Limit: limit}
	}
	return out, nil
}

// People are not metered. ctech-dfe counted distinct members across the
// account's organizations and refused an invitation past the plan's limit.
//
// It is gone rather than raised to a large number, because the count was about
// to become wrong rather than merely strict: after the membership unification
// (ctech-billing ADR 0023) organization_users stops being the access record and
// becomes an authorization overlay, so counting it would count who holds a ROLE
// rather than who has access. Somebody invited and not yet given a role would
// not count; somebody whose reach was revoked still would.
//
// Where the quota belongs — the workspace in ctech-account, or reach in the
// product — is an open question, and metering the wrong set while it is open is
// worse than not metering. Companies are still metered; see ReserveCompany.
// The users quota left the catalogue on 2026-10-10 (spec O4).

// MeterLevelCompanies is billing's level meter for an organization's enabled
// companies (billing plans spec § 6; price price_dfe_ondemand_companies_monthly,
// aggregation max). Not the quota meter MeterCompanies: the quota reads the
// plan's `quota_companies`, the level is what the on-demand price bills by peak.
const MeterLevelCompanies = "dfe_companies"

// CompanyReservation is what ReserveCompany hands the fiscal configuration
// write: items for the same transaction. Empty Items: not an enablement.
type CompanyReservation struct {
	Items          []types.TransactWriteItem
	OrganizationID string
	CompanyPK      string
}

// ReserveCompany checks the company quota where a company starts to count, its
// first fiscal configuration (spec O5), and marks the organization's companies
// level dirty in the same transaction. It refuses with 402 when the enabled
// companies already fill the plan; companies already enabled keep emitting
// whatever the count (O6).
//
// The guard serializes this decision with every other enablement of the
// organization: two companies racing for the last slot both read N-1, and only
// the first transaction to bump the guard's version commits. The marker is
// written whatever the plan, so the level is known the day the organization
// moves to on-demand.
func (s *BillingService) ReserveCompany(ctx context.Context, organizationID, companyPK string) (*CompanyReservation, error) {
	if !s.Enabled() {
		return &CompanyReservation{}, nil
	}
	// Already enabled is not a change, organization or not: a company that
	// emits keeps saving its configuration (O6). Only enabling is refused.
	if s.enablement != nil {
		docTypes, err := s.enablement.ConfiguredDocTypes(ctx, companyPK)
		if err != nil {
			return nil, err
		}
		if len(docTypes) > 0 {
			return &CompanyReservation{}, nil
		}
	}
	if organizationID == "" {
		return nil, ErrNoOrganization
	}
	snap, err := s.snapshotFor(ctx, organizationID, companyPK)
	if err != nil {
		return nil, err
	}
	if !GrantsService(snap) {
		return nil, BlockedProblem(snap)
	}
	limit, ok := Quota(snap, MeterCompanies)
	if !ok {
		return nil, problem.QuotaExceeded(MeterCompanies, snap.Plan, 0, 0,
			"o plano da organização não permite habilitar empresas")
	}
	out := &CompanyReservation{OrganizationID: organizationID, CompanyPK: companyPK}
	if limit >= 0 {
		// The guard version is read BEFORE the live count. An enablement that
		// commits after this read bumps the version, so this one's transaction
		// fails; one that committed before it is in the (consistent) count. Read
		// the other way round, an enablement landing between the count and the
		// guard read would pass both checks.
		guard, err := s.repo.BuildQuotaGuardTx(ctx, organizationID, MeterCompanies)
		if err != nil {
			return nil, err
		}
		used, err := s.companiesUsed(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		if used >= limit {
			return nil, problem.QuotaExceeded(MeterCompanies, snap.Plan, limit, used,
				fmt.Sprintf("o plano da organização permite %d empresa(s) habilitada(s) e já há %d", limit, used))
		}
		out.Items = append(out.Items, guard)
	}
	out.Items = append(out.Items, s.repo.BuildMarkLevelDirtyTx(organizationID, MeterLevelCompanies))
	return out, nil
}

// CompanyLevelChangeTx is the marker any path that DISABLES a company must add
// to its own transaction (removing the last fiscal configuration, deleting or
// unlinking a company). No such path exists today; whoever adds one includes
// this, and Task 8's reporter does the rest.
func (s *BillingService) CompanyLevelChangeTx(organizationID string) (types.TransactWriteItem, bool) {
	if !s.Enabled() || organizationID == "" {
		return types.TransactWriteItem{}, false
	}
	return s.repo.BuildMarkLevelDirtyTx(organizationID, MeterLevelCompanies), true
}

// CheckMDFEScope enforces non-numeric MDF-e constraints carried by the plan.
// The Free plan permits MDF-e only with the issuer's own traction vehicle; in
// the fiscal payload a non-nil owner is precisely the third-party `prop` group.
func (s *BillingService) CheckMDFEScope(ctx context.Context, orgPK string, hasThirdPartyOwner bool) error {
	if !s.Enabled() || !hasThirdPartyOwner {
		return nil
	}
	snap, err := s.SnapshotForOrg(ctx, orgPK)
	if err != nil {
		return err
	}
	if snap.Features[metadataKeyMDFEScope] != mdfeScopeOwnFleet {
		return nil
	}
	return problem.QuotaExceeded(MeterMDFe, snap.Plan, 0, 0,
		"seu plano permite MDF-e apenas com veículo de tração da própria empresa",
	)
}

// companiesUsed is how many of the organization's companies count against the
// plan: the enabled ones (ADR 0021). Without an enablement source it counts
// every linked company, the stricter answer.
//
// include names companies known to belong to the organization that the
// organization-index (a GSI, eventually consistent) may not show yet: the
// company enabled a moment ago. The index is membership only; enablement is
// read consistently from the config tables.
func (s *BillingService) companiesUsed(ctx context.Context, organizationID string, include ...string) (int64, error) {
	refs, err := s.orgs.CompaniesOf(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	pks := make([]string, 0, len(refs)+len(include))
	seen := map[string]bool{}
	for _, r := range refs {
		pks = append(pks, r.PK)
		seen[r.PK] = true
	}
	for _, pk := range include {
		if pk != "" && !seen[pk] {
			pks = append(pks, pk)
			seen[pk] = true
		}
	}
	if s.enablement == nil {
		return int64(len(pks)), nil
	}
	enabled, err := countEnabled(ctx, s.enablement, pks)
	if err != nil {
		return 0, err
	}
	return int64(enabled), nil
}

// BlockedProblem turns a snapshot that grants nothing into the 402 that says
// why, naming a machine-readable reason so the UI picks the right screen: the
// first checkout, an overdue bill, or a plan picker.
//
// Exported because the blocking middleware answers the same question about the
// same snapshot, and two copies of this switch would be two answers the day a
// status is added — with the middleware's copy being the one that decides
// whether documents may be issued.
func BlockedProblem(s *repositories.AccountSnapshot) *problem.Problem {
	switch {
	case s == nil || s.SubscriptionID == "":
		return problem.PaymentRequired(problem.ReasonSubscriptionMissing,
			"esta organização não tem um plano ativo; escolha um plano para começar a emitir")
	case s.Status == "PAST_DUE":
		return problem.PaymentRequired(problem.ReasonSubscriptionPastDue,
			"há uma fatura em aberto; regularize o pagamento para voltar a emitir")
	case s.Status == "INCOMPLETE":
		return problem.PaymentRequired(problem.ReasonSubscriptionIncomplete,
			"a assinatura ainda não foi paga; conclua o pagamento para começar a emitir")
	case s.Status == "PAUSED":
		return problem.PaymentRequired(problem.ReasonSubscriptionPaused,
			"a assinatura está pausada")
	default:
		return problem.PaymentRequired(problem.ReasonSubscriptionCanceled,
			"a assinatura foi cancelada; escolha um plano para voltar a emitir")
	}
}

// MarkEventProcessed records a webhook event id, answering false if it was
// already recorded. See the repository for why the marker is written before the
// work rather than after.
func (s *BillingService) MarkEventProcessed(ctx context.Context, eventID string) (bool, error) {
	return s.repo.MarkEventProcessed(ctx, eventID)
}

// OrganizationOf returns the ctech-account organization of a company, from the
// local record (written by the re-key and by LinkService.Link, the same fact
// reach answers). "" for a company whose record has none.
func (s *BillingService) OrganizationOf(ctx context.Context, companyPK string) (string, error) {
	company, err := s.orgs.Company(ctx, companyPK)
	if err != nil {
		return "", err
	}
	if company == nil {
		return "", problem.NotFound("organização não encontrada")
	}
	return company.OrganizationID, nil
}

// SnapshotForOrg returns the standing that governs a company: its
// organization's subscription (spec O1). The parameter is the company pk the
// request carries, as before.
func (s *BillingService) SnapshotForOrg(ctx context.Context, companyPK string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(""), nil
	}
	organizationID, err := s.OrganizationOf(ctx, companyPK)
	if err != nil {
		return nil, err
	}
	return s.snapshotFor(ctx, organizationID, companyPK)
}

// snapshotFor is the organization's snapshot. (The pre-migration fallback to
// the owner's USER_ row was removed when the window closed.)
func (s *BillingService) snapshotFor(ctx context.Context, organizationID, _ string) (*repositories.AccountSnapshot, error) {
	return s.Snapshot(ctx, organizationID)
}

// counterAccount is the account a snapshot's counters live under: always its
// organization.
func counterAccount(s *repositories.AccountSnapshot) (string, error) {
	if s.OrganizationID == "" {
		return "", ErrNoOrganization
	}
	return s.OrganizationID, nil
}

// OrganizationFromRef reads a DF-e organization out of a billing customer's
// external_ref. Anything else (USER_, another product's prefix) is not ours to
// sync.
func OrganizationFromRef(externalRef string) (string, bool) {
	id, ok := strings.CutPrefix(externalRef, repositories.OrgBillingPrefix)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// SyncBySubscription resolves a subscription id back to the DF-e organization
// and re-syncs it. This is what a webhook does.
//
// It resolves through billing rather than through a local index, and the two
// extra reads are the price of never being wrong: an index would be one more
// thing to keep in step, and it would be missing exactly during the race a
// webhook is most likely to lose — the `subscription.created` delivery that
// overtakes this service's own write.
func (s *BillingService) SyncBySubscription(ctx context.Context, subscriptionID string) error {
	if !s.Enabled() || subscriptionID == "" {
		return nil
	}
	sub, err := s.client.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	customer, err := s.client.GetCustomer(ctx, sub.CustomerID)
	if err != nil {
		return err
	}
	organizationID, ok := OrganizationFromRef(customer.ExternalRef)
	if !ok {
		// Pre-migration customers; nothing reads USER_ rows any more. Anything
		// else belongs to another product.
		slog.InfoContext(ctx, "billing: webhook for a customer that is not a DF-e organization; ignored",
			"customer_id", customer.ID, "external_ref", customer.ExternalRef)
		return nil
	}
	_, err = s.Sync(ctx, organizationID)
	return err
}

// itemsOf turns price ids into subscription items. Quantity is left unset:
// billing reads anything below 1 as 1, and a quantity is only meaningful for a
// fixed price bought several times, which no DF-e plan does.
func itemsOf(priceIDs []string) []billingclient.Item {
	out := make([]billingclient.Item, 0, len(priceIDs))
	for _, id := range priceIDs {
		out = append(out, billingclient.Item{PriceID: id})
	}
	return out
}

// subscribeIdempotencyKey and changeIdempotencyKey make a retry of one intent
// return the first answer instead of a second subscription or a second prorated
// invoice.
//
// They are derived from the request rather than random, because the request is
// what a double-click repeats. Including the prices means "assinar o Pro" twice
// is one subscription while "assinar o Pro" then "assinar o Ilimitado" are two
// distinct intents, which is the distinction a random key would lose and a
// constant key would collapse.
func subscribeIdempotencyKey(userID string, priceIDs []string) string {
	return "sub:" + userID + ":" + strings.Join(priceIDs, ",")
}

func changeIdempotencyKey(subscriptionID string, priceIDs []string) string {
	return "chg:" + subscriptionID + ":" + strings.Join(priceIDs, ",")
}
