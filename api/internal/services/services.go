package services

import (
	"context"
	"strconv"
	"strings"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// AttrServiceSchemaVersion é a versão dos subgrupos gravada pelo servidor. Um
// registro anterior aos subgrupos não tem o atributo e vale como versão 1: é
// legível como sempre, e ganha o carimbo na primeira atualização. Não existe
// migração destrutiva.
const AttrServiceSchemaVersion = "schema_version"

// ServiceSchemaVersionLegacy é o que se assume quando o atributo não existe.
const ServiceSchemaVersionLegacy = 1

// ServiceService is the catálogo de serviços consumido pela emissão de NFS-e.
type ServiceService struct {
	repo      *repositories.ServiceRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
	crud      *CRUDMutationHelper
}

func NewServiceService(repo *repositories.ServiceRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ServiceService {
	return &ServiceService{
		repo:      repo,
		auditRepo: auditRepo,
		cache:     c,
		crud:      NewCRUDMutationHelper(auditRepo, c),
	}
}

func (s *ServiceService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := BuildItemCacheKey(orgPK, "services", sk)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, sk)
	}, "service not found")
}

func (s *ServiceService) List(ctx context.Context, orgPK string, opts repositories.ServiceListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, "services", opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.List(ctx, orgPK, opts)
	})
}

// Create writes the service and its CREATE audit row atomically.
func (s *ServiceService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string, version int) (map[string]types.AttributeValue, error) {
	return s.crud.Create(ctx, orgPK, repositories.AuditResourceService, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, stampServiceSchemaVersion(fields, version))
		return tx, item, nil
	}, s.repo.TransactWrite)
}

// Update writes the service change and its UPDATE audit row atomically.
func (s *ServiceService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string, version int) (map[string]types.AttributeValue, error) {
	updates[AttrServiceSchemaVersion] = version
	return s.crud.Update(ctx, orgPK, sk, repositories.AuditResourceService, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	}, s.repo.TransactWrite)
}

// Delete removes the service and writes its DELETE audit row atomically.
func (s *ServiceService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	return s.crud.Delete(ctx, orgPK, sk, repositories.AuditResourceService, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, sk), nil
	}, s.repo.TransactWrite)
}

// stampServiceSchemaVersion carimba a versão do contrato no item gravado.
func stampServiceSchemaVersion(fields map[string]types.AttributeValue, version int) map[string]types.AttributeValue {
	fields[AttrServiceSchemaVersion] = &types.AttributeValueMemberN{Value: strconv.Itoa(version)}
	return fields
}

// ServiceScenario nomeia um cenário de emissão que o catálogo pode ou não
// cobrir. O diagnóstico existe porque o serviço só precisa dos grupos do
// cenário que ele realmente atende: exigir tudo de todos tornaria o cadastro
// impossível de preencher.
type ServiceScenario = string

const (
	ScenarioNacional      ServiceScenario = "prestacao_nacional"
	ScenarioExterior      ServiceScenario = "prestacao_exterior"
	ScenarioFederal       ServiceScenario = "tributacao_federal"
	ScenarioIBSCBS        ServiceScenario = "ibs_cbs"
	ScenarioCompraGov     ServiceScenario = "compra_governamental"
	ScenarioTransparencia ServiceScenario = "lei_da_transparencia"
)

// serviceScenarioFields lista, por cenário, os caminhos pontilhados exigidos.
// Os nomes são os mesmos do JSON do contrato, para o cliente conseguir apontar
// o campo na tela sem tradução.
var serviceScenarioFields = map[ServiceScenario][]string{
	ScenarioNacional: {"trib_nacional_code", "iss.trib_issqn", "iss.tax_rate",
		"location_defaults.c_loc_prestacao"},
	ScenarioExterior: {"location_defaults.c_pais_prestacao",
		"foreign_trade_defaults.md_prestacao", "foreign_trade_defaults.vinc_prest",
		"foreign_trade_defaults.tp_moeda", "foreign_trade_defaults.mec_af_comex_p",
		"foreign_trade_defaults.mec_af_comex_t", "foreign_trade_defaults.mov_temp_bens",
		"foreign_trade_defaults.mdic"},
	ScenarioFederal:       {"federal.cst_pis_cofins", "federal.tp_ret_pis_cofins"},
	ScenarioIBSCBS:        {"ibs_cbs.cst", "ibs_cbs.c_class_trib", "ibs_cbs.c_ind_op", "ibs_cbs.ind_dest"},
	ScenarioCompraGov:     {"ibs_cbs.tp_ente_gov"},
	ScenarioTransparencia: {"tot_trib.p_tot_trib_sn"},
}

// ServiceCompleteness devolve, por cenário, os campos que faltam no registro.
// Cenário sem pendência sai com lista vazia — a chave permanece para o cliente
// saber que o cenário foi avaliado, e não que ele não existe.
func ServiceCompleteness(item map[string]types.AttributeValue) map[ServiceScenario][]string {
	out := make(map[ServiceScenario][]string, len(serviceScenarioFields))
	for scenario, fields := range serviceScenarioFields {
		missing := make([]string, 0)
		for _, field := range fields {
			if !hasAttrPath(item, field) {
				missing = append(missing, field)
			}
		}
		out[scenario] = missing
	}
	return out
}

// hasAttrPath resolve um caminho pontilhado dentro do item do DynamoDB e
// informa se ele tem valor útil. String vazia e NULL contam como ausência.
func hasAttrPath(item map[string]types.AttributeValue, path string) bool {
	current := item
	segments := strings.Split(path, ".")
	for index, segment := range segments {
		value, ok := current[segment]
		if !ok {
			return false
		}
		if index == len(segments)-1 {
			return attrHasValue(value)
		}
		nested, ok := value.(*types.AttributeValueMemberM)
		if !ok {
			return false
		}
		current = nested.Value
	}
	return false
}

func attrHasValue(value types.AttributeValue) bool {
	switch typed := value.(type) {
	case *types.AttributeValueMemberNULL:
		return false
	case *types.AttributeValueMemberS:
		return typed.Value != ""
	case *types.AttributeValueMemberN:
		return typed.Value != ""
	case *types.AttributeValueMemberM:
		return len(typed.Value) > 0
	case *types.AttributeValueMemberL:
		return len(typed.Value) > 0
	default:
		return true
	}
}

// ServiceSchemaVersionOf lê a versão do contrato gravada no item; registro sem
// o atributo é legado (versão 1).
func ServiceSchemaVersionOf(item map[string]types.AttributeValue) int {
	value, ok := item[AttrServiceSchemaVersion].(*types.AttributeValueMemberN)
	if !ok {
		return ServiceSchemaVersionLegacy
	}
	version, err := strconv.Atoi(value.Value)
	if err != nil {
		return ServiceSchemaVersionLegacy
	}
	return version
}
