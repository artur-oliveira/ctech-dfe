package repositories

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// batchGetLimit is DynamoDB's hard cap on keys per BatchGetItem request.
const batchGetLimit = 100

// Reusable registry entities — the decisions that used to be retyped on every
// issuance, stored once and referenced by id. They all share one shape:
// pk = {org_pk}, sk = {PREFIX}_{uuid}, plus a `name-index` GSI for prefix
// search, so they share one repository instead of four near-identical copies.
const (
	TableTaxProfiles  = "organization_tax_profiles"
	TableOperations   = "organization_operations"
	TablePaymentTerms = "organization_payment_terms"
	TableVehicleSets  = "organization_vehicle_sets"
	// TablePaymentTerminals guarda os terminais de captura (POS): CNPJ recebedor
	// e identificador do terminal são invariantes por maquininha.
	TablePaymentTerminals = "organization_payment_terminals"
	// TableTollProviders guarda as fornecedoras de vale-pedágio: CNPJ da
	// fornecedora e do pagador são invariantes; por viagem muda só nº e valor.
	TableTollProviders = "organization_toll_providers"
	// TableCargoUnits guarda as unidades de transporte e de carga (carreta,
	// vagão, contêiner, pallet): identificação e tipo recorrem entre viagens.
	TableCargoUnits = "organization_cargo_units"
	// TableImportDeclarations guarda as declarações de importação: uma DI cobre
	// várias notas e vários itens, então ela é cadastro, não campo de emissão.
	TableImportDeclarations = "organization_import_declarations"
	// TableInsurancePolicies guarda as apólices de seguro da carga: a apólice e
	// a seguradora recorrem entre viagens; por viagem muda só a averbação.
	TableInsurancePolicies = "organization_insurance_policies"
	// TableProductLots guarda os lotes de produção (prod/rastro): o lote é do
	// produto e reaparece em várias notas até acabar.
	TableProductLots = "organization_product_lots"

	SKPrefixTaxProfile      = "TAXPROFILE_"
	SKPrefixOperation       = "OPERATION_"
	SKPrefixPaymentTerm     = "PAYMENTTERM_"
	SKPrefixVehicleSet      = "VEHICLESET_"
	SKPrefixPaymentTerminal = "TERMINAL_"
	SKPrefixTollProvider    = "TOLLPROVIDER_"
	SKPrefixCargoUnit       = "CARGOUNIT_"
	SKPrefixImportDI        = "IMPORTDI_"
	SKPrefixInsurance       = "INSURANCE_"
	SKPrefixProductLot      = "PRODUCTLOT_"

	// OrgEntityNameIndex is the GSI created for every registry table (see
	// getOrgEntityTable in cdk/lib/dynamodb-stack.ts).
	OrgEntityNameIndex = "name-index"
	OrgEntityNameField = "name"
)

// OrgEntityListOpts configures a registry listing.
type OrgEntityListOpts struct {
	NamePrefix string
	Sort       string
	Limit      int
	StartKey   map[string]types.AttributeValue
}

// OrgEntityRepository is the shared persistence for every reusable registry
// entity. Concrete repositories embed it so fx can inject them by distinct type
// while the CRUD body stays defined exactly once.
type OrgEntityRepository struct {
	CRUDRepository[map[string]types.AttributeValue]
	skPrefix string
}

func newOrgEntityRepository(db *dynamodb.Client, cfg *config.Config, table, skPrefix string) OrgEntityRepository {
	return OrgEntityRepository{
		CRUDRepository: NewCRUDRepository[map[string]types.AttributeValue](db, cfg, table),
		skPrefix:       skPrefix,
	}
}

// SK accepts either a bare id or an already-prefixed sk, so routes may take
// either from the path without the caller having to know which.
func (r *OrgEntityRepository) SK(id string) string {
	if strings.HasPrefix(id, r.skPrefix) {
		return id
	}
	return r.skPrefix + id
}

func (r *OrgEntityRepository) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Create(ctx, orgPK, r.SK(GenerateID()), fields)
}

func (r *OrgEntityRepository) Get(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, r.SK(id))
}

