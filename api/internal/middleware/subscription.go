package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// The subscription gate (D2): an organization whose paying account has no live
// subscription may not issue documents or write registry data.
//
// **It is default-deny by shape.** Rather than being added to each write route —
// which is a list somebody eventually forgets to extend — it is mounted once on
// the /v1.0 group and refuses every mutating request that is not explicitly
// exempt. A route added tomorrow is gated without anybody remembering to gate it,
// and making it exempt takes an edit here, where the whole set is visible.
//
// It runs **before** RBAC, which is the opposite of what it looks like it should
// do. RBAC is what resolves the organization from the header, so a gate after it
// would have to be per-route again; and the leak is small — a member learns their
// organization's subscription lapsed, which is what the screen is about to tell
// them anyway.

// mutatingMethods are the ones the gate considers. Reads are never blocked: the
// customer paid for the documents they already have, fiscal custody is a
// five-year legal obligation, and withholding somebody's own XML over an unpaid
// invoice is not a lever this product will pull.
func isMutating(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	default:
		return false
	}
}

// exemptPrefixes are whole surfaces the gate never touches.
//
// The first two are the way out of the block: an account that cannot manage its
// subscription because its subscription lapsed has no way to pay, and one that
// cannot accept the terms cannot use the product it just paid for.
var exemptPrefixes = []string{
	"/v1.0/billing/",
	"/v1.0/auth/",
	// Accepting or declining an invitation acts on the invitee's own account,
	// not on the organization's plan — and the invitee may not even be a member
	// yet, so there is no organization to resolve.
	"/v1.0/invitations/",
}

// exemptSuffixes are mutations on **documents that already exist**.
//
// Every one of them is an obligation rather than a new sale, and the carve-outs
// were decided deliberately (plan § "Carve-outs do D2"):
//
//   - Cancelling an NF-e has a 24-hour legal deadline. Blocking it over an
//     unpaid invoice makes an overdue bill into a fiscal problem the customer
//     cannot fix at any price.
//   - A carta de correção, an MDF-e closure, a driver or document included in a
//     trip in progress: all of them finish something already issued.
//   - Manifestação and distribuição are how a company answers documents **other
//     people** issued against its CNPJ. Blocking them punishes the customer for
//     somebody else's actions.
//
// `substitute` is deliberately absent: substituting issues a **new** document,
// and it would otherwise be the way around the whole gate.
var exemptSuffixes = []string{
	"/cancel",
	"/correction-letter",
	"/manifestation",
	"/close",
	"/include-condutor",
	"/include-dfe",
	"/events",
	"/sync",
	"/import-xml",
	"/nfe/key",
	// A preview computes and returns; it writes nothing and issues nothing.
	"/cargo-preview",
}

// RequireActiveSubscription blocks issuance and registry writes for an
// organization whose paying account has no live subscription.
func RequireActiveSubscription(billing *services.BillingService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// No-charge mode: the gate is not merely permissive, it does not run at
		// all, so a dev environment pays no DynamoDB read per request.
		if billing == nil || !billing.Enabled() {
			return c.Next()
		}
		if !isMutating(c.Method()) || isExempt(c.Path()) {
			return c.Next()
		}
		orgPK := organizationOf(c)
		if orgPK == "" {
			// No organization in scope: creating one, or an account-level route.
			// Creating an organization has its own quota check, which needs the
			// caller's account rather than an organization that does not exist yet.
			return c.Next()
		}

		snap, err := billing.SnapshotForOrg(c.Context(), orgPK)
		if err != nil {
			// The gate must not turn a billing outage into an outage of the whole
			// product. The snapshot is durable and read from DynamoDB, so an error
			// here is DynamoDB being unavailable — in which case the request is
			// about to fail anyway, and failing it here would mislabel the cause.
			return c.Next()
		}
		if services.GrantsService(snap) {
			return c.Next()
		}
		return services.BlockedProblem(snap).Send(c)
	}
}

func isExempt(path string) bool {
	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, suffix := range exemptSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

// organizationOf resolves the organization the request acts on, from the header
// or the path — the same two places the permission checker looks.
//
// It does not validate membership: that is RBAC's job, and it runs next. This
// only needs to know *which* organization's plan to read, and a caller naming an
// organization they do not belong to is refused a moment later with a 403.
func organizationOf(c fiber.Ctx) string {
	raw := c.Get(OrgHeader)
	if raw == "" {
		raw = c.Params(OrgPKKey)
	}
	if raw == "" {
		return ""
	}
	orgPK, err := repositories.ParseOrgPK(raw)
	if err != nil {
		return ""
	}
	return orgPK
}
