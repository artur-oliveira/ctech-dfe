package v1

import (
	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Outbound webhooks (docs/specs/2026-11-15-webhooks.md). Every route needs the
// Dfe-Organization-Pk header (the company, resolved to its organization) and the
// ctech-account role owner or admin, checked in WebhookService (403 otherwise,
// outage included). The signing secret is in the answer of the POST that creates
// the endpoint and of rotate-secret, and nowhere else.

const (
	webhookDeliveriesDefaultLimit = 20
	webhookDeliveriesMaxLimit     = 50
)

type webhookBody struct {
	URL        string   `json:"url"`
	Events     []string `json:"events"`
	CompanyPKs []string `json:"company_pks"`
}

// webhookCreated is the endpoint plus its secret, shown once. The embedded
// endpoint hides the secret (`json:"-"`); this field shadows it.
type webhookCreated struct {
	repositories.WebhookEndpoint
	Secret string `json:"secret"`
}

// RegisterWebhooks mounts /v1.0/webhooks/*.
func RegisterWebhooks(router fiber.Router, svc *services.WebhookService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/webhooks", authMw, perm.RequireMember())

	g.Get("", func(c fiber.Ctx) error {
		list, err := svc.List(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{
			"data":          list.Endpoints,
			"company_pks":   list.CompanyPKs,
			"enabled":       svc.Enabled(),
			"max_endpoints": repositories.MaxWebhookEndpoints,
			"event_types":   services.WebhookEventTypes,
		})
	})

	g.Post("", func(c fiber.Ctx) error {
		var body webhookBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		e, err := svc.Register(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), services.WebhookRegistration{
			URL: body.URL, Events: body.Events, CompanyPKs: body.CompanyPKs,
		})
		if err != nil {
			return sendProblem(c, err)
		}
		secret := e.Secret
		return c.Status(fiber.StatusCreated).JSON(webhookCreated{WebhookEndpoint: *e, Secret: secret})
	})

	g.Delete("/:id", func(c fiber.Ctx) error {
		if err := svc.Delete(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	g.Post("/:id/rotate-secret", func(c fiber.Ctx) error {
		secret, err := svc.RotateSecret(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"secret": secret})
	})

	g.Post("/:id/enable", func(c fiber.Ctx) error {
		if err := svc.Enable(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	g.Post("/:id/ping", func(c fiber.Ctx) error {
		if err := svc.Ping(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	g.Get("/:id/deliveries", func(c fiber.Ctx) error {
		limit := min(max(fiber.Query(c, "limit", webhookDeliveriesDefaultLimit), 1), webhookDeliveriesMaxLimit)
		cursor := c.Query("cursor")
		page, next, err := svc.Deliveries(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id"), decodeCursor(cursor), limit)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"data": page, "next_cursor": buildNextCursor(next, cursor)})
	})

	g.Get("/:id/deliveries/:event_id", func(c fiber.Ctx) error {
		d, attempts, err := svc.Delivery(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id"), c.Params("event_id"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"delivery": d, "attempts": attempts})
	})

	g.Post("/:id/deliveries/:event_id/replay", func(c fiber.Ctx) error {
		if err := svc.Replay(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), c.Params("id"), c.Params("event_id")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
