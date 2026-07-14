package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterAuth mounts GET /auth/me, GET /auth/roles, and POST /auth/terms-addendum/accept.
func RegisterAuth(router fiber.Router, userSvc *services.UserService, _ *services.OrganizationService, roleRepo *repositories.RoleRepository, authMw fiber.Handler) {
	g := router.Group("/auth", authMw)

	g.Get("/me", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		accessToken := currentAccessToken(c)

		// First login: provision the local row (org memberships only — no
		// profile fields, see UserRepository.CreateMinimal) before reading it.
		if _, err := userSvc.GetOrCreate(c.Context(), userID); err != nil {
			return sendProblem(c, err)
		}

		result, err := userSvc.GetMeData(c.Context(), userID, accessToken)
		if err != nil {
			return sendProblem(c, err)
		}
		result["user_id"] = userID
		return c.JSON(result)
	})

	g.Post("/terms-addendum/accept", func(c fiber.Ctx) error {
		userID := middleware.GetUserID(c)
		if err := userSvc.AcceptTermsAddendum(c.Context(), userID); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	g.Get("/roles", func(c fiber.Ctx) error {
		roles, err := roleRepo.ListAll(c.Context())
		if err != nil {
			return sendProblem(c, err)
		}
		items, err := unmarshalList(roles)
		if err != nil {
			return sendProblem(c, err)
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]any{
				"name":        item["name"],
				"description": item["description"],
			})
		}
		return c.JSON(out)
	})
}
