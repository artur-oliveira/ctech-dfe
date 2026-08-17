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
	metadataKeyPlan   = "plan"
	metadataKeyMeter  = "meter"
	metadataQuotaPref = "quota_"
	// metadataKeyVisibility marks a price nobody may subscribe to through this
	// API. See visibilityInternal.
	metadataKeyVisibility = "visibility"
)

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

func accountBillingCacheKey(userID string) string {
	return fmt.Sprintf("billing:%s", repositories.RawUserID(userID))
}

const catalogCacheKey = "billing:catalog"

// BillingService is the DF-e's view of ctech-billing.
type BillingService struct {
	repo    *repositories.AccountBillingRepository
	client  *billingclient.Client
	users   *UserService
	members *MembershipService
	orgs    *OrganizationService
	cache   cache.Backend
}

func NewBillingService(
	repo *repositories.AccountBillingRepository,
	client *billingclient.Client,
	users *UserService,
	members *MembershipService,
	orgs *OrganizationService,
	c cache.Backend,
) *BillingService {
	return &BillingService{repo: repo, client: client, users: users, members: members, orgs: orgs, cache: c}
}

// Enabled reports whether this deployment charges for anything.
func (s *BillingService) Enabled() bool { return s.client.Enabled() }

// noChargeSnapshot is what every account looks like when billing is switched
// off: unlimited, and honest about why.
func noChargeSnapshot(userID string) *repositories.AccountSnapshot {
	return &repositories.AccountSnapshot{
		UserID:   repositories.RawUserID(userID),
		Status:   StatusActive,
		Plan:     PlanUnlimited,
		Entitled: true,
		NoCharge: true,
	}
}

// Snapshot returns the account's billing standing, cache-first.
//
// It never calls billing. A miss reads the durable row, and an account with no
// row yet gets an empty snapshot rather than a synchronous sync — because this
// is called from the issuance path, and an account with no subscription is a
// question DynamoDB can answer in a millisecond while billing would take a
// network round trip to say the same thing.
func (s *BillingService) Snapshot(ctx context.Context, userID string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(userID), nil
	}
	key := accountBillingCacheKey(userID)
	if v, ok := CacheGet[repositories.AccountSnapshot](ctx, s.cache, key); ok {
		return v, nil
	}
	snap, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		snap = &repositories.AccountSnapshot{UserID: repositories.RawUserID(userID)}
	}
	CacheSet(ctx, s.cache, key, *snap, snapshotCacheTTL)
	return snap, nil
}

// Sync re-reads billing and rewrites the snapshot. This is the only writer.
//
// A customer billing does not know is not an error: it is an account that has
// never chosen a plan, and it produces an empty snapshot so the caller sees
// "sem assinatura" rather than an outage.
func (s *BillingService) Sync(ctx context.Context, userID string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(userID), nil
	}
	raw := repositories.RawUserID(userID)
	ent, err := s.client.GetEntitlements(ctx, repositories.AccountBillingPK(raw))
	switch {
	case err == nil:
	case errors.Is(err, billingclient.ErrCustomerNotFound):
		ent = &billingclient.Entitlements{}
	default:
		return nil, err
	}

	snap := SnapshotFrom(raw, ent)
	if err := s.repo.Put(ctx, snap); err != nil {
		return nil, err
	}
	s.Invalidate(ctx, raw)
	return snap, nil
}

