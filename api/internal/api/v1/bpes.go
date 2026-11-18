package v1

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	bpesvc "gopkg.aoctech.app/dfe/api/internal/services/bpes"

	"github.com/gofiber/fiber/v3"
)

// RegisterBPes mounts all /bpes routes.
func RegisterBPes(router fiber.Router, svc *bpesvc.BpeService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/bpes", authMw)

	// POST /bpes: emite um BP-e. `variant` escolhe o corpo: normal (padrão), tm ou ta.
	g.Post("", perm.Require("create.bpes"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		var doc map[string]types.AttributeValue
		var err error
		switch emitVariant(c.Body()) {
		case bpesvc.VariantTM:
			var body bpesvc.BpeTMRequest
			if p := bindJSON(c, &body); p != nil {
				return sendProblem(c, p)
			}
			doc, err = svc.EmitTM(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		default:
			var body bpesvc.BpeTicketRequest
			if p := bindJSON(c, &body); p != nil {
				return sendProblem(c, p)
			}
			doc, err = svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		}
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(doc)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /bpes
	g.Get("", perm.Require("list.bpes"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListBPes(c.Context(), middleware.GetOrgPK(c), repositories.NFeListOpts{
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

	// GET /bpes/:access_key
	g.Get("/:access_key", perm.Require("get.bpes"), func(c fiber.Ctx) error {
		doc, err := svc.GetBPe(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if doc == nil {
			return sendProblem(c, bpesvc.ErrBPeNotFound)
		}
		return sendItem(c, doc)
	})

	// GET /bpes/:access_key/xml
	g.Get("/:access_key/xml", perm.Require("get.bpes"), func(c fiber.Ctx) error {
		download, err := svc.GetBPeXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// GET /bpes/:access_key/dabpe: URL do PDF do DABPE em cache, renderizado no processo
	g.Get("/:access_key/dabpe", perm.Require("get.bpes"), func(c fiber.Ctx) error {
		download, err := svc.GetDABPEURL(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Query("format"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// POST /bpes/:access_key/cancel
	g.Post("/:access_key/cancel", perm.Require("delete.bpes"), func(c fiber.Ctx) error {
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

	// POST /bpes/:access_key/no-show: passageiro não embarcou (110115)
	g.Post("/:access_key/no-show", perm.Require("create.bpe_events"), func(c fiber.Ctx) error {
		var body CancelEventBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.NoShow(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Justification, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// POST /bpes/:access_key/seat-change: alteração de poltrona (110116)
	g.Post("/:access_key/seat-change", perm.Require("create.bpe_events"), func(c fiber.Ctx) error {
		var body struct {
			Seat           string `json:"seat" validate:"required,max=3,numeric"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.ChangeSeat(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Seat, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// POST /bpes/:access_key/excess-baggage: excesso de bagagem (110117)
	g.Post("/:access_key/excess-baggage", perm.Require("create.bpe_events"), func(c fiber.Ctx) error {
		var body struct {
			Quantity       string `json:"quantity" validate:"required,min=1,max=20"`
			Value          string `json:"value" validate:"required"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.ExcessBaggage(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Quantity, body.Value, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// POST /bpes/:access_key/supply-not-performed: fornecimento não realizado, só TA (112140)
	g.Post("/:access_key/supply-not-performed", perm.Require("create.bpe_events"), func(c fiber.Ctx) error {
		var body struct {
			IBSValue       string `json:"ibs_value" validate:"required"`
			CBSValue       string `json:"cbs_value" validate:"required"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		doc, err := svc.SupplyNotPerformed(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.IBSValue, body.CBSValue, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, doc)
	})

	// POST /bpes/:access_key/payment-link: vinculação de pagamento (110300)
	g.Post("/:access_key/payment-link", perm.Require("create.bpe_events"), func(c fiber.Ctx) error {
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

	// POST /bpes/:access_key/payment-link-cancel: cancela um 110300 (110301) pelo event_sk
	g.Post("/:access_key/payment-link-cancel", perm.Require("create.bpe_events"), func(c fiber.Ctx) error {
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

	// GET /bpes/:access_key/events
	g.Get("/:access_key/events", perm.Require("get.bpe_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListBPeEvents(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /bpes/:access_key/events/:event_sk/xml
	g.Get("/:access_key/events/:event_sk/xml", perm.Require("get.bpe_events"), func(c fiber.Ctx) error {
		download, err := svc.GetEventXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})
}

// emitVariant lê só o campo `variant` do corpo para escolher o DTO; JSON inválido devolve "" e o bindJSON do DTO comum
// responde o 400.
func emitVariant(body []byte) string {
	var peek struct {
		Variant string `json:"variant"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return ""
	}
	return peek.Variant
}
