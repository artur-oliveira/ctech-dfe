package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	ctesvc "gopkg.aoctech.app/dfe/api/internal/services/ctes"

	"github.com/gofiber/fiber/v3"
)

// RegisterCTes mounts the /ctes routes (CT-e rodoviário a partir de NF-e).
func RegisterCTes(router fiber.Router, svc *ctesvc.CteService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/ctes", authMw)

	// POST /ctes/derive — lê as notas e devolve o que o CT-e herda delas (não grava nada)
	g.Post("/derive", perm.Require("create.ctes"), func(c fiber.Ctx) error {
		var body ctesvc.CteDeriveBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		res, err := svc.Derive(c.Context(), middleware.GetOrgPK(c), body.NFeKeys)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(res)
	})

	// POST /ctes — emite (202 + registro pending; o worker autoriza)
	g.Post("", perm.Require("create.ctes"), func(c fiber.Ctx) error {
		var body ctesvc.CteEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(cte)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusAccepted).JSON(m)
	})

	// GET /ctes
	g.Get("", perm.Require("list.ctes"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListCTes(c.Context(), middleware.GetOrgPK(c), repositories.NFeListOpts{
			Limit:    intQuery(c, "limit", 50),
			StartKey: decodeCursor(cursor),
			Incoming: ptrIntQuery(c, "incoming"),
			Number:   ptrIntQuery(c, "number"),
			Year:     ptrIntQuery(c, "year"),
			Month:    ptrIntQuery(c, "month"),
			Day:      ptrIntQuery(c, "day"),
			Sort:     c.Query("sort", "asc"),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /ctes/:access_key
	g.Get("/:access_key", perm.Require("get.ctes"), func(c fiber.Ctx) error {
		cte, err := svc.GetCTe(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if cte == nil {
			return sendProblem(c, ctesvc.ErrCTeNotFound)
		}
		return sendItem(c, cte)
	})
}
