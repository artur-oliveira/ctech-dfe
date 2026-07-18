package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// crudHandlers bundles the per-resource service calls behind the router.
// The Get/Delete tails are uniform across resources, so the factory wires
// them directly; List/Create/Update keep a closure because each resource
// binds a different body type, applies different validation (e.g. RequirePJFields),
// and (for vehicles) has role-based list/extra routes. Keeping that logic
// in the closure — not the factory — is the surgical, low-risk extract.
type crudHandlers struct {
	listPerm   string
	createPerm string
	getPerm    string
	updatePerm string
	deletePerm string

	// param is the detail path param (e.g. "cpf_cnpj", "product_id", "sk").
	param string

	list   func(c fiber.Ctx, orgPK string, o crudListOpts) (*repositories.QueryResult, error)
	create func(c fiber.Ctx, orgPK, userID, userName string) (map[string]types.AttributeValue, error)
	get    func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error)
	update func(c fiber.Ctx, orgPK, id, userID, userName string) (map[string]types.AttributeValue, error)
	del    func(c fiber.Ctx, orgPK, id, userID, userName string) error
}

// crudListOpts is the subset of list options common to every resource:
// pagination + cursor. Per-resource filters are applied by the List closure.
type crudListOpts struct {
	Cursor   string
	Limit    int
	Sort     string
	StartKey map[string]types.AttributeValue
}

// parseCRUDListOpts reads the shared pagination/cursor query params.
func parseCRUDListOpts(c fiber.Ctx) crudListOpts {
	cursor := c.Query("cursor")
	return crudListOpts{
		Cursor:   cursor,
		Limit:    intQuery(c, "limit", 50),
		Sort:     c.Query("sort", "asc"),
		StartKey: decodeCursor(cursor),
	}
}

// mountCRUD wires the standard GET-list / POST / GET-detail / PUT / DELETE
// routes for one resource, collapsing the repeated group+perm+sendProblem
// boilerplate. Extra resource-specific routes (e.g. vehicles' /requirements)
// are mounted by the caller after this returns.
func mountCRUD(router fiber.Router, path string, authMw fiber.Handler, perm *middleware.PermChecker, userSvc *services.UserService, h crudHandlers) {
	g := router.Group(path, authMw)

	g.Get("", perm.Require(h.listPerm), func(c fiber.Ctx) error {
		o := parseCRUDListOpts(c)
		res, err := h.list(c, middleware.GetOrgPK(c), o)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, o.Cursor)
	})

	g.Post("", perm.Require(h.createPerm), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		item, err := h.create(c, middleware.GetOrgPK(c), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendCreated(c, item)
	})

	g.Get("/:"+h.param, perm.Require(h.getPerm), func(c fiber.Ctx) error {
		item, err := h.get(c, middleware.GetOrgPK(c), c.Params(h.param))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	g.Put("/:"+h.param, perm.Require(h.updatePerm), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		item, err := h.update(c, middleware.GetOrgPK(c), c.Params(h.param), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	g.Delete("/:"+h.param, perm.Require(h.deletePerm), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		if err := h.del(c, middleware.GetOrgPK(c), c.Params(h.param), userID, userName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}