// Invalidate drops the cached snapshot. Called by the webhook and by every
// mutation here.
func (s *BillingService) Invalidate(ctx context.Context, userID string) {
	_ = s.cache.Delete(ctx, accountBillingCacheKey(repositories.RawUserID(userID)))
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
func SnapshotFrom(userID string, ent *billingclient.Entitlements) *repositories.AccountSnapshot {
	snap := &repositories.AccountSnapshot{UserID: repositories.RawUserID(userID)}
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

// GetOrCreateCustomer returns the account's billing customer, registering it on
// first use.
//
// The profile comes from ctech-account through the caller's own access token,
// never from a local copy — the same discipline GetMeData follows, and for the
// same reason: ctech-account owns the name and the e-mail, and a stale copy sent
// to billing ends up printed on an invoice.
func (s *BillingService) GetOrCreateCustomer(ctx context.Context, userID, accessToken string) (string, error) {
	if !s.Enabled() {
		return "", billingclient.ErrNotConfigured
	}
	raw := repositories.RawUserID(userID)
	externalRef := repositories.AccountBillingPK(raw)

	// The snapshot is checked first only to save a round trip; billing remains
	// the authority, and a snapshot without a customer id falls through.
	if snap, err := s.Snapshot(ctx, raw); err == nil && snap.CustomerID != "" {
		return snap.CustomerID, nil
	}
	switch existing, err := s.client.GetEntitlements(ctx, externalRef); {
	case err == nil && existing.CustomerID != "":
		return existing.CustomerID, nil
	case err != nil && !errors.Is(err, billingclient.ErrCustomerNotFound):
		return "", err
	}

	profile, err := s.users.GetUserInfo(ctx, accessToken)
	if err != nil {
		// Without a name and an e-mail the customer would be created blank, and a
		// customer is not editable afterwards through any route this service has.
		// Failing here is recoverable — the user retries — while a blank customer
		// is not.
		return "", problem.InternalServer("não foi possível ler o perfil da conta para iniciar a assinatura")
	}
	name := actorNameFromProfile(profile)
	if name == "" {
		name = raw
	}
	customer, err := s.client.CreateCustomer(ctx, billingclient.CreateCustomerInput{
		ExternalRef: externalRef,
		UserID:      raw,
		Name:        name,
		Email:       profile.Email,
	})
	if err != nil {
		return "", err
	}
	return customer.ID, nil
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
	known := map[string]billingclient.Price{}
	for _, product := range products {
		for _, price := range product.Prices {
			known[price.ID] = price
		}
	}

	plan := ""
	for _, id := range priceIDs {
		price, ok := known[id]
		if !ok {
			// Deliberately the same message for "does not exist" and "exists but
			// is not for sale": a caller probing ids learns nothing from it.
			return problem.BadRequest(fmt.Sprintf("preço indisponível: %s", id))
		}
		itemPlan := price.Metadata[metadataKeyPlan]
		if plan == "" {
			plan = itemPlan
			continue
		}
		if itemPlan != plan {
			return problem.BadRequest("todos os preços devem ser do mesmo plano")
		}
	}
	return nil
}

// Choose puts the account on a plan for the first time.
//
// It refuses when a subscription already grants service, and that refusal is the
// difference between this and Change: subscribing twice would leave two
// subscriptions billing the same account, and neither billing nor this service
// has a rule for which of them wins.
func (s *BillingService) Choose(ctx context.Context, userID, accessToken string, priceIDs []string) (*repositories.AccountSnapshot, *billingclient.Invoice, error) {
	if !s.Enabled() {
		return nil, nil, problem.NotImplemented("a cobrança está desativada nesta instalação")
	}
	if len(priceIDs) == 0 {
		return nil, nil, problem.BadRequest("informe ao menos um preço")
	}
	raw := repositories.RawUserID(userID)

	snap, err := s.Snapshot(ctx, raw)
	if err != nil {
		return nil, nil, err
	}
	if snap.SubscriptionID != "" && snap.Status != "" && snap.Status != "CANCELED" {
		return nil, nil, problem.Conflict("esta conta já tem uma assinatura; use a troca de plano")
	}
	if err := s.validatePrices(ctx, priceIDs); err != nil {
		return nil, nil, err
	}

	customerID, err := s.GetOrCreateCustomer(ctx, raw, accessToken)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.client.CreateSubscription(ctx, customerID, itemsOf(priceIDs), subscribeIdempotencyKey(raw, priceIDs))
	if err != nil {
		return nil, nil, err
	}
	// Synced rather than derived from `res`: the response says what was created,
	// and the snapshot must say what is true — which for a paid plan is
	// INCOMPLETE with an invoice outstanding, not the plan the user picked.
	fresh, err := s.Sync(ctx, raw)
	if err != nil {
		return nil, nil, err
	}
	return fresh, res.Invoice, nil
}

// Change moves the account to a different plan, billing the prorated difference.
func (s *BillingService) Change(ctx context.Context, userID string, priceIDs []string) (*repositories.AccountSnapshot, *billingclient.Invoice, error) {
	if !s.Enabled() {
		return nil, nil, problem.NotImplemented("a cobrança está desativada nesta instalação")
	}
	if len(priceIDs) == 0 {
		return nil, nil, problem.BadRequest("informe ao menos um preço")
	}
	raw := repositories.RawUserID(userID)
	snap, err := s.Snapshot(ctx, raw)
	if err != nil {
		return nil, nil, err
	}
	if snap.SubscriptionID == "" {
		return nil, nil, problem.Conflict("esta conta ainda não tem assinatura; escolha um plano primeiro")
	}
	// Same guard as Choose: an account already on the internal plan must not be
	// able to move a second account onto it, and a downgrade must not smuggle in
	// an archived price.
	if err := s.validatePrices(ctx, priceIDs); err != nil {
		return nil, nil, err
	}
	res, err := s.client.ChangeSubscription(ctx, snap.SubscriptionID, itemsOf(priceIDs), changeIdempotencyKey(snap.SubscriptionID, priceIDs))
	if err != nil {
		return nil, nil, err
	}
	fresh, err := s.Sync(ctx, raw)
	if err != nil {
		return nil, nil, err
	}
	return fresh, res.Invoice, nil
}

// Cancel ends the account's subscription.
func (s *BillingService) Cancel(ctx context.Context, userID string, atPeriodEnd bool) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return nil, problem.NotImplemented("a cobrança está desativada nesta instalação")
	}
	raw := repositories.RawUserID(userID)
	snap, err := s.Snapshot(ctx, raw)
	if err != nil {
		return nil, err
	}
	if snap.SubscriptionID == "" {
		return nil, problem.NotFound("esta conta não tem assinatura")
	}
	key := fmt.Sprintf("cancel:%s:%t", snap.SubscriptionID, atPeriodEnd)
	if _, err := s.client.CancelSubscription(ctx, snap.SubscriptionID, atPeriodEnd, key); err != nil {
		return nil, err
	}
	return s.Sync(ctx, raw)
}

