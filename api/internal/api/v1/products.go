package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterProducts mounts /products routes under a tenant-scoped group.
func RegisterProducts(router fiber.Router, svc *services.ProductService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/products", authMw)

	// GET /products
	g.Get("", perm.Require("list.organization_products"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.List(c.Context(), middleware.GetOrgPK(c), repositories.ProductListOpts{
			DescriptionPrefix: c.Query("description"),
			CodePrefix:        c.Query("code"),
			OrderBy:           c.Query("order_by"),
			Sort:              c.Query("sort", "asc"),
			Limit:             intQuery(c, "limit", 50),
			StartKey:          decodeCursor(cursor),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// POST /products
	g.Post("", perm.Require("create.organization_products"), func(c fiber.Ctx) error {
		av, p := bindAVValidated[ProductBody](c)
		if p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Create(c.Context(), middleware.GetOrgPK(c), av, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /products/:product_id
	g.Get("/:product_id", perm.Require("get.organization_products"), func(c fiber.Ctx) error {
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("product_id"))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// PUT /products/:product_id
	g.Put("/:product_id", perm.Require("update.organization_products"), func(c fiber.Ctx) error {
		var dto ProductBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Update(c.Context(), middleware.GetOrgPK(c), c.Params("product_id"), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// DELETE /products/:product_id
	g.Delete("/:product_id", perm.Require("delete.organization_products"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		if err := svc.Delete(c.Context(), middleware.GetOrgPK(c), c.Params("product_id"), userID, userName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
