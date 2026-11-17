package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	nfcomsvc "gopkg.aoctech.app/dfe/api/internal/services/nfcoms"

	"github.com/gofiber/fiber/v3"
)

// RegisterNFComs mounts all /nfcoms routes.
func RegisterNFComs(router fiber.Router, svc *nfcomsvc.NfcomService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/nfcoms", authMw)

	// POST /nfcoms: emit a new NFCom
	g.Post("", perm.Require("create.nfcoms"), func(c fiber.Ctx) error {
		var body nfcomsvc.NfcomEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(doc)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /nfcoms
	g.Get("", perm.Require("list.nfcoms"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNFComs(c.Context(), middleware.GetOrgPK(c), repositories.NFeListOpts{
			Limit:    intQuery(c, "limit", 50),
			StartKey: decodeCursor(cursor),
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

	// GET /nfcoms/:access_key
	g.Get("/:access_key", perm.Require("get.nfcoms"), func(c fiber.Ctx) error {
		doc, err := svc.GetNFCom(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if doc == nil {
			return sendProblem(c, nfcomsvc.ErrNFComNotFound)
		}
		return sendItem(c, doc)
	})

	// GET /nfcoms/:access_key/xml
	g.Get("/:access_key/xml", perm.Require("get.nfcoms"), func(c fiber.Ctx) error {
		download, err := svc.GetNFComXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// GET /nfcoms/:access_key/danfecom: cached DANFE-COM PDF URL rendered in-process
	g.Get("/:access_key/danfecom", perm.Require("get.nfcoms"), func(c fiber.Ctx) error {
		download, err := svc.GetDANFEComURL(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Query("format"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// POST /nfcoms/:access_key/cancel
	g.Post("/:access_key/cancel", perm.Require("delete.nfcoms"), func(c fiber.Ctx) error {
		var body CancelEventBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Justification, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// POST /nfcoms/:access_key/payment-link: vinculação de pagamento (110300)
	g.Post("/:access_key/payment-link", perm.Require("create.nfcom_events"), func(c fiber.Ctx) error {
		var body struct {
			services.PaymentLink
			SequenceNumber int `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.RegisterPaymentLink(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.PaymentLink, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// POST /nfcoms/:access_key/payment-link-cancel: cancela um 110300 (110301) pelo event_sk
	g.Post("/:access_key/payment-link-cancel", perm.Require("create.nfcom_events"), func(c fiber.Ctx) error {
		var body struct {
			EventSK        string `json:"event_sk" validate:"required"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.CancelPaymentLink(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.EventSK, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// GET /nfcoms/:access_key/events
	g.Get("/:access_key/events", perm.Require("get.nfcom_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNFComEvents(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /nfcoms/:access_key/events/:event_sk/xml
	g.Get("/:access_key/events/:event_sk/xml", perm.Require("get.nfcom_events"), func(c fiber.Ctx) error {
		download, err := svc.GetEventXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})
}
