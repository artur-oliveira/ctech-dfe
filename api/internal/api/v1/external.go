package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterExternal mounts /external routes.
func RegisterExternal(router fiber.Router, svc *services.ExternalService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/external", authMw)

	// GET /external/lookup-organizations?cpf_cnpj=...&uf=...
	// Permission: get.organization_persons (matches Python _RES = "organization_persons")
	g.Get("/lookup-organizations", perm.Require("get.organization_persons"), func(c fiber.Ctx) error {
		orgPK := middleware.GetOrgPK(c)
		cpfCNPJ := c.Query("cpf_cnpj")
		uf := c.Query("uf")

		if cpfCNPJ == "" {
			return sendProblem(c, problem.BadRequest("query parameter 'cpf_cnpj' is required"))
		}
		if uf == "" {
			return sendProblem(c, problem.BadRequest("query parameter 'uf' is required"))
		}

		result, err := svc.LookupOrganization(c.Context(), orgPK, cpfCNPJ, uf)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(result)
	})
}
