package v1

import (
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"
	"github.com/artur-oliveira/ctech-dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// RegisterPersons mounts /persons routes.
func RegisterPersons(router fiber.Router, svc *services.PersonService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	mountCRUD(router, "/persons", authMw, perm, userSvc, crudHandlers{
		listPerm:   "list.organization_persons",
		createPerm: "create.organization_persons",
		getPerm:    "get.organization_persons",
		updatePerm: "update.organization_persons",
		deletePerm: "delete.organization_persons",
		param:      "cpf_cnpj",

		list: func(c fiber.Ctx, orgPK string, o crudListOpts) (*repositories.QueryResult, error) {
			return svc.List(c.Context(), orgPK, repositories.PersonListOpts{
				NamePrefix: c.Query("name"),
				Sort:       o.Sort,
				Limit:      o.Limit,
				StartKey:   o.StartKey,
			})
		},

		create: func(c fiber.Ctx, orgPK, userID, userName string) (map[string]types.AttributeValue, error) {
			var dto PersonCreateBody
			if p := bindJSON(c, &dto); p != nil {
				return nil, p
			}
			if err := services.RequirePJFields(dto.CpfOrCnpj, dto.Person.Crt); err != nil {
				return nil, err
			}
			body, err := structToMap(dto)
			if err != nil {
				return nil, err
			}
			return svc.Create(c.Context(), orgPK, dto.CpfOrCnpj, body, userID, userName)
		},

		get: func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error) {
			return svc.Get(c.Context(), orgPK, id)
		},

		update: func(c fiber.Ctx, orgPK, id, userID, userName string) (map[string]types.AttributeValue, error) {
			var dto PersonUpdateBody
			if p := bindJSON(c, &dto); p != nil {
				return nil, p
			}
			if dto.Person != nil {
				crt := dto.Person.Crt
				if crt == nil {
					current, err := svc.Get(c.Context(), orgPK, id)
					if err != nil {
						return nil, err
					}
					currentMap, err := unmarshal(current)
					if err != nil {
						return nil, err
					}
					crt, _ = extractCrtAndRegs(currentMap)
				}
				if err := services.RequirePJFields(id, crt); err != nil {
					return nil, err
				}
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
