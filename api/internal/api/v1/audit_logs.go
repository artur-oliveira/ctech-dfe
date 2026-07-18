package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterAuditLogs mounts /audit-logs — OWNER/ADMIN-only, org-scoped via the
// active-org header like every other top-level resource.
func RegisterAuditLogs(router fiber.Router, svc *services.AuditLogService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/audit-logs", authMw, perm.RequireOwnerOrAdmin())

	// GET /audit-logs
	g.Get("", func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.List(c.Context(), middleware.GetOrgPK(c), services.AuditLogQueryOpts{
			ResourceType: c.Query("resource_type"),
			ResourceID:   c.Query("resource_id"),
			UserID:       c.Query("user_id"),
			Limit:        intQuery(c, "limit", 50),
			StartKey:     decodeCursor(cursor),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})
}
