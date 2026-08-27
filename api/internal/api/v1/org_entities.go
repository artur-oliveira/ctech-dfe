package v1

import (
	"context"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// queryParamName is the name-prefix search parameter shared by every registry
// listing (they all sit on the same `name-index`).
const queryParamName = "name"

// orgEntitySvc is what every reusable-registry service exposes. Declaring it as
// an interface is what lets one mount function serve all four registries
// instead of four copies differing only by type.
type orgEntitySvc interface {
	List(ctx context.Context, orgPK string, opts repositories.OrgEntityListOpts) (*repositories.QueryResult, error)
	Get(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error)
	Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error)
	Update(ctx context.Context, orgPK, id string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error)
	Delete(ctx context.Context, orgPK, id, userID, userName string) error
}

// orgEntityRoutes describes one registry's mounting: its path, path parameter,
// RBAC resource and request body type.
type orgEntityRoutes struct {
	path     string
	param    string
	resource string
	// bindCreate/bindUpdate isolate the only genuinely per-entity part: which
	// DTO the body is validated against.
	bindCreate func(fiber.Ctx) (map[string]types.AttributeValue, error)
	bindUpdate func(fiber.Ctx) (map[string]any, error)
}

// mountOrgEntity mounts the five CRUD routes for one reusable registry entity.
func mountOrgEntity(router fiber.Router, authMw fiber.Handler, perm *middleware.PermChecker,
	userSvc *services.UserService, svc orgEntitySvc, r orgEntityRoutes) {
	mountCRUD(router, r.path, authMw, perm, userSvc, crudHandlers{
		listPerm:   "list." + r.resource,
		createPerm: "create." + r.resource,
		getPerm:    "get." + r.resource,
		updatePerm: "update." + r.resource,
		deletePerm: "delete." + r.resource,
		param:      r.param,

		list: func(c fiber.Ctx, orgPK string, o crudListOpts) (*repositories.QueryResult, error) {
			return svc.List(c.Context(), orgPK, repositories.OrgEntityListOpts{
				NamePrefix: c.Query(queryParamName),
				Sort:       o.Sort,
				Limit:      o.Limit,
				StartKey:   o.StartKey,
			})
		},
		create: func(c fiber.Ctx, orgPK, userID, userName string) (map[string]types.AttributeValue, error) {
			av, err := r.bindCreate(c)
			if err != nil {
				return nil, err
			}
			return svc.Create(c.Context(), orgPK, av, userID, userName)
		},
		get: func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error) {
			return svc.Get(c.Context(), orgPK, id)
		},
		update: func(c fiber.Ctx, orgPK, id, userID, userName string) (map[string]types.AttributeValue, error) {
			body, err := r.bindUpdate(c)
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

// bindEntityCreate/bindEntityUpdate are the two generic binders every registry
// uses; only the DTO type changes.
func bindEntityCreate[T any](c fiber.Ctx) (map[string]types.AttributeValue, error) {
	av, p := bindAVValidated[T](c)
	if p != nil {
		return nil, p
	}
	return av, nil
}

func bindEntityUpdate[T any](c fiber.Ctx) (map[string]any, error) {
	var dto T
	if p := bindJSON(c, &dto); p != nil {
		return nil, p
	}
	return structToMap(dto)
}

// entityValidator é o corpo que tem uma regra que as tags de validação não
// expressam (um campo obrigatório só em função de outro, por exemplo).
type entityValidator interface{ Validate() error }

// bindValidatedCreate/Update aplicam essa regra depois do bind genérico.
func bindValidatedCreate[T entityValidator](c fiber.Ctx) (map[string]types.AttributeValue, error) {
	var dto T
	if p := bindJSON(c, &dto); p != nil {
		return nil, p
	}
	if err := dto.Validate(); err != nil {
		return nil, err
	}
	av, err := structToAV(dto)
	if err != nil {
		return nil, problem.InternalServer(err.Error())
	}
	return av, nil
}

func bindValidatedUpdate[T entityValidator](c fiber.Ctx) (map[string]any, error) {
	var dto T
	if p := bindJSON(c, &dto); p != nil {
		return nil, p
	}
	if err := dto.Validate(); err != nil {
		return nil, err
	}
	return structToMap(dto)
}

// RegisterVehicleSets mounts /vehicle-sets under a tenant-scoped group.
func RegisterVehicleSets(router fiber.Router, svc *services.VehicleSetService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/vehicle-sets",
		param:      "vehicle_set_id",
		resource:   "organization_vehicle_sets",
		bindCreate: bindEntityCreate[VehicleSetBody],
		bindUpdate: bindEntityUpdate[VehicleSetBody],
	})
}

// RegisterPaymentTerms mounts /payment-terms under a tenant-scoped group.
func RegisterPaymentTerms(router fiber.Router, svc *services.PaymentTermService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/payment-terms",
		param:      "payment_term_id",
		resource:   "organization_payment_terms",
		bindCreate: bindEntityCreate[PaymentTermBody],
		bindUpdate: bindEntityUpdate[PaymentTermBody],
	})
}

