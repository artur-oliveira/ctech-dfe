package v1

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// The organization's billing surface (docs/specs/2026-10-10-organization-subscription.md § 2).
//
// Every /billing/subscription* and /billing/invoices route requires the
// Dfe-Organization-Pk header: the subscription addressed is that of the
// selected company's ctech-account organization. Reading the plan and usage is
// open to anybody with access to the company; choosing, changing, cancelling
// and listing invoices require the ctech-account role owner or admin, checked
// in BillingService (403 otherwise, outage included).

// planChoiceBody selects a set of prices. It is a list because a usage-based
// plan meters several document types and each is its own price — one
// subscription, several items.
type planChoiceBody struct {
	PriceIDs []string `json:"price_ids" validate:"required,min=1,dive,required"`
}

type cancelBody struct {
	// AtPeriodEnd distinguishes the two cancellations. They are different
	// operations, not a shade of one: at the period end the customer keeps what
	// they already paid for, and immediately gives it up.
	AtPeriodEnd bool `json:"at_period_end"`
}

// RegisterBilling mounts /v1.0/billing/* and the webhook.
func RegisterBilling(router fiber.Router, app *fiber.App, svc *services.BillingService, webhookSecret string, authMw fiber.Handler, perm *middleware.PermChecker) {
	registerBillingWebhook(app, svc, webhookSecret)

	billing := router.Group("/billing", authMw)

	// GET /billing/plans — the catalogue, from billing rather than a local list.
	billing.Get("/plans", func(c fiber.Ctx) error {
		products, err := svc.Plans(c.Context())
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"data": products, "billing_enabled": svc.Enabled()})
	})

	// GET /billing/subscription — the organization's standing and what it has used.
	billing.Get("/subscription", perm.RequireMember(), func(c fiber.Ctx) error {
		scope, err := scopeOf(c, svc)
		if err != nil {
			return sendProblem(c, err)
		}
		snap, err := svc.SnapshotOf(c.Context(), scope)
		if err != nil {
			return sendProblem(c, err)
		}
		usage, err := svc.Usage(c.Context(), scope.OrganizationID, scope.CompanyPK)
		if err != nil {
			return sendProblem(c, err)
		}
		// An outage reading the role shows the plan read-only; it never fails
		// the read.
		manageable, _ := svc.CanManage(c.Context(), scope)
		view := organizationSubscriptionView(snap, svc.OrganizationName(c.Context(), scope), manageable)
		view["usage"] = usage
		return c.JSON(view)
	})

	// POST /billing/subscription — choose a plan for the first time.
	billing.Post("/subscription", perm.RequireMember(), func(c fiber.Ctx) error {
		var body planChoiceBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		scope, err := scopeOf(c, svc)
		if err != nil {
			return sendProblem(c, err)
		}
		snap, invoice, err := svc.Choose(c.Context(), scope, currentAccessToken(c), body.PriceIDs)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(checkoutView(snap, invoice))
	})

	// POST /billing/subscription/change — upgrade or downgrade.
	billing.Post("/subscription/change", perm.RequireMember(), func(c fiber.Ctx) error {
		var body planChoiceBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		scope, err := scopeOf(c, svc)
		if err != nil {
			return sendProblem(c, err)
		}
		snap, invoice, err := svc.Change(c.Context(), scope, body.PriceIDs)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(checkoutView(snap, invoice))
	})

	// POST /billing/subscription/cancel
	billing.Post("/subscription/cancel", perm.RequireMember(), func(c fiber.Ctx) error {
		var body cancelBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		scope, err := scopeOf(c, svc)
		if err != nil {
			return sendProblem(c, err)
		}
		snap, err := svc.Cancel(c.Context(), scope, body.AtPeriodEnd)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(subscriptionView(snap))
	})

	// GET /billing/invoices — the organization's invoices for a month (owner/admin).
	billing.Get("/invoices", perm.RequireMember(), func(c fiber.Ctx) error {
		now := time.Now()
		year := fiber.Query(c, "year", now.Year())
		month := fiber.Query(c, "month", int(now.Month()))
		if month < 1 || month > 12 {
			return sendProblem(c, problem.BadRequest("mês inválido"))
		}
		scope, err := scopeOf(c, svc)
		if err != nil {
			return sendProblem(c, err)
		}
		invoices, err := svc.Invoices(c.Context(), scope, year, month)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"data": invoices})
	})
}

// RegisterOrganizationPlan mounts the read-only, organization-scoped view.
//
// It is registered from the organizations group rather than here because it is
// org-scoped and must sit behind the same tenant resolution every other
// `/organizations/:org_pk` route does.
func RegisterOrganizationPlan(scoped fiber.Router, svc *services.BillingService, perm *middleware.PermChecker) {
	// GET /organizations/:org_pk/plan — the plan that governs this company: it
	// answers for the ctech-account organization of that company.
	//
	// OWNER and ADMIN (local roles) only, and read-only for both. Managing the
	// plan happens on /billing/subscription*, gated by the ctech-account role.
	scoped.Get("/plan", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		snap, err := svc.SnapshotForOrg(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		view := subscriptionView(snap)
		// The organization's view says which account pays, never how to change
		// it: an ADMIN seeing a "mudar plano" button they cannot use is worse
		// than not seeing one.
		view["manageable"] = false
		return c.JSON(view)
	})
}

// organizationSubscriptionView is subscriptionView plus who the plan belongs
// to and whether the caller may act on it. `manageable` is the UI's only input
// for showing the buttons that spend money.
func organizationSubscriptionView(s *repositories.AccountSnapshot, organizationName string, manageable bool) fiber.Map {
	out := subscriptionView(s)
	out["organization"] = map[string]string{"id": s.OrganizationID, "name": organizationName}
	out["manageable"] = manageable
	// Paying is managing: the open invoice and its checkout link are for the
	// organization's owners and admins only.
	if !manageable {
		delete(out, "open_invoice")
	}
	return out
}

