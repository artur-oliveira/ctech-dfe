package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	mdfesvc "gopkg.aoctech.app/dfe/api/internal/services/mdfes"

	"github.com/gofiber/fiber/v3"
)

// RegisterMDFes mounts all /mdfes routes.
func RegisterMDFes(router fiber.Router, svc *mdfesvc.MdfeService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/mdfes", authMw)

	// POST /mdfes — emit a new MDF-e
	g.Post("", perm.Require("create.mdfes"), func(c fiber.Ctx) error {
		var body mdfesvc.MdfeEmitBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.Emit(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(mdfe)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})

	// POST /mdfes/cargo-preview — parse referenced docs and return cargo data
	g.Post("/cargo-preview", perm.Require("create.mdfes"), func(c fiber.Ctx) error {
		var body struct {
			Documents []mdfesvc.MdfeDocRef `json:"documents" validate:"required,min=1,dive"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		preview, err := svc.PreviewCargo(c.Context(), middleware.GetOrgPK(c), body.Documents)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(preview)
	})

	// GET /mdfes
	g.Get("", perm.Require("list.mdfes"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListMDFes(c.Context(), middleware.GetOrgPK(c), repositories.NFeListOpts{
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

	// GET /mdfes/:access_key
	g.Get("/:access_key", perm.Require("get.mdfes"), func(c fiber.Ctx) error {
		mdfe, err := svc.GetMDFe(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		if mdfe == nil {
			return sendProblem(c, mdfesvc.ErrMDFeNotFound)
		}
		return sendItem(c, mdfe)
	})

	// GET /mdfes/:access_key/xml
	g.Get("/:access_key/xml", perm.Require("get.mdfes"), func(c fiber.Ctx) error {
		download, err := svc.GetMDFeXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// GET /mdfes/:access_key/damdfe — cached DAMDFE PDF URL rendered in-process
	g.Get("/:access_key/damdfe", perm.Require("get.mdfes"), func(c fiber.Ctx) error {
		accessKey := c.Params("access_key")
		download, err := svc.GetDAMDFEURL(c.Context(), middleware.GetOrgPK(c), accessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})

	// POST /mdfes/:access_key/cancel
	g.Post("/:access_key/cancel", perm.Require("delete.mdfes"), func(c fiber.Ctx) error {
		var body CancelEventBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Justification, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// POST /mdfes/:access_key/close — encerramento
	g.Post("/:access_key/close", perm.Require("create.mdfe_events"), func(c fiber.Ctx) error {
		var body struct {
			CMun           string `json:"ibge_code" validate:"required,ibge"`
			UF             string `json:"uf" validate:"required,uf"`
			ByThirdParty   bool   `json:"by_third_party"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.Close(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.CMun, body.UF, body.ByThirdParty, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// POST /mdfes/:access_key/include-condutor
	g.Post("/:access_key/include-condutor", perm.Require("create.mdfe_events"), func(c fiber.Ctx) error {
		var body struct {
			Name           string `json:"name" validate:"required,max=60"`
			CPF            string `json:"cpf" validate:"required,cpf"`
			SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.IncludeCondutor(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.Name, body.CPF, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// POST /mdfes/:access_key/include-dfe
	g.Post("/:access_key/include-dfe", perm.Require("create.mdfe_events"), func(c fiber.Ctx) error {
		var body struct {
			CMunCarrega    string                  `json:"loading_ibge_code" validate:"required,ibge"`
			XMunCarrega    string                  `json:"loading_city" validate:"required,max=120"`
			Documents      []mdfesvc.IncludeDFeDoc `json:"documents" validate:"required,min=1,dive"`
			SequenceNumber int                     `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.IncludeDFe(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.CMunCarrega, body.XMunCarrega, body.Documents, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// POST /mdfes/:access_key/payment — evPagtoOperMDFe (110116)
	g.Post("/:access_key/payment", perm.Require("create.mdfe_events"), func(c fiber.Ctx) error {
		var body struct {
			Trips          mdfesvc.MdfePaymentTrips `json:"trips" validate:"required"`
			Payments       []mdfesvc.MdfePayment    `json:"payments" validate:"required,min=1,dive"`
			SequenceNumber int                      `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.PayTransportOperation(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			body.Trips, body.Payments, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// POST /mdfes/:access_key/confirm-service — evConfirmaServMDFe (110117)
	g.Post("/:access_key/confirm-service", perm.Require("create.mdfe_events"), func(c fiber.Ctx) error {
		var body struct {
			SequenceNumber int `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.ConfirmService(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// POST /mdfes/:access_key/change-payment — evAlteracaoPagtoServMDFe (110118)
	g.Post("/:access_key/change-payment", perm.Require("create.mdfe_events"), func(c fiber.Ctx) error {
		var body struct {
			Payments       []mdfesvc.MdfePayment `json:"payments" validate:"required,min=1,dive"`
			SequenceNumber int                   `json:"sequence_number" validate:"omitempty,gte=1"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		mdfe, err := svc.ChangeServicePayment(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"),
			body.Payments, defaultSeq(body.SequenceNumber), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, mdfe)
	})

	// GET /mdfes/:access_key/events
	g.Get("/:access_key/events", perm.Require("get.mdfe_events"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.ListMDFeEvents(c.Context(), c.Params("access_key"),
			intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})

	// GET /mdfes/:access_key/events/:event_sk/xml
	g.Get("/:access_key/events/:event_sk/xml", perm.Require("get.mdfe_events"), func(c fiber.Ctx) error {
		download, err := svc.GetEventXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(download)
	})
}

// defaultSeq returns 1 when the caller omits the sequence number.
func defaultSeq(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}