func (r *OrgEntityRepository) List(ctx context.Context, orgPK string, opts OrgEntityListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.NamePrefix != "" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.NamePrefix,
			IndexName: OrgEntityNameIndex, SKField: OrgEntityNameField,
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: r.skPrefix,
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

// BatchGet fetches many registry rows in one round trip, keyed by the id the
// caller asked for. Missing ids are simply absent from the result — the caller
// decides whether that is an error. Used by issuance, which must never do one
// GetItem per line item inside a loop.
func (r *OrgEntityRepository) BatchGet(ctx context.Context, orgPK string, ids []string) (map[string]map[string]types.AttributeValue, error) {
	out := make(map[string]map[string]types.AttributeValue, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	// De-duplicate: several products commonly reference the same profile.
	skToID := make(map[string]string, len(ids))
	keys := make([]map[string]types.AttributeValue, 0, len(ids))
	for _, id := range ids {
		sk := r.SK(id)
		if _, seen := skToID[sk]; seen {
			continue
		}
		skToID[sk] = id
		keys = append(keys, map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: orgPK},
			"sk": &types.AttributeValueMemberS{Value: sk},
		})
	}

	for start := 0; start < len(keys); start += batchGetLimit {
		end := min(start+batchGetLimit, len(keys))
		unprocessed := map[string]types.KeysAndAttributes{
			r.TableName: {Keys: keys[start:end]},
		}
		// BatchGetItem may return UnprocessedKeys under throttling; retrying
		// them is the documented contract, not an optional optimization.
		for len(unprocessed) > 0 {
			res, err := r.BatchGetItemRaw(ctx, &dynamodb.BatchGetItemInput{RequestItems: unprocessed})
			if err != nil {
				return nil, err
			}
			for _, item := range res.Responses[r.TableName] {
				sk, _ := item["sk"].(*types.AttributeValueMemberS)
				if sk == nil {
					continue
				}
				if id, ok := skToID[sk.Value]; ok {
					out[id] = item
				}
			}
			unprocessed = res.UnprocessedKeys
		}
	}
	return out, nil
}

func (r *OrgEntityRepository) Update(ctx context.Context, orgPK, id string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, r.SK(id), updates)
}

func (r *OrgEntityRepository) Delete(ctx context.Context, orgPK, id string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, r.SK(id))
}

// BuildCreateTxItem mirrors Create's key/timestamp construction without writing.
func (r *OrgEntityRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	// marshalEntity never errors for T = map[string]types.AttributeValue (base.go).
	tx, item, err := r.CRUDRepository.BuildCreateTxItem(orgPK, r.SK(GenerateID()), fields)
	logUnexpectedError("build organization entity create transaction", err)
	return tx, item
}

func (r *OrgEntityRepository) BuildUpdateTxItem(orgPK, id string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, r.SK(id), updates)
}

func (r *OrgEntityRepository) BuildDeleteTxItem(orgPK, id string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, r.SK(id))
}

// ── Concrete registries ──────────────────────────────────────────────────────

// PaymentTermRepository — organization_payment_terms. Uma condição de pagamento
// ("30/60/90", "à vista", "boleto 28 dias") expande para payments, cobr.fat e
// cobr_duplicatas na emissão.
type PaymentTermRepository struct{ OrgEntityRepository }

func NewPaymentTermRepository(db *dynamodb.Client, cfg *config.Config) *PaymentTermRepository {
	return &PaymentTermRepository{newOrgEntityRepository(db, cfg, TablePaymentTerms, SKPrefixPaymentTerm)}
}

// VehicleSetRepository — organization_vehicle_sets. Uma composição veicular
// junta trator, reboques e condutores num conjunto escolhido de uma vez.
type VehicleSetRepository struct{ OrgEntityRepository }

func NewVehicleSetRepository(db *dynamodb.Client, cfg *config.Config) *VehicleSetRepository {
	return &VehicleSetRepository{newOrgEntityRepository(db, cfg, TableVehicleSets, SKPrefixVehicleSet)}
}

// PaymentTerminalRepository — organization_payment_terminals. Um terminal de
// captura tem CNPJ recebedor e id próprios, invariantes por maquininha.
type PaymentTerminalRepository struct{ OrgEntityRepository }

