package v1

import (
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/services"
	ciotsvc "gopkg.aoctech.app/dfe/api/internal/services/ciots"
	"gopkg.aoctech.app/dfe/api/internal/validation"
	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

const (
	queryEnvironment = "environment"
	queryPlates      = "plates"
)

// ciotView turns a stored row into the response: the declaration is a JSON object, never an escaped string.
func ciotView(item map[string]types.AttributeValue) (map[string]any, error) {
	m, err := unmarshal(item)
	if err != nil {
		return nil, err
	}
	if raw, ok := m["declaration_json"].(string); ok {
		delete(m, "declaration_json")
		m["declaration"] = json.RawMessage(raw)
	}
	return m, nil
}

func ciotEnv(c fiber.Ctx) (string, error) {
	switch env := c.Query(queryEnvironment, services.EnvProd); env {
	case services.EnvProd, services.EnvHom:
		return env, nil
	default:
		return "", problem.BadRequest("environment deve ser prod ou hom")
	}
}

// RegisterCIOTs mounts /ciots, /ciot-carriers and the public /freight-floor.
func RegisterCIOTs(router fiber.Router, svc *ciotsvc.Service, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	sendRow := func(c fiber.Ctx, status int, item map[string]types.AttributeValue, err error) error {
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := ciotView(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(status).JSON(m)
	}

	// Public and stateless: no organization, no data of anyone.
	router.Get("/freight-floor", func(c fiber.Ctx) error {
		in := ciot.FloorInput{
			Composition: c.Query("composition"), HighPerformance: c.Query("high_performance"),
			CargoType: intQuery(c, "cargo_type", 0), Axles: intQuery(c, "axles", 0),
			DistanceKm: intQuery(c, "distance_km", 0), EmptyReturn: c.Query("empty_return") == "true",
		}
		res, err := ciotsvc.FreightFloor(in)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(res)
	})

	carriers := router.Group("/ciot-carriers", authMw)
	carrierDoc := func(c fiber.Ctx) (string, error) {
		doc := ciot.NormalizeDoc(c.Params("cpfCnpj"))
		if p := validation.Struct(&struct {
			Doc string `validate:"cpfcnpj"`
		}{doc}); p != nil {
			return "", p
		}
		return doc, nil
	}
	carriers.Get("/:cpfCnpj/status", perm.Require("get.ciots"), func(c fiber.Ctx) error {
		doc, err := carrierDoc(c)
		if err != nil {
			return sendProblem(c, err)
		}
		env, err := ciotEnv(c)
		if err != nil {
			return sendProblem(c, err)
		}
		res, err := svc.CarrierStatus(c.Context(), middleware.GetOrgPK(c), env, doc)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(res)
	})
	carriers.Get("/:cpfCnpj/fleet", perm.Require("get.ciots"), func(c fiber.Ctx) error {
		doc, err := carrierDoc(c)
		if err != nil {
			return sendProblem(c, err)
		}
		env, err := ciotEnv(c)
		if err != nil {
			return sendProblem(c, err)
		}
		res, err := svc.CarrierFleet(c.Context(), middleware.GetOrgPK(c), env, doc, strings.Split(c.Query(queryPlates), ","))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(res)
	})

	g := router.Group("/ciots", authMw)
	g.Post("", perm.Require("create.ciots"), func(c fiber.Ctx) error {
		var body ciotsvc.DeclareBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Declare(c.Context(), middleware.GetOrgPK(c), body, userID, userName)
		return sendRow(c, fiber.StatusAccepted, item, err)
	})
	g.Get("", perm.Require("list.ciots"), func(c fiber.Ctx) error {
		env, err := ciotEnv(c)
		if err != nil {
			return sendProblem(c, err)
		}
		cursor := c.Query("cursor")
		res, err := svc.List(c.Context(), middleware.GetOrgPK(c), env, intQuery(c, "limit", 50), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})
	g.Get("/:id", perm.Require("get.ciots"), func(c fiber.Ctx) error {
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("id"))
		return sendRow(c, fiber.StatusOK, item, err)
	})
	g.Post("/:id/cancellation", perm.Require("update.ciots"), func(c fiber.Ctx) error {
		var body ciotsvc.CancelBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		item, err := svc.Cancel(c.Context(), middleware.GetOrgPK(c), c.Params("id"), body)
		return sendRow(c, fiber.StatusAccepted, item, err)
	})
	g.Post("/:id/rectification", perm.Require("update.ciots"), func(c fiber.Ctx) error {
		var body ciotsvc.RectifyBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		item, err := svc.Rectify(c.Context(), middleware.GetOrgPK(c), c.Params("id"), body)
		return sendRow(c, fiber.StatusAccepted, item, err)
	})
	g.Post("/:id/closure", perm.Require("update.ciots"), func(c fiber.Ctx) error {
		var body ciotsvc.CloseBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		item, err := svc.Close(c.Context(), middleware.GetOrgPK(c), c.Params("id"), body)
		return sendRow(c, fiber.StatusAccepted, item, err)
	})
}
