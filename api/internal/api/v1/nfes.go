package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
	nfesvc "github.com/artur-oliveira/ctech-dfe/api/internal/services/nfes"

	"github.com/gofiber/fiber/v3"
)

// RegisterNFes mounts all /nfes routes.
func RegisterNFes(router fiber.Router, svc *nfesvc.NfeService, ext *services.ExternalService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
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
		orgPK := middleware.GetOrgPK(c)
		accessKey := c.Params("access_key")
		nfce, err := svc.GetNFe(c.Context(), orgPK, accessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		if nfce == nil {
			return sendProblem(c, nfesvc.ErrNFCeNotFound)
		}
		xml, err := svc.GetNFeXML(c.Context(), orgPK, accessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		pdf, err := ext.GeneratePDF(
			c.Context(),
			services.DocTypeNFe,
			services.ServiceGerarDanfe,
			services.UFFromCode[accessKey[0:2]],
			services.StripPKPrefix(orgPK),
			string(xml),
			attrStr(nfce, "status") == services.StatusCancelled,
		)
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/pdf")
		c.Set("Content-Disposition", `attachment; filename="`+accessKey+`.pdf"`)
		return c.Send(pdf)
	})

	// GET /nfes/:access_key/xml
	g.Get("/:access_key/xml", perm.Require("get.nfes"), func(c fiber.Ctx) error {
		data, err := svc.GetNFeXML(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Disposition", `attachment; filename="`+c.Params("access_key")+`.xml"`)
		return c.Send(data)
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
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.Manifestation(c.Context(), middleware.GetOrgPK(c), c.Params("access_key"), body.EventType, body.SequenceNumber, body.Justification, userID, userName)
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
		data, eventType, err := svc.GetEventXML(c.Context(), c.Params("access_key"), c.Params("event_sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		c.Set("Content-Type", "application/xml")
		c.Set("Content-Disposition", `attachment; filename="`+eventType+`-`+c.Params("access_key")+`.xml"`)
		return c.Send(data)
	})
}
