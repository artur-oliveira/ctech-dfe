package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterVehicles mounts /vehicles routes.
func RegisterVehicles(router fiber.Router, svc *services.VehicleService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/vehicles", authMw)

	// GET /vehicles
	g.Get("", perm.Require("list.organization_vehicles"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		opts := repositories.VehicleListOpts{
			PlatePrefix: c.Query("plate"),
			Sort:        c.Query("sort", "asc"),
			Limit:       intQuery(c, "limit", 50),
			StartKey:    decodeCursor(cursor),
		}
		var res *repositories.QueryResult
		var err error
		if role := c.Query("role"); role != "" {
			res, err = svc.ListByRole(c.Context(), middleware.GetOrgPK(c), role, opts)
		} else {
			res, err = svc.List(c.Context(), middleware.GetOrgPK(c), opts)
		}
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// POST /vehicles
	g.Post("", perm.Require("create.organization_vehicles"), func(c fiber.Ctx) error {
		var dto VehicleCreateBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Create(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /vehicles/:sk
	g.Get("/:sk", perm.Require("get.organization_vehicles"), func(c fiber.Ctx) error {
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// GET /vehicles/:sk/requirements
	g.Get("/:sk/requirements", perm.Require("get.organization_vehicles"), func(c fiber.Ctx) error {
		docType := c.Query("doc_type")
		role := c.Query("role")
		validDocTypes := map[string]bool{services.DocTypeMdfe: true, services.DocTypeNfe: true, services.DocTypeCteOS: true}
		validRoles := map[string]bool{services.VehicleRoleTractor: true, services.VehicleRoleTrailer: true}
		if !validDocTypes[docType] {
			return sendProblem(c, problem.BadRequest("doc_type inválido: "+docType))
		}
		if !validRoles[role] {
			return sendProblem(c, problem.BadRequest("role inválido: "+role))
		}
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"missing": services.Missing(item, docType, role)})
	})

	// PUT /vehicles/:sk
	g.Put("/:sk", perm.Require("update.organization_vehicles"), func(c fiber.Ctx) error {
		var dto VehicleUpdateBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Update(c.Context(), middleware.GetOrgPK(c), c.Params("sk"), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// DELETE /vehicles/:sk
	g.Delete("/:sk", perm.Require("delete.organization_vehicles"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		if err := svc.Delete(c.Context(), middleware.GetOrgPK(c), c.Params("sk"), userID, userName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
