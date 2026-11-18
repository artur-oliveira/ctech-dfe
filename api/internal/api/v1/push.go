package v1

import (
	"regexp"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Web Push (docs/specs/2026-11-17-web-push.md). Notifications are personal: the routes need a member of the company in
// the Dfe-Organization-Pk header (any role, even viewer), and a scoped token needs dfe:notifications:read or :write.
// RequireMember already refuses a scoped token that is not the first-party session, so an API key never reaches them.
// /push/config only needs the scope: the screen asks it before it knows whether to show itself.
const (
	permPushRead  = "get.notifications"
	permPushWrite = "update.notifications"
)

// pushDeviceID is the id RegisterPush accepts in a path: PushSubscriptionID, 16 hex characters.
var pushDeviceID = regexp.MustCompile(`^[0-9a-f]{16}$`)

type pushSubscriptionBody struct {
	Endpoint string `json:"endpoint" validate:"required,max=2048"`
	Keys     struct {
		P256dh string `json:"p256dh" validate:"required,max=128"`
		Auth   string `json:"auth" validate:"required,max=64"`
	} `json:"keys" validate:"required"`
}

type pushPreferencesBody struct {
	Events []string `json:"events" validate:"max=8,dive,max=40"`
}

// RegisterPush mounts /v1.0/push/*.
func RegisterPush(router fiber.Router, svc *services.PushService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/push", authMw)

	g.Get("/config", middleware.RequireScope(permPushRead), func(c fiber.Ctx) error {
		return c.JSON(svc.Config())
	})

	m := g.Group("", perm.RequireMember())

	m.Put("/subscriptions", middleware.RequireScope(permPushWrite), func(c fiber.Ctx) error {
		var body pushSubscriptionBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		device, err := svc.Subscribe(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), services.PushSubscriptionInput{
			Endpoint: body.Endpoint, P256dh: body.Keys.P256dh, Auth: body.Keys.Auth, UserAgent: c.Get(fiber.HeaderUserAgent),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(device)
	})

	m.Get("/subscriptions", middleware.RequireScope(permPushRead), func(c fiber.Ctx) error {
		devices, err := svc.Devices(c.Context(), middleware.GetUserID(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"data": devices, "max_subscriptions": repositories.MaxPushSubscriptionsPerUser})
	})

	m.Delete("/subscriptions/:id", middleware.RequireScope(permPushWrite), func(c fiber.Ctx) error {
		id := c.Params("id")
		if !pushDeviceID.MatchString(id) {
			return sendProblem(c, problem.BadRequest("identificador de dispositivo inválido"))
		}
		if err := svc.Unsubscribe(c.Context(), middleware.GetUserID(c), id); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	m.Get("/preferences", middleware.RequireScope(permPushRead), func(c fiber.Ctx) error {
		events, err := svc.Preferences(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"events": events})
	})

	m.Put("/preferences", middleware.RequireScope(permPushWrite), func(c fiber.Ctx) error {
		var body pushPreferencesBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		events, err := svc.SetPreferences(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c), body.Events)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"events": events})
	})

	m.Post("/test", middleware.RequireScope(permPushWrite), func(c fiber.Ctx) error {
		if err := svc.Test(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c)); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusAccepted)
	})
}