// Invoices lists the account's invoices, newest month first.
//
// Billing's M2M invoice list is **tenant-wide** — it has no customer filter —
// so the result is narrowed here by subscription id. Publishing it unfiltered
// would show every CTech customer's invoices to whoever asked.
func (s *BillingService) Invoices(ctx context.Context, userID string, year, month int) ([]billingclient.Invoice, error) {
	if !s.Enabled() {
		return nil, nil
	}
	raw := repositories.RawUserID(userID)
	snap, err := s.Snapshot(ctx, raw)
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
	MeterNFe       = "nfe"
	MeterNFCe      = "nfce"
	MeterCTe       = "cte"
	MeterMDFe      = "mdfe"
	MeterNFSe      = "nfse"
	MeterCompanies = "companies"
	MeterUsers     = "users"
)

// DocumentMeters are the meters counted per issuance. `companies` and `users`
// are absent on purpose: those are current-state counts derived by counting
// rows, not running totals — deleting an organization must give the slot back,
// and a counter would have to be decremented by every path that removes one.
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

	if _, err := s.repo.ReserveUsage(ctx, snap.UserID, usagePeriod(snap), meter, limit); err != nil {
		if errors.Is(err, repositories.ErrQuotaExceeded) {
			return problem.QuotaExceeded(meter, snap.Plan, limit, limit,
				fmt.Sprintf("o limite de %d %s do plano já foi atingido neste período", limit, strings.ToUpper(meter)))
		}
		return err
	}
	return nil
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
	if err != nil || snap.UserID == "" {
		slog.WarnContext(ctx, "billing: could not refund a quota unit", "org_pk", orgPK, "meter", meter, "error", err)
		return
	}
	if err := s.repo.RefundUsage(ctx, snap.UserID, usagePeriod(snap), meter); err != nil {
		slog.WarnContext(ctx, "billing: refund failed", "org_pk", orgPK, "meter", meter, "error", err)
	}
}

// refundMarker names the once-only claim for a document's refund. The meter is
// part of it because the same access key can never appear under two meters, and
// spelling that out costs nothing while a shared key would silently swallow the
// second refund if it ever did.
func refundMarker(meter, docKey string) string { return "refund:" + meter + ":" + docKey }

