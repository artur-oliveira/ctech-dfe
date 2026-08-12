package v1

import (
	"fmt"
	"strconv"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// LookupByKeyBody is the payload for POST /distributions/nfe/key.
type LookupByKeyBody struct {
	AccessKey string `json:"access_key" validate:"required,dfe_access_key"`
}

func RegisterDistributions(router fiber.Router, svc *services.DistributionService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/distributions", authMw)

	// GET /distributions/{doc_type}/history
	g.Get("/:doc_type/history", perm.RequireDynamic("list.%s_distributions", "doc_type"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		result, err := svc.ListDistributions(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), repositories.DistributionListOpts{
			Limit:    intQuery(c, "limit", 50),
			StartKey: decodeCursor(cursor),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, result, cursor)
	})

	// POST /distributions/{doc_type}/sync
	g.Post("/:doc_type/sync", perm.RequireDynamic("create.%s_distributions", "doc_type"), func(c fiber.Ctx) error {
		result, err := svc.EnqueueSync(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusAccepted).JSON(result)
	})

	// GET /distributions/{doc_type}/history/{nsu}/xml
	g.Get("/:doc_type/history/:nsu/xml", perm.RequireDynamic("get.%s_distributions", "doc_type"), func(c fiber.Ctx) error {
		nsu, err := strconv.Atoi(c.Params("nsu"))
		if err != nil {
			return sendProblem(c, problem.BadRequest("nsu inválido"))
		}
		xmlBytes, err := svc.GetDistributionXML(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), nsu)
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="NSU_%015d.xml"`, nsu))
		return c.Send(xmlBytes)
	})

	// GET /distributions/{doc_type}/nsu/{nsu}
	g.Get("/:doc_type/nsu/:nsu", perm.RequireDynamic("get.%s_distributions", "doc_type"), func(c fiber.Ctx) error {
		nsu, err := strconv.Atoi(c.Params("nsu"))
		if err != nil {
			return sendProblem(c, problem.BadRequest("nsu inválido"))
		}
		result, err := svc.LookupByNSU(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), nsu)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(result)
	})

	// POST /distributions/nfe/key — async consChNFe (NF-e only; see spec §E "Fora de escopo")
	g.Post("/nfe/key", perm.Require("create.nfe_distributions"), func(c fiber.Ctx) error {
		var body LookupByKeyBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		result, err := svc.EnqueueLookupByKey(c.Context(), middleware.GetOrgPK(c), body.AccessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusAccepted).JSON(result)
	})
}
