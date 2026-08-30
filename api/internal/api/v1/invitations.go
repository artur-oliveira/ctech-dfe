package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterInvitations mounts the token-addressed invitation endpoints. These sit
// outside the org-scoped RBAC group — the invitee is not yet a member — so they
// are guarded only by authentication.
//
// CREATING an invitation is retired in favour of ctech-account's (see
// members.go). Reading, accepting and declining one are NOT: a token minted
// before the retirement is a promise somebody made, and breaking it would strand
// whoever holds the link with no way to join and nobody to ask. They go when the
// last pending invitation has expired, which is a decision with a date on it
// rather than a deploy.
func RegisterInvitations(router fiber.Router, invSvc *services.InvitationService, userSvc *services.UserService, authMw fiber.Handler) {
	g := router.Group("/invitations", authMw)

	// GET /invitations/:token — non-consuming preview
	g.Get("/:token", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		preview, err := invSvc.Preview(c.Context(), c.Params("token"), userID)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(preview)
	})

	// POST /invitations/:token/accept — join the org
	g.Post("/:token/accept", func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		m, err := invSvc.Accept(c.Context(), c.Params("token"), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		// Refresh the user's cached /auth/me + org list so the new org shows immediately.
		userSvc.InvalidateCache(c.Context(), userID)
		return c.JSON(fiber.Map{"org_pk": m.OrgPK, "role": m.Role})
	})

	// POST /invitations/:token/decline — refuse the invitation
	g.Post("/:token/decline", func(c fiber.Ctx) error {
		if err := invSvc.Decline(c.Context(), c.Params("token")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
