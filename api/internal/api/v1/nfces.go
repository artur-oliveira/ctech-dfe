package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"

	"github.com/gofiber/fiber/v3"
)

// RegisterNFCes mounts all /nfces routes (NFC-e, modelo 65).
func RegisterNFCes(router fiber.Router, svc *nfesvc.NfceService, ext *services.ExternalService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/nfces", authMw)

	// POST /nfces — emit a new NFC-e
	g.Post("", perm.Require("create.nfces"), func(c fiber.Ctx) error {
		var body nfesvc.NfceEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfce, err := svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(nfce)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /nfces
	g.Get("", perm.Require("list.nfces"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNFCes(c.Context(), middleware.GetOrgPK(c), repositories.NFeListOpts{
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

	// GET /nfces/:access_key
	g.Get("/:access_key", perm.Require("get.nfces"), func(c fiber.Ctx) error {
		nfce, err := svc.GetNFCe(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if nfce == nil {
			return sendProblem(c, nfesvc.ErrNFCeNotFound)
		}
		return sendItem(c, nfce)
	})

	// GET /nfces/:access_key/xml
	g.Get("/:access_key/xml", perm.Require("get.nfces"), func(c fiber.Ctx) error {
		data, err := svc.GetNFCeXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Disposition", `attachment; filename="`+c.Params("access_key")+`.xml"`)
		return c.Send(data)
	})

	// GET /nfces/:access_key/danfce — DANFC-e PDF rendered by py-dfe
	g.Get("/:access_key/danfce", perm.Require("get.nfces"), func(c fiber.Ctx) error {
		orgPK := middleware.GetOrgPK(c)
		accessKey := c.Params("access_key")
		nfce, err := svc.GetNFCe(c.Context(), orgPK, accessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		if nfce == nil {
			return sendProblem(c, nfesvc.ErrNFCeNotFound)
		}
		xml, err := svc.GetNFCeXML(c.Context(), orgPK, accessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		pdf, err := ext.GeneratePDF(c.Context(), services.DocTypeNFCe, services.ServiceGerarDanfe,
			services.UFFromCode[accessKey[0:2]], services.StripPKPrefix(orgPK), string(xml),
			attrStr(nfce, "status") == services.StatusCancelled)
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/pdf")
		c.Set("Content-Disposition", `attachment; filename="`+accessKey+`.pdf"`)
		return c.Send(pdf)
	})

	// POST /nfces/:access_key/cancel
	g.Post("/:access_key/cancel", perm.Require("delete.nfces"), func(c fiber.Ctx) error {
		var body CancelEventBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfce, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Justification, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfce)
	})

	// POST /nfces/:access_key/substitute — cancel by substitution (event 110112)
	g.Post("/:access_key/substitute", perm.Require("delete.nfces"), func(c fiber.Ctx) error {
		var body SubstituteCancelBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfce, err := svc.Substitute(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.SubstituteKey, body.Justification, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfce)
	})

	// GET /nfces/:access_key/events
	g.Get("/:access_key/events", perm.Require("get.nfce_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNFCeEvents(c.Context(), c.Params("access_key"),
			intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /nfces/:access_key/events/:event_sk/xml
	g.Get("/:access_key/events/:event_sk/xml", perm.Require("get.nfce_events"), func(c fiber.Ctx) error {
		data, eventType, err := svc.GetEventXML(c.Context(), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Disposition", `attachment; filename="`+eventType+`-`+c.Params("access_key")+`.xml"`)
		return c.Send(data)
	})
}
