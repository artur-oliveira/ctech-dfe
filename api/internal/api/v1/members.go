package v1

import (
	"github.com/gofiber/fiber/v3"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// MemberRoleBody is the payload for changing a member's role or creating an
// invitation. Role must be one the caller may grant (ADMIN/USER/VIEWER).
type MemberRoleBody struct {
	Role string `json:"role" validate:"required,oneof=ADMIN USER VIEWER"`
}

// registerMemberRoutes mounts member-management and invitation endpoints under
// the already-tenant-scoped /organizations/:org_pk group. Visibility is
// role-gated: OWNER/ADMIN can view and invite; only OWNER can remove members or
// change roles.
func registerMemberRoutes(scoped fiber.Router, h OrgHandlers, perm *middleware.PermChecker) {
	// GET /members — list org members
	scoped.Get("/members", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		members, err := h.MemberSvc.ListByOrg(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(members)
	})

	// DELETE /members/:user_id — remove a member (OWNER only, never self)
	scoped.Delete("/members/:user_id", perm.RequireOwner(), func(c fiber.Ctx) error {
		orgPK := middleware.GetOrgPK(c)
		target := c.Params("user_id")
		if repositories.RawUserID(target) == repositories.RawUserID(middleware.GetUserID(c)) {
			return sendProblem(c, problem.BadRequest("você não pode remover a si mesmo"))
		}
		deletedBy, deletedByName := resolveActor(c, h.UserSvc)
		if err := h.MemberSvc.Remove(c.Context(), orgPK, target, deletedBy, deletedByName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// PUT /members/:user_id/role — change a member's role (OWNER only)
	scoped.Put("/members/:user_id/role", perm.RequireOwner(), func(c fiber.Ctx) error {
		var body MemberRoleBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		if err := h.MemberSvc.ChangeRole(c.Context(), middleware.GetOrgPK(c), c.Params("user_id"), body.Role); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// GET /invitations — list pending invitations
	scoped.Get("/invitations", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		items, err := h.InvSvc.ListPending(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(items)
	})

	// POST /invitations — create an invitation; the raw token is returned only here
	scoped.Post("/invitations", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		var body MemberRoleBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, h.UserSvc)
		token, item, err := h.InvSvc.Create(c.Context(), middleware.GetOrgPK(c), body.Role, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		item["token"] = token
		return c.Status(fiber.StatusCreated).JSON(item)
	})

	// DELETE /invitations/:id — revoke a pending invitation (id is its pk)
	scoped.Delete("/invitations/:id", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		if err := h.InvSvc.Revoke(c.Context(), middleware.GetOrgPK(c), c.Params("id")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
