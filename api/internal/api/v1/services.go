package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// RegisterServices mounts /services routes under a tenant-scoped group.
func RegisterServices(router fiber.Router, svc *services.ServiceService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	mountCRUD(router, "/services", authMw, perm, userSvc, crudHandlers{
		listPerm: "list.organization_services", createPerm: "create.organization_services",
		getPerm: "get.organization_services", updatePerm: "update.organization_services",
		deletePerm: "delete.organization_services",
		param:      "service_id",

		list: func(c fiber.Ctx, orgPK string, o crudListOpts) (*repositories.QueryResult, error) {
			return svc.List(c.Context(), orgPK, repositories.ServiceListOpts{
				DescriptionPrefix: c.Query("description"),
				CodePrefix:        c.Query("code"),
				OrderBy:           c.Query("order_by"),
				Sort:              o.Sort,
				Limit:             o.Limit,
				StartKey:          o.StartKey,
			})
		},
		create: func(c fiber.Ctx, orgPK, userID, userName string) (map[string]types.AttributeValue, error) {
			av, p := bindAVValidated[ServiceBody](c)
			if p != nil {
				return nil, p
			}
			return svc.Create(c.Context(), orgPK, av, userID, userName)
		},
		get: func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error) {
			return svc.Get(c.Context(), orgPK, id)
		},
		update: func(c fiber.Ctx, orgPK, id, userID, userName string) (map[string]types.AttributeValue, error) {
			var dto ServiceBody
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
}