// scopeOf resolves the selected company to the organization a billing route
// acts on.
func scopeOf(c fiber.Ctx, svc *services.BillingService) (*services.BillingScope, error) {
	return svc.ScopeFor(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c))
}

// subscriptionView is the organization's standing on the wire.
//
// It publishes `grants_service` alongside `status` rather than only the status,
// because "may I issue a document right now" is a question the UI must not
// answer by reimplementing the status list — that list differs from billing's
// own `entitled` by decision, and a second copy of it in TypeScript is a copy
// that drifts.
func subscriptionView(s *repositories.AccountSnapshot) fiber.Map {
	out := fiber.Map{
		"has_subscription":     s.SubscriptionID != "",
		"status":               s.Status,
		"plan":                 s.Plan,
		"grants_service":       services.GrantsService(s),
		"cancel_at_period_end": s.CancelAtPeriodEnd,
		"period_start":         s.PeriodStart,
		"period_end":           s.PeriodEnd,
		"quotas":               s.Quotas,
		"no_charge":            s.NoCharge,
	}
	if s.OpenInvoice != nil {
		out["open_invoice"] = s.OpenInvoice
	}
	return out
}

// checkoutView is what a mutation returns: the new standing, and the invoice
// when the operation produced one.
//
// The invoice is absent on a free plan, on the first period of a usage-based
// plan, and on a downgrade. All three are ordinary, so the caller branches on
// its presence — a client that assumes a bill will send somebody to a checkout
// that does not exist.
func checkoutView(s *repositories.AccountSnapshot, invoice *billingclient.Invoice) fiber.Map {
	out := subscriptionView(s)
	if invoice != nil {
		out["invoice"] = invoice
	}
	return out
}

// registerBillingWebhook mounts billing's notify-back.
//
// It is on the app rather than the /v1.0 group and carries no auth middleware,
// because billing holds no user token — it authenticates with an HMAC over the
// body it sent. The /v1/internal prefix keeps it off the public listener at the
// balancer, which is defence in depth and not the control: a path that is merely
// hard to reach is a path somebody eventually reaches.
//
// **An unset secret means the route is not mounted at all.** A signature check
// that cannot run is not a signature check, and the route it guards accepts
// subscription state changes from outside — so the absence must be a 404 that
// somebody notices, never an endpoint that trusts whatever arrives.
func registerBillingWebhook(app *fiber.App, svc *services.BillingService, secret string) {
	if secret == "" {
		slog.Warn("billing webhook route not mounted: BILLING_WEBHOOK_SECRET is unset")
		return
	}
	app.Post("/v1/internal/webhooks/billing", func(c fiber.Ctx) error {
		body := c.Body()
		if err := billingclient.VerifyWebhook(
			secret,
			c.Get(billingclient.HeaderTimestamp),
			c.Get(billingclient.HeaderSignature),
			body,
			time.Now(),
		); err != nil {
			slog.WarnContext(c.Context(), "billing webhook rejected",
				"event_id", c.Get(billingclient.HeaderEventID),
				"type", c.Get(billingclient.HeaderEventType))
			return sendProblem(c, problem.Unauthorized("assinatura inválida"))
		}

		eventID := c.Get(billingclient.HeaderEventID)
		if eventID == "" {
			return sendProblem(c, problem.BadRequest("entrega sem identificador de evento"))
		}

		// The durable marker is written only after the idempotent refresh. That
		// keeps a transient billing/DynamoDB failure retryable. Concurrent copies
		// may both refresh the same whole snapshot; the conditional marker then
		// records exactly one completion and both deliveries answer successfully.
		var event billingclient.WebhookEvent
		if err := c.Bind().Body(&event); err != nil {
			return sendProblem(c, problem.BadRequest("corpo inválido"))
		}
		// The body is read for one thing: which subscription to go and ask
		// billing about. Nothing in it is treated as true — the snapshot is
		// always rebuilt from a fresh read, so a forged body that somehow passed
		// the signature still cannot state a status.
		subscriptionID := event.Data.SubscriptionID
		if subscriptionID == "" && event.Data.Object == "subscription" {
			subscriptionID = event.Data.ID
		}
		if subscriptionID == "" {
			// A one-off invoice belongs to no subscription and governs no
			// account. Acknowledged rather than retried forever.
			slog.InfoContext(c.Context(), "billing webhook names no subscription",
				"event_id", eventID, "type", event.Type)
			return c.JSON(fiber.Map{"received": true})
		}

		if err := svc.SyncBySubscription(c.Context(), subscriptionID); err != nil {
			// A failure here must be a non-2xx: billing retries on a backoff for
			// about two days, and that retry is the only thing standing between a
			// transient outage and a snapshot that stays wrong until the account
			// is touched again.
			slog.ErrorContext(c.Context(), "billing webhook sync failed",
				"event_id", eventID, "subscription_id", subscriptionID, "error", err)
			return sendProblem(c, problem.InternalServer("falha ao sincronizar a assinatura"))
		}
		// Sync is idempotent. Mark only after it succeeds so a transient
		// dependency failure remains retryable; concurrent copies may safely
		// repeat the refresh before exactly one records completion.
		fresh, err := svc.MarkEventProcessed(c.Context(), eventID)
		if err != nil {
			return sendProblem(c, err)
		}
		if !fresh {
			return c.JSON(fiber.Map{"received": true, "duplicate": true})
		}
		return c.JSON(fiber.Map{"received": true})
	})
}