func NewPaymentTerminalRepository(db *dynamodb.Client, cfg *config.Config) *PaymentTerminalRepository {
	return &PaymentTerminalRepository{newOrgEntityRepository(db, cfg, TablePaymentTerminals, SKPrefixPaymentTerminal)}
}

// TollProviderRepository — organization_toll_providers.
type TollProviderRepository struct{ OrgEntityRepository }

func NewTollProviderRepository(db *dynamodb.Client, cfg *config.Config) *TollProviderRepository {
	return &TollProviderRepository{newOrgEntityRepository(db, cfg, TableTollProviders, SKPrefixTollProvider)}
}

// CargoUnitRepository — organization_cargo_units.
type CargoUnitRepository struct{ OrgEntityRepository }

func NewCargoUnitRepository(db *dynamodb.Client, cfg *config.Config) *CargoUnitRepository {
	return &CargoUnitRepository{newOrgEntityRepository(db, cfg, TableCargoUnits, SKPrefixCargoUnit)}
}

// ImportDeclarationRepository — organization_import_declarations.
type ImportDeclarationRepository struct{ OrgEntityRepository }

func NewImportDeclarationRepository(db *dynamodb.Client, cfg *config.Config) *ImportDeclarationRepository {
	return &ImportDeclarationRepository{newOrgEntityRepository(db, cfg, TableImportDeclarations, SKPrefixImportDI)}
}

// ProductLotRepository — organization_product_lots.
type ProductLotRepository struct{ OrgEntityRepository }

func NewProductLotRepository(db *dynamodb.Client, cfg *config.Config) *ProductLotRepository {
	return &ProductLotRepository{newOrgEntityRepository(db, cfg, TableProductLots, SKPrefixProductLot)}
}

// InsurancePolicyRepository — organization_insurance_policies.
type InsurancePolicyRepository struct{ OrgEntityRepository }

func NewInsurancePolicyRepository(db *dynamodb.Client, cfg *config.Config) *InsurancePolicyRepository {
	return &InsurancePolicyRepository{newOrgEntityRepository(db, cfg, TableInsurancePolicies, SKPrefixInsurance)}
}

// OperationRepository — organization_operations. Uma natureza de operação junta
// os valores que sempre andam juntos por cenário de negócio.
type OperationRepository struct{ OrgEntityRepository }

func NewOperationRepository(db *dynamodb.Client, cfg *config.Config) *OperationRepository {
	return &OperationRepository{newOrgEntityRepository(db, cfg, TableOperations, SKPrefixOperation)}
}

// OperationIsDefaultField é o atributo que marca a operação pré-selecionada.
const OperationIsDefaultField = "is_default"

// ListDefaults devolve as operações marcadas como padrão. Em regime normal há
// no máximo uma; devolve lista porque a exclusividade é garantida na escrita, e
// uma leitura que assumisse "no máximo uma" esconderia o estrago se a garantia
// falhasse.
//
// Filtra em memória de propósito: `is_default` é booleano, e o par de filtro
// tipado do QueryOpts compara strings. Uma organização tem dezenas de
// operações, não milhares — a partição inteira cabe numa página.
func (r *OperationRepository) ListDefaults(ctx context.Context, orgPK string) ([]map[string]types.AttributeValue, error) {
	res, err := r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: SKPrefixOperation, Limit: operationListCap,
	})
	if err != nil {
		return nil, err
	}
	var defaults []map[string]types.AttributeValue
	for _, item := range res.Items {
		if b, ok := item[OperationIsDefaultField].(*types.AttributeValueMemberBOOL); ok && b.Value {
			defaults = append(defaults, item)
		}
	}
	return defaults, nil
}

// operationListCap é o teto de operações lidas ao procurar a padrão. Bem acima
// de qualquer organização real; existe para o loop nunca ser ilimitado.
const operationListCap = 200

// TaxProfileRepository — organization_tax_profiles. A profile is one tax
// treatment applied to a set of CFOPs, shared by many products.
type TaxProfileRepository struct{ OrgEntityRepository }

func NewTaxProfileRepository(db *dynamodb.Client, cfg *config.Config) *TaxProfileRepository {
	return &TaxProfileRepository{newOrgEntityRepository(db, cfg, TableTaxProfiles, SKPrefixTaxProfile)}
}
