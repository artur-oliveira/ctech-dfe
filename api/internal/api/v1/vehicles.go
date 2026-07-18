package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// validateVehicleRequirementsParams checks the doc_type/role query params for
// GET /vehicles/:sk/requirements against the finite set of supported values.
func validateVehicleRequirementsParams(docType, role string) *problem.Problem {
	validDocTypes := map[string]bool{services.DocTypeMdfe: true, services.DocTypeNfe: true, services.DocTypeCteOS: true}
	validRoles := map[string]bool{services.VehicleRoleTractor: true, services.VehicleRoleTrailer: true}
	if !validDocTypes[docType] {
		return problem.BadRequest("doc_type inválido: " + docType)
	}
	if !validRoles[role] {
		return problem.BadRequest("role inválido: " + role)
	}
	return nil
}

// RegisterVehicles mounts /vehicles routes.
func RegisterVehicles(router fiber.Router, svc *services.VehicleService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	mountCRUD(router, "/vehicles", authMw, perm, userSvc, crudHandlers{
		listPerm: "list.organization_vehicles", createPerm: "create.organization_vehicles",
		getPerm: "get.organization_vehicles", updatePerm: "update.organization_vehicles",
		deletePerm: "delete.organization_vehicles",
		param:      "sk",

		list: func(c fiber.Ctx, orgPK string, o crudListOpts) (*repositories.QueryResult, error) {
			opts := repositories.VehicleListOpts{
				PlatePrefix: c.Query("plate"),
				Sort:        o.Sort,
				Limit:       o.Limit,
				StartKey:    o.StartKey,
			}
			if role := c.Query("role"); role != "" {
				return svc.ListByRole(c.Context(), orgPK, role, opts)
			}
			return svc.List(c.Context(), orgPK, opts)
		},
		create: func(c fiber.Ctx, orgPK, userID, userName string) (map[string]types.AttributeValue, error) {
			var dto VehicleCreateBody
			if p := bindJSON(c, &dto); p != nil {
				return nil, p
			}
			body, err := structToMap(dto)
			if err != nil {
				return nil, err
			}
			return svc.Create(c.Context(), orgPK, body, userID, userName)
		},
		get: func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error) {
			return svc.Get(c.Context(), orgPK, id)
		},
		update: func(c fiber.Ctx, orgPK, id, userID, userName string) (map[string]types.AttributeValue, error) {
			var dto VehicleUpdateBody
			if p := bindJSON(c, &dto); p != nil {
				return nil, p
			}
			body, err := structToMap(dto)
			if err != nil {
				return nil, err
			}
			return svc.Update(c.Context(), orgPK, id, body, userID, userName)
		},
		del: func(c fiber.Ctx, orgPK, id, userID, userName string) error {
			return svc.Delete(c.Context(), orgPK, id, userID, userName)
		},
	})

	g := router.Group("/vehicles", authMw)

	// GET /vehicles/:sk/requirements
	g.Get("/:sk/requirements", perm.Require("get.organization_vehicles"), func(c fiber.Ctx) error {
		docType := c.Query("doc_type")
		role := c.Query("role")
		if p := validateVehicleRequirementsParams(docType, role); p != nil {
			return sendProblem(c, p)
		}
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"missing": services.Missing(item, docType, role)})
	})
}
