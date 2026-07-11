package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterPersons mounts /persons routes.
func RegisterPersons(router fiber.Router, svc *services.PersonService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/persons", authMw)

	// GET /persons
	g.Get("", perm.Require("list.organization_persons"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.List(c.Context(), middleware.GetOrgPK(c), repositories.PersonListOpts{
			NamePrefix: c.Query("name"),
			Sort:       c.Query("sort", "asc"),
			Limit:      intQuery(c, "limit", 50),
			StartKey:   decodeCursor(cursor),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// POST /persons
	g.Post("", perm.Require("create.organization_persons"), func(c fiber.Ctx) error {
		var dto PersonCreateBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Create(c.Context(), middleware.GetOrgPK(c), dto.CpfOrCnpj, body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /persons/:cpf_cnpj
	g.Get("/:cpf_cnpj", perm.Require("get.organization_persons"), func(c fiber.Ctx) error {
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("cpf_cnpj"))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// PUT /persons/:cpf_cnpj
	g.Put("/:cpf_cnpj", perm.Require("update.organization_persons"), func(c fiber.Ctx) error {
		var dto PersonUpdateBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Update(c.Context(), middleware.GetOrgPK(c), c.Params("cpf_cnpj"), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// DELETE /persons/:cpf_cnpj
	g.Delete("/:cpf_cnpj", perm.Require("delete.organization_persons"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		if err := svc.Delete(c.Context(), middleware.GetOrgPK(c), c.Params("cpf_cnpj"), userID, userName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
