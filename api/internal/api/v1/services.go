package v1

import (
	"strconv"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// withServiceDiagnostics acrescenta os campos derivados da leitura: a versão do
// contrato (registro legado responde 1) e o que falta por cenário de emissão.
// Nada disso é persistido — é diagnóstico calculado a cada leitura.
func withServiceDiagnostics(item map[string]types.AttributeValue) map[string]types.AttributeValue {
	if item == nil {
		return nil
	}
	item[services.AttrServiceSchemaVersion] = &types.AttributeValueMemberN{
		Value: strconv.Itoa(services.ServiceSchemaVersionOf(item)),
	}
	scenarios := map[string]types.AttributeValue{}
	for scenario, missing := range services.ServiceCompleteness(item) {
		values := make([]types.AttributeValue, 0, len(missing))
		for _, field := range missing {
			values = append(values, &types.AttributeValueMemberS{Value: field})
		}
		scenarios[scenario] = &types.AttributeValueMemberL{Value: values}
	}
	item["completeness"] = &types.AttributeValueMemberM{Value: scenarios}
	return item
}

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
			return svc.Create(c.Context(), orgPK, av, userID, userName, ServiceSchemaVersion)
		},
		get: func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error) {
			item, err := svc.Get(c.Context(), orgPK, id)
			if err != nil {
				return nil, err
			}
			return withServiceDiagnostics(item), nil
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
			return svc.Update(c.Context(), orgPK, id, body, userID, userName, ServiceSchemaVersion)
		},
		del: func(c fiber.Ctx, orgPK, id, userID, userName string) error {
			return svc.Delete(c.Context(), orgPK, id, userID, userName)
		},
	})
}
