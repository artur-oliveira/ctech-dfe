package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	ctesvc "gopkg.aoctech.app/dfe/api/internal/services/ctes"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// acceptedCte responde 202 com o registro pending de uma emissão de CT-e (complemento, substituto), ou o problema.
func acceptedCte(c fiber.Ctx) func(map[string]types.AttributeValue, error) error {
	return func(cte map[string]types.AttributeValue, err error) error {
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(cte)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusAccepted).JSON(m)
	}
}

// RegisterCTes mounts the /ctes routes (CT-e rodoviário a partir de NF-e).
func RegisterCTes(router fiber.Router, svc *ctesvc.CteService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/ctes", authMw)

	// POST /ctes/derive — lê as notas e devolve o que o CT-e herda delas (não grava nada)
	g.Post("/derive", perm.Require("create.ctes"), perm.Require("get.nfes"), func(c fiber.Ctx) error {
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
	g.Post("", perm.Require("create.ctes"), perm.Require("get.nfes"), func(c fiber.Ctx) error {
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

	// POST /ctes/complement: CT-e complementar de valores (tpCTe 1) de CT-e autorizados da organização
	g.Post("/complement", perm.Require("create.ctes"), func(c fiber.Ctx) error {
		var body ctesvc.CteComplementBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		return acceptedCte(c)(svc.Complement(c.Context(), middleware.GetOrgPK(c), body, userID, userName))
	})

	// POST /ctes/substitute: CT-e substituto (tpCTe 3) de um CT-e autorizado da organização
	g.Post("/substitute", perm.Require("create.ctes"), func(c fiber.Ctx) error {
		var body ctesvc.CteSubstituteBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		return acceptedCte(c)(svc.Substitute(c.Context(), middleware.GetOrgPK(c), body, userID, userName))
	})

	// POST /ctes/os: CT-e OS (modelo 67: pessoas, valores ou excesso de bagagem)
	g.Post("/os", perm.Require("create.ctes"), func(c fiber.Ctx) error {
		var body ctesvc.CteOsEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		return acceptedCte(c)(svc.CteOsEmit(c.Context(), middleware.GetOrgPK(c), body, userID, userName))
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

	// GET /ctes/:access_key/sefaz-status — situação atual na SEFAZ (CTeConsulta), com protocolo e eventos.
	g.Get("/:access_key/sefaz-status", perm.Require("get.ctes"), func(c fiber.Ctx) error {
		res, err := svc.ConsultSituation(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(res)
	})

	// GET /ctes/:access_key/dacte — cached DACTE PDF URL rendered in-process
	g.Get("/:access_key/dacte", perm.Require("get.ctes"), func(c fiber.Ctx) error {
		download, err := svc.GetDACTEURL(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// GET /ctes/:access_key
	g.Get("/:access_key", perm.Require("get.ctes"), func(c fiber.Ctx) error {
		cte, err := svc.GetCTeDetail(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if cte == nil {
			return sendProblem(c, ctesvc.ErrCTeNotFound)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/cancel — cancelamento (110111); o CT-e vai a cancel_pending até a SEFAZ responder
	g.Post("/:access_key/cancel", perm.Require("delete.ctes"), func(c fiber.Ctx) error {
		var body struct {
			Justification string `json:"justification" validate:"required,min=15,max=255"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Justification, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/correction-letter — carta de correção (110110)
	g.Post("/:access_key/correction-letter", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			Corrections    []ctesvc.Correction `json:"corrections" validate:"required,min=1,max=20,dive"`
			SequenceNumber int                 `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.CorrectionLetter(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Corrections, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/service-disagreement — prestação de serviço em desacordo (610110), evento do tomador
	g.Post("/:access_key/service-disagreement", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			Notes          string `json:"notes" validate:"omitempty,min=15,max=255"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.ServiceDisagreement(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Notes, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/service-disagreement-cancel — cancela um evento de desacordo (610111)
	g.Post("/:access_key/service-disagreement-cancel", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			EventSK        string `json:"event_sk" validate:"required"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.CancelServiceDisagreement(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.EventSK, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/delivery-receipt: comprovante de entrega (110180)
	g.Post("/:access_key/delivery-receipt", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			ctesvc.DeliveryReceipt
			SequenceNumber int `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.RegisterDeliveryReceipt(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.DeliveryReceipt, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/delivery-failure: insucesso na entrega (110190)
	g.Post("/:access_key/delivery-failure", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			ctesvc.DeliveryFailure
			SequenceNumber int `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.RegisterDeliveryFailure(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.DeliveryFailure, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/delivery-cancel: cancela um 110180 (kind=receipt) ou um 110190 (kind=failure)
	g.Post("/:access_key/delivery-cancel", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			Kind           string `json:"kind" validate:"required,oneof=receipt failure"`
			EventSK        string `json:"event_sk" validate:"required"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.CancelDeliveryEvent(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.EventSK, body.Kind == "failure", body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/payment-link: vinculação de pagamento (110300), CT-e 57 e 67
	g.Post("/:access_key/payment-link", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			ctesvc.PaymentLink
			SequenceNumber int `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.RegisterPaymentLink(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.PaymentLink, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// POST /ctes/:access_key/payment-link-cancel: cancela um 110300 (110301) pelo event_sk
	g.Post("/:access_key/payment-link-cancel", perm.Require("create.cte_events"), func(c fiber.Ctx) error {
		var body struct {
			EventSK        string `json:"event_sk" validate:"required"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		cte, err := svc.CancelPaymentLink(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.EventSK, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, cte)
	})

	// GET /ctes/:access_key/events
	g.Get("/:access_key/events", perm.Require("get.cte_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListCTeEvents(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /ctes/:access_key/events/:event_sk/xml
	g.Get("/:access_key/events/:event_sk/xml", perm.Require("get.cte_events"), func(c fiber.Ctx) error {
		download, err := svc.GetEventXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})
}