// RefundOnce gives a quota unit back exactly once per document.
//
// The results queue redelivers, and a refund is not idempotent the way a usage
// report is: billing dedupes a report by its event key, but this counter would
// simply go down twice. The marker is claimed *before* the refund, so a failure
// in between loses the refund rather than repeating it — one slot the customer
// has to ask for back is recoverable, a slot handed out on every redelivery is
// free issuance.
func (s *BillingService) RefundOnce(ctx context.Context, orgPK, meter, docKey string) {
	if !s.Enabled() {
		return
	}
	fresh, err := s.repo.MarkEventProcessed(ctx, refundMarker(meter, docKey))
	if err != nil {
		slog.WarnContext(ctx, "billing: could not claim a refund", "org_pk", orgPK, "meter", meter, "error", err)
		return
	}
	if !fresh {
		return
	}
	s.Refund(ctx, orgPK, meter)
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

// Usage reports what the account has consumed this period against what it may.
//
// Document meters come from the counters; `companies` and `users` are counted
// live, because both are current state rather than a running total — deleting an
// organization gives the slot back, and a counter would have to be decremented
// by every path that removes one.
func (s *BillingService) Usage(ctx context.Context, userID string) (map[string]UsageMeter, error) {
	raw := repositories.RawUserID(userID)
	snap, err := s.Snapshot(ctx, raw)
	if err != nil {
		return nil, err
	}
	counters, err := s.repo.GetUsage(ctx, raw, usagePeriod(snap))
	if err != nil {
		return nil, err
	}

	out := map[string]UsageMeter{}
	for _, meter := range DocumentMeters {
		limit, ok := Quota(snap, meter)
		if !ok {
			continue
		}
		out[meter] = UsageMeter{Used: counters[meter], Limit: limit}
	}

	if limit, ok := Quota(snap, MeterCompanies); ok {
		owned, err := s.ownedOrganizations(ctx, raw)
		if err != nil {
			return nil, err
		}
		out[MeterCompanies] = UsageMeter{Used: int64(len(owned)), Limit: limit}
	}
	if limit, ok := Quota(snap, MeterUsers); ok {
		people, err := s.distinctMembers(ctx, raw)
		if err != nil {
			return nil, err
		}
		out[MeterUsers] = UsageMeter{Used: int64(len(people)), Limit: limit}
	}
	return out, nil
}

// CheckCompanyQuota refuses creating one more organization than the plan allows.
func (s *BillingService) CheckCompanyQuota(ctx context.Context, userID string) error {
	if !s.Enabled() {
		return nil
	}
	raw := repositories.RawUserID(userID)
	snap, err := s.Snapshot(ctx, raw)
	if err != nil {
		return err
	}
	if !GrantsService(snap) {
		return BlockedProblem(snap)
	}
	limit, ok := Quota(snap, MeterCompanies)
	if !ok {
		return problem.QuotaExceeded(MeterCompanies, snap.Plan, 0, 0,
			"seu plano não permite cadastrar empresas")
	}
	if limit < 0 {
		return nil
	}
	owned, err := s.ownedOrganizations(ctx, raw)
	if err != nil {
		return err
	}
	if int64(len(owned)) >= limit {
		return problem.QuotaExceeded(MeterCompanies, snap.Plan, limit, int64(len(owned)),
			fmt.Sprintf("seu plano permite %d empresa(s) e você já tem %d", limit, len(owned)))
	}
	return nil
}

// CheckUserQuota refuses admitting one more person than the plan allows.
//
// It counts **distinct people across the account's organizations**, not members
// of one of them (D5). Somebody who helps with two of the same customer's
// companies is one person, and billing them twice is the kind of arithmetic a
// customer checks. The owner counts.
//
// `candidate` is the person about to be admitted; already being a member of
// another of the account's organizations means the count does not grow, so the
// invitation is free.
func (s *BillingService) CheckUserQuota(ctx context.Context, orgPK, candidate string) error {
	if !s.Enabled() {
		return nil
	}
	owner, err := s.OwnerOf(ctx, orgPK)
	if err != nil || owner == "" {
		return err
	}
	snap, err := s.Snapshot(ctx, owner)
	if err != nil {
		return err
	}
	limit, ok := Quota(snap, MeterUsers)
	if !ok || limit < 0 {
		// Absent here means the plan predates the quota rather than "no users
		// allowed" — an organization with no members cannot be operated at all,
		// so refusing every invitation would be a worse reading than allowing.
		return nil
	}
	people, err := s.distinctMembers(ctx, owner)
	if err != nil {
		return err
	}
	if candidate != "" && people[repositories.RawUserID(candidate)] {
		return nil
	}
	if int64(len(people)) >= limit {
		return problem.QuotaExceeded(MeterUsers, snap.Plan, limit, int64(len(people)),
			fmt.Sprintf("seu plano permite %d usuário(s) e você já tem %d", limit, len(people)))
	}
	return nil
}

// ownedOrganizations lists the organizations an account pays for.
//
// It reads the account's memberships and keeps the ones where it is the OWNER,
// rather than querying organizations by `owner_user_id` — there is no index on
// that field, and adding one to answer a question the membership GSI already
// answers would be an index for nothing. The two agree because they are written
// in the same transaction.
func (s *BillingService) ownedOrganizations(ctx context.Context, userID string) ([]string, error) {
	memberships, err := s.members.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(memberships))
	for _, m := range memberships {
		if m.Role == repositories.RoleOwner && m.OrgPK != "" {
			out = append(out, m.OrgPK)
		}
	}
	return out, nil
}

