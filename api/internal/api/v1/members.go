package v1

import (
	"github.com/gofiber/fiber/v3"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// MemberRoleBody is the payload for changing a member's role.
//
// It used to serve invitation creation too; that route is retired in favour of
// ctech-account's (ctech-billing ADR 0023).
//
// Which roles are allowed is **not** listed here. It used to be, as
// `oneof=ADMIN USER VIEWER`, and that made this tag a third copy of a list that
// also lives in the invitation service and in member management — three places
// to keep in agreement, and the day one of them gained OWNER the other two would
// have kept quiet. The single answer is repositories.GrantableRoles, checked by
// the services, which also produce a far better message than a `oneof` failure:
// asking for OWNER is answered with "use ADMIN, que tem os mesmos acessos".
type MemberRoleBody struct {
	Role string `json:"role" validate:"required"`
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

	// POST /invitations — retired.
	//
	// Invitations are ctech-account's (ctech-billing ADR 0023). Two invitation
	// flows for one workspace is two e-mails, two tokens and two ways to be
	// half-invited — and only the platform's can say WHICH COMPANIES the
	// invitation grants, which is the case this product is for: an accountant
	// inviting a junior to five of forty CNPJs.
	//
	// It answers rather than disappearing. Somebody with this call in a script
	// deserves to be told where it moved, and a 404 would send them looking for
	// a typo in their own integration.
	scoped.Post("/invitations", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		return sendProblem(c, problem.Retired(
			"os convites agora são criados na sua conta CTech, em Organizações — lá é possível escolher as empresas que a pessoa poderá usar"))
	})

	// DELETE /invitations/:id — revoke a pending invitation (id is its pk)
	scoped.Delete("/invitations/:id", perm.RequireOwnerOrAdmin(), func(c fiber.Ctx) error {
		if err := h.InvSvc.Revoke(c.Context(), middleware.GetOrgPK(c), c.Params("id")); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