// RegisterPaymentTerminals mounts /payment-terminals under a tenant-scoped group.
func RegisterPaymentTerminals(router fiber.Router, svc *services.PaymentTerminalService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/payment-terminals",
		param:      "payment_terminal_id",
		resource:   "organization_payment_terminals",
		bindCreate: bindEntityCreate[PaymentTerminalBody],
		bindUpdate: bindEntityUpdate[PaymentTerminalBody],
	})
}

// RegisterTollProviders mounts /toll-providers under a tenant-scoped group.
func RegisterTollProviders(router fiber.Router, svc *services.TollProviderService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/toll-providers",
		param:      "toll_provider_id",
		resource:   "organization_toll_providers",
		bindCreate: bindEntityCreate[TollProviderBody],
		bindUpdate: bindEntityUpdate[TollProviderBody],
	})
}

// RegisterCargoUnits mounts /cargo-units under a tenant-scoped group.
func RegisterCargoUnits(router fiber.Router, svc *services.CargoUnitService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/cargo-units",
		param:      "cargo_unit_id",
		resource:   "organization_cargo_units",
		bindCreate: bindEntityCreate[CargoUnitBody],
		bindUpdate: bindEntityUpdate[CargoUnitBody],
	})
}

// RegisterImportDeclarations mounts /import-declarations under a tenant-scoped group.
func RegisterImportDeclarations(router fiber.Router, svc *services.ImportDeclarationService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/import-declarations",
		param:      "import_declaration_id",
		resource:   "organization_import_declarations",
		bindCreate: bindValidatedCreate[ImportDeclarationBody],
		bindUpdate: bindValidatedUpdate[ImportDeclarationBody],
	})
}

// RegisterInsurancePolicies mounts /insurance-policies under a tenant-scoped group.
func RegisterInsurancePolicies(router fiber.Router, svc *services.InsurancePolicyService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/insurance-policies",
		param:      "insurance_policy_id",
		resource:   "organization_insurance_policies",
		bindCreate: bindValidatedCreate[InsurancePolicyBody],
		bindUpdate: bindValidatedUpdate[InsurancePolicyBody],
	})
}

// RegisterOperations mounts /operations under a tenant-scoped group.
func RegisterOperations(router fiber.Router, svc *services.OperationService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/operations",
		param:      "operation_id",
		resource:   "organization_operations",
		bindCreate: bindOperationCreate,
		bindUpdate: bindOperationUpdate,
	})
}

// bindOperationCreate/bindOperationUpdate acrescentam ao binding genérico a
// validação dos placeholders das mensagens fiscais — uma chave desconhecida tem
// que falhar aqui, no cadastro, e não virar um buraco silencioso no XML.
func bindOperationCreate(c fiber.Ctx) (map[string]types.AttributeValue, error) {
	var dto OperationBody
	if p := bindJSON(c, &dto); p != nil {
		return nil, p
	}
	if err := validateOperationPlaceholders(dto); err != nil {
		return nil, err
	}
	return structToAV(dto)
}

func bindOperationUpdate(c fiber.Ctx) (map[string]any, error) {
	var dto OperationBody
	if p := bindJSON(c, &dto); p != nil {
		return nil, p
	}
	if err := validateOperationPlaceholders(dto); err != nil {
		return nil, err
	}
	return structToMap(dto)
}

func validateOperationPlaceholders(dto OperationBody) error {
	for _, tpl := range []*string{dto.InfAdFisco, dto.InfCpl} {
		if tpl == nil {
			continue
		}
		if err := services.ValidatePlaceholders(*tpl); err != nil {
			return err
		}
	}
	// obsCont/obsFisco aceitam os mesmos placeholders: chave desconhecida é 400
	// aqui, no cadastro, nunca silêncio no XML.
	for _, obs := range append(append([]ObsBody{}, dto.ObsCont...), dto.ObsFisco...) {
		if err := services.ValidatePlaceholders(obs.XTexto); err != nil {
			return err
		}
	}
	return nil
}

// RegisterTaxProfiles mounts /tax-profiles under a tenant-scoped group.
func RegisterTaxProfiles(router fiber.Router, svc *services.TaxProfileService, userSvc *services.UserService,
	authMw fiber.Handler, perm *middleware.PermChecker) {
	mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
		path:       "/tax-profiles",
		param:      "tax_profile_id",
		resource:   "organization_tax_profiles",
		bindCreate: bindEntityCreate[TaxProfileBody],
		bindUpdate: bindEntityUpdate[TaxProfileBody],
	})
}