// distinctMembers is the set of people across every organization the account
// owns, the owner included.
func (s *BillingService) distinctMembers(ctx context.Context, userID string) (map[string]bool, error) {
	orgs, err := s.ownedOrganizations(ctx, userID)
	if err != nil {
		return nil, err
	}
	people := map[string]bool{repositories.RawUserID(userID): true}
	for _, orgPK := range orgs {
		members, err := s.members.ListByOrg(ctx, orgPK)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			people[repositories.RawUserID(m.UserID)] = true
		}
	}
	return people, nil
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

// SnapshotForOrg returns the billing standing that governs an organization: the
// snapshot of the account that owns it.
//
// This is the resolution the whole design turns on. Members do not subscribe —
// the organization points at one account through `owner_user_id`, and that
// account's plan decides what everyone in the organization may do, whatever
// their role. Resolving the other way, from the requesting user to their own
// subscription, would block every member who is not the owner.
func (s *BillingService) SnapshotForOrg(ctx context.Context, orgPK string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(""), nil
	}
	owner, err := s.OwnerOf(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if owner == "" {
		return &repositories.AccountSnapshot{}, nil
	}
	return s.Snapshot(ctx, owner)
}

// OwnerOf returns the account that pays for an organization.
//
// The field first, and a scan of the members as the fallback. The fallback is
// the migration: organizations created before `owner_user_id` existed have none,
// and rather than a one-shot backfill script that somebody has to remember to
// run against each environment, the first read repairs the row. It is the same
// read-fallback self-heal the membership table used through its own migration.
//
// The repair is best-effort — a failed write is logged and the answer returned
// anyway, because the caller asked who the owner is and that is now known.
func (s *BillingService) OwnerOf(ctx context.Context, orgPK string) (string, error) {
	org, err := s.orgs.Get(ctx, orgPK)
	if err != nil {
		return "", err
	}
	if org == nil {
		return "", problem.NotFound("organização não encontrada")
	}
	if v, ok := org[repositories.AttrOwnerUserID].(*types.AttributeValueMemberS); ok && v.Value != "" {
		return v.Value, nil
	}

	members, err := s.members.ListByOrg(ctx, orgPK)
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.Role != repositories.RoleOwner {
			continue
		}
		owner := repositories.RawUserID(m.UserID)
		if err := s.orgs.SetOwnerUserID(ctx, orgPK, owner); err != nil {
			slog.WarnContext(ctx, "billing: could not backfill owner_user_id",
				"org_pk", orgPK, "owner", owner, "error", err)
		}
		return owner, nil
	}
	// An organization with no OWNER at all. It cannot happen through any write
	// path this service has — creation writes one and it cannot be removed — so
	// it means a hand-edited row, and answering "nobody pays for this" is more
	// useful than an error that hides which organization it was.
	slog.ErrorContext(ctx, "billing: organization has no owner", "org_pk", orgPK)
	return "", nil
}

// SyncBySubscription resolves a subscription id back to the DF-e account and
// re-syncs it. This is what a webhook does.
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
	userID := repositories.RawUserID(customer.ExternalRef)
	if userID == "" || userID == customer.ExternalRef {
		// The external ref is this service's own `USER_{sub}`. Anything else
		// belongs to another product sharing the tenant, and syncing it here would
		// write a DF-e snapshot for an account that has none.
		slog.InfoContext(ctx, "billing: webhook for a customer that is not a DF-e account",
			"customer_id", customer.ID, "external_ref", customer.ExternalRef)
		return nil
	}
	_, err = s.Sync(ctx, userID)
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
