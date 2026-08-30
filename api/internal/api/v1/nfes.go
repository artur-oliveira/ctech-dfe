package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"
	"gopkg.aoctech.app/dfe/api/internal/validation"

	"github.com/gofiber/fiber/v3"
)

// RegisterNFes mounts all /nfes routes.
func RegisterNFes(router fiber.Router, svc *nfesvc.NfeService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/nfes", authMw)

	// POST /nfes — emit a new NF-e
	g.Post("", perm.Require("create.nfes"), func(c fiber.Ctx) error {
		var body nfesvc.NfeEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(nfe)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /nfes
	g.Get("", perm.Require("list.nfes"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNFes(c.Context(), middleware.GetOrgPK(c), repositories.NFeListOpts{
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

	// ── Inutilização de numeração ────────────────────────────────────────────
	// Registered before /:access_key so the literal path is not captured by the
	// access-key parameter route.

	// POST /nfes/inutilizations — inutilize an unused number range
	g.Post("/inutilizations", perm.Require("create.nfe_events"), func(c fiber.Ctx) error {
		var body nfesvc.InutilizationBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Inutilize(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /nfes/inutilizations — list requested/homologated ranges
	g.Get("/inutilizations", perm.Require("list.nfe_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListInutilizations(c.Context(), middleware.GetOrgPK(c),
			intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /nfes/inutilizations/gaps — numbering holes still open
	g.Get("/inutilizations/gaps", perm.Require("list.nfe_events"), func(c fiber.Ctx) error {
		gaps, err := svc.NumberGaps(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"items": gaps})
	})

	// GET /nfes/inutilizations/:sk/xml — ProcInutNFe (request assinado + retorno)
	g.Get("/inutilizations/:sk/xml", perm.Require("list.nfe_events"), func(c fiber.Ctx) error {
		download, err := svc.GetInutilizationXML(c.Context(), middleware.GetOrgPK(c), c.Params("sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// GET /nfes/:access_key
	g.Get("/:access_key", perm.Require("get.nfes"), func(c fiber.Ctx) error {
		nfe, err := svc.GetNFe(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if nfe == nil {
			return sendProblem(c, nfesvc.ErrNFeNotFound)
		}
		return sendItem(c, nfe)
	})

	// GET /nfes/:access_key/danfe
	g.Get("/:access_key/danfe", perm.Require("get.nfes"), func(c fiber.Ctx) error {
		accessKey := c.Params("access_key")
		download, err := svc.GetDANFeURL(c.Context(), middleware.GetOrgPK(c), accessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// GET /nfes/:access_key/xml
	g.Get("/:access_key/xml", perm.Require("get.nfes"), func(c fiber.Ctx) error {
		download, err := svc.GetNFeXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// POST /nfes/:access_key/cancel
	g.Post("/:access_key/cancel", perm.Require("delete.nfes"), func(c fiber.Ctx) error {
		var body CancelEventBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Justification, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})

	// POST /nfes/:access_key/correction-letter
	g.Post("/:access_key/correction-letter", perm.Require("create.nfe_events"), func(c fiber.Ctx) error {
		var body struct {
			CorrectionText string `json:"correction_text" validate:"required,min=15,max=1000"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.CorrectionLetter(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.CorrectionText, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})

	// POST /nfes/:access_key/manifestation
	g.Post("/:access_key/manifestation", perm.Require("create.nfe_events"), func(c fiber.Ctx) error {
		var body struct {
			EventType      string  `json:"event_type" validate:"required,oneof=210200 210210 210220 210240"`
			SequenceNumber int     `json:"sequence_number" validate:"omitempty,gte=1"`
			Justification  *string `json:"justification" validate:"omitempty,min=15,max=255"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		accessKey := c.Params("access_key")
		if !validation.ValidAccessKey(accessKey) {
			return sendProblem(c, problem.BadRequest("chave de acesso inválida"))
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.Manifestation(c.Context(), middleware.GetOrgPK(c), accessKey, body.EventType, body.SequenceNumber, body.Justification, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})

	// POST /nfes/:access_key/prorrogation — pedido de prorrogação (111500/111501)
	g.Post("/:access_key/prorrogation", perm.Require("create.nfe_events"), func(c fiber.Ctx) error {
		var body struct {
			EventType      string                    `json:"event_type" validate:"required,oneof=111500 111501"`
			Items          []nfesvc.ProrrogationItem `json:"items" validate:"required,min=1,max=990,dive"`
			SequenceNumber int                       `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.RequestProrrogation(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			body.EventType, body.Items, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})

	// POST /nfes/:access_key/prorrogation-cancel — cancelamento do pedido (111502/111503)
	g.Post("/:access_key/prorrogation-cancel", perm.Require("create.nfe_events"), func(c fiber.Ctx) error {
		var body struct {
			EventType      string `json:"event_type" validate:"required,oneof=111502 111503"`
			RequestID      string `json:"request_id" validate:"required,max=60"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.CancelProrrogation(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			body.EventType, body.RequestID, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})

	// POST /nfes/:access_key/cancel-event — cancelamento de evento (110001)
	g.Post("/:access_key/cancel-event", perm.Require("delete.nfe_events"), func(c fiber.Ctx) error {
		var body struct {
			CancelledEventType string `json:"cancelled_event_type" validate:"required,len=6,numeric"`
			CancelledProtocol  string `json:"cancelled_protocol" validate:"required,max=15,numeric"`
			SequenceNumber     int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.CancelEvent(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			body.CancelledEventType, body.CancelledProtocol, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})

	// GET /nfes/:access_key/events
	g.Get("/:access_key/events", perm.Require("get.nfe_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNFeEvents(c.Context(), c.Params("access_key"),
			intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /nfes/:access_key/events/:event_sk/xml
	g.Get("/:access_key/events/:event_sk/xml", perm.Require("get.nfe_events"), func(c fiber.Ctx) error {
		download, err := svc.GetEventXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})
}
