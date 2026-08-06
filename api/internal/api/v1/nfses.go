package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	nfsesvc "gopkg.aoctech.app/dfe/api/internal/services/nfses"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"

	"github.com/gofiber/fiber/v3"
)

// paramID é o identificador do documento na rota. Aceita id_dps ou chave de
// acesso — GetNfse resolve os dois, porque a chave só existe após autorização.
const paramID = "id"

// municipalParamArgs monta os argumentos posicionais da consulta de
// parametrização municipal na ordem do path do ADN (nacional.MunicipalParameters).
// A aridade é validada no serviço contra nacional.ParamArity; aqui só se
// traduz query string em posição.
func municipalParamArgs(kind, city, service, competence, benefitNumber string) []string {
	switch kind {
	case nacional.ParamAliquota, nacional.ParamRegimesEspeciais:
		return []string{city, service, competence}
	case nacional.ParamBeneficio:
		return []string{city, benefitNumber, competence}
	case nacional.ParamRetencoes:
		return []string{city, competence}
	default:
		return []string{city}
	}
}

// RegisterNfses mounts all /nfses routes.
func RegisterNfses(router fiber.Router, svc *nfsesvc.NfseService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/nfses", authMw)

	// POST /nfses — emit a new NFS-e
	g.Post("", perm.Require("create.nfses"), func(c fiber.Ctx) error {
		var body nfsesvc.NfseEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		nfse, err := svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(nfse)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// GET /nfses
	g.Get("", perm.Require("list.nfses"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListNfses(c.Context(), middleware.GetOrgPK(c), repositories.NfseListOpts{
			Limit:    intQuery(c, "limit", 50),
			StartKey: decodeCursor(cursor),
			Status:   ptrQuery(c, "status"),
			Number:   ptrIntQuery(c, "number"),
			Year:     ptrIntQuery(c, "year"),
			Month:    ptrIntQuery(c, "month"),
			Sort:     c.Query("sort", "asc"),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /nfses/:id
	g.Get("/:id", perm.Require("get.nfses"), func(c fiber.Ctx) error {
		item, err := svc.GetNfse(c.Context(), middleware.GetOrgPK(c), c.Params(paramID))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// GET /nfses/:id/xml — XML da NFS-e autorizada
	g.Get("/:id/xml", perm.Require("get.nfses"), func(c fiber.Ctx) error {
		data, err := svc.GetNfseXML(c.Context(), middleware.GetOrgPK(c), c.Params(paramID))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendXML(c, data, c.Params(paramID))
	})

	// GET /nfses/:id/dps-xml — a DPS assinada por nós
	g.Get("/:id/dps-xml", perm.Require("get.nfses"), func(c fiber.Ctx) error {
		data, err := svc.GetDPSXML(c.Context(), middleware.GetOrgPK(c), c.Params(paramID))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendXML(c, data, "DPS-"+c.Params(paramID))
	})

	// GET /nfses/:id/danfse — proxy do PDF do ADN
	g.Get("/:id/danfse", perm.Require("get.nfses"), func(c fiber.Ctx) error {
		pdf, err := svc.GetDANFSE(c.Context(), middleware.GetOrgPK(c), c.Params(paramID))
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set(fiber.HeaderContentType, "application/pdf")
		c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+c.Params(paramID)+`.pdf"`)
		return c.Send(pdf)
	})

	// POST /nfses/:id/cancel
	g.Post("/:id/cancel", perm.Require("delete.nfses"), func(c fiber.Ctx) error {
		var body struct {
			ReasonCode        string `json:"reason_code" validate:"required,max=2"`
			ReasonDescription string `json:"reason_description" validate:"required,max=255"`
			SequenceNumber    int    `json:"sequence_number" validate:"omitempty,gte=1,lte=999"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params(paramID),
			body.ReasonCode, body.ReasonDescription, body.SequenceNumber, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// POST /nfses/:id/substitute — nova emissão que substitui a original. Não é
	// evento: o fisco gera o 105102 e cancela a substituída.
	g.Post("/:id/substitute", perm.Require("create.nfses"), func(c fiber.Ctx) error {
		var body nfsesvc.NfseEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Substitute(c.Context(), middleware.GetOrgPK(c), c.Params(paramID), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// POST /nfses/:id/events
	g.Post("/:id/events", perm.Require("create.nfse_events"), func(c fiber.Ctx) error {
		var body nfsesvc.NfseEventBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.SendEvent(c.Context(), middleware.GetOrgPK(c), c.Params(paramID), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})

	// GET /nfses/:id/events
	g.Get("/:id/events", perm.Require("get.nfse_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListEvents(c.Context(), middleware.GetOrgPK(c), c.Params(paramID),
			intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /nfses/:id/events/:event_sk/xml
	g.Get("/:id/events/:event_sk/xml", perm.Require("get.nfse_events"), func(c fiber.Ctx) error {
		data, eventType, err := svc.GetEventXML(c.Context(), middleware.GetOrgPK(c), c.Params(paramID), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendXML(c, data, eventType+"-"+c.Params(paramID))
	})

	// Consultas que não pendem de um documento.
	n := router.Group("/nfse", authMw)

	// GET /nfse/municipal-parameters/:city/:kind
	n.Get("/municipal-parameters/:city/:kind", perm.Require("get.nfses"), func(c fiber.Ctx) error {
		kind := c.Params("kind")
		args := municipalParamArgs(kind, c.Params("city"),
			c.Query("service"), c.Query("competence"), c.Query("benefit_number"))
		res, err := svc.MunicipalParameters(c.Context(), middleware.GetOrgPK(c), kind, args)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(res)
	})

	// GET /nfse/distributions — documentos recebidos do ADN
	n.Get("/distributions", perm.Require("list.nfses"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListDistributions(c.Context(), middleware.GetOrgPK(c), repositories.DistributionListOpts{
			Limit:    intQuery(c, "limit", 50),
			StartKey: decodeCursor(cursor),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})
}
