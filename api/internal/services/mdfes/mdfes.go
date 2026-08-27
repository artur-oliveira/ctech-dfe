// Package mdfes implements the MDF-e (Manifesto Eletrônico de Documentos Fiscais)
// business logic layer. XML signing and SEFAZ communication remain in the Python
// Lambda (py-dfe); this service handles DynamoDB records, cargo resolution from
// referenced documents, SQS event publishing, and document/event retrieval.
package mdfes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	EnvProd = services.EnvProd
	EnvHom  = services.EnvHom

	SefazEnvProd = services.SefazEnvProd
	SefazEnvHom  = services.SefazEnvHom

	// Document lifecycle statuses.
	StatusPending       = "pending"
	StatusAuthorized    = "authorized"
	StatusRejected      = "rejected"
	StatusCancelled     = "cancelled"
	StatusCancelPending = "cancel_pending"
	StatusClosePending  = "close_pending"
	StatusClosed        = "closed"

	// Referenced document types.
	docTypeNFe = "nfe"
	docTypeCTe = "cte"

	// Transport modals (API value). Only rodoviário is enabled for emission in
	// the MVP; the others are modelled and dispatched but gated at Emit.
	ModalRodoviario  = "rodoviario"
	ModalAereo       = "aereo"
	ModalAquaviario  = "aquaviario"
	ModalFerroviario = "ferroviario"

	// MDF-e modal codes (campo ide/modal): 1-Rodoviário, 2-Aéreo, 3-Aquaviário,
	// 4-Ferroviário.
	modalCodeRodoviario  = "1"
	modalCodeAereo       = "2"
	modalCodeAquaviario  = "3"
	modalCodeFerroviario = "4"

	// layoutDateTimeTZ is the SEFAZ dhEmi/dhIniViagem layout (AAAA-MM-DDTHH:MM:SS±TZD).
	layoutDateTimeTZ = "2006-01-02T15:04:05-07:00"

	// SEFAZ service names (py-dfe routing).
	sefazServiceAutorizacao = "MDFeRecepcaoSinc"
	sefazServiceEvento      = "MDFeRecepcaoEvento"

	// DynamoDB / S3 identifiers.
	tableMdfes     = "mdfes"
	eventsTableKey = "mdfe"
	s3PrefixMdfe   = "mdfe"

	// tot/cUnid: unidade de medida da carga. "01" = KG.
	cUnidKG = "01"

	// Default tpCarga (prodPred) when not supplied: "05" = Carga Geral.
	defaultTpCarga = "05"

	// MDF-e event type codes (eventoMDFe_v3.00).
	TpEventoCancelamento    = "110111"
	TpEventoEncerramento    = "110112"
	TpEventoInclusaoCond    = "110114"
	TpEventoInclusaoDFe     = "110115"
	TpEventoPagamentoOper   = "110116"
	TpEventoConfirmaServico = "110117"
	TpEventoAlteracaoPagto  = "110118"

	// indEncPorTerceiro (evEncMDFe): único valor aceito pelo XSD — encerramento
	// feito por terceiro. Ausente significa encerrado pelo emitente.
	indEncPorTerceiroSim = "1"

	// categCombVeic (valePed) — categoria da combinação veicular, derivada do
	// número de reboques do próprio manifesto.
	categCombCaminhao             = "02" // caminhão simples
	categCombCaminhaoReboque      = "04" // caminhão + 1 reboque
	categCombCaminhaoDoisReboques = "06" // caminhão + 2 reboques
	categCombCaminhaoTresReboques = "07" // caminhão + 3 ou mais reboques

	// Campos do CSRT na configuração fiscal (organization_mdfe_configs).
	csrtIDField = "csrt_id"
	csrtField   = "csrt"
)

// modalCodes maps an API modal value to its ide/modal code.
var modalCodes = map[string]string{
	ModalRodoviario:  modalCodeRodoviario,
	ModalAereo:       modalCodeAereo,
	ModalAquaviario:  modalCodeAquaviario,
	ModalFerroviario: modalCodeFerroviario,
}

// enabledModals are the modals the Emit service accepts. Rodoviário is the only
// one wired end-to-end (vehicle/owner data + DAMDFE); the remaining modals are
// modelled in the builder but not yet exposed for emission.
var enabledModals = map[string]bool{
	ModalRodoviario: true,
}

// ErrMDFeNotFound is returned when an MDF-e cannot be found in any partition.
var ErrMDFeNotFound = problem.NotFound("MDF-e não encontrado")

// errInvalidDocXML is returned when a referenced document XML cannot be parsed.
var errInvalidDocXML = problem.BadRequest("XML do documento referenciado é inválido ou incompleto")

// MdfeService manages the MDF-e lifecycle.
type MdfeService struct {
	orgRepo     *repositories.OrganizationRepository
	certRepo    *repositories.CertificateRepository
	configRepo  *repositories.MdfeConfigRepository
	mdfeRepo    *repositories.MdfeRepository
	nfeRepo     *repositories.NfeRepository
	cteRepo     *repositories.CteRepository
	eventRepo   *repositories.DocumentEventRepository
	vehicleRepo *repositories.VehicleRepository
	// personRepo resolve os CPFs dos condutores de uma composição veicular.
	personRepo       *repositories.PersonRepository
	vehicleSetRepo   *repositories.VehicleSetRepository
	tollProviderRepo *repositories.TollProviderRepository
	// cargoUnitRepo traz as unidades de transporte e de carga do cadastro.
	cargoUnitRepo *repositories.CargoUnitRepository
	// productRepo reencontra no cadastro o produto que a NF-e referenciada
	// declarou, para derivar dele o grupo peri (produto perigoso).
	productRepo *repositories.ProductRepository
	clients     *awsclient.Clients
	workerSvc   *services.WorkerService
	billingSvc  *services.BillingService
	bucketDocs  string
	tech        TechData
}

// TechData carries the technical-responsible (infRespTec) information.
type TechData struct {
	CNPJ    string
	Name    string
	Email   string
	Phone   string
	Version string
}

func NewMdfeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	configRepo *repositories.MdfeConfigRepository,
	mdfeRepo *repositories.MdfeRepository,
	nfeRepo *repositories.NfeRepository,
	cteRepo *repositories.CteRepository,
	eventRepo *repositories.DocumentEventRepository,
	vehicleRepo *repositories.VehicleRepository,
	personRepo *repositories.PersonRepository,
	vehicleSetRepo *repositories.VehicleSetRepository,
	tollProviderRepo *repositories.TollProviderRepository,
	productRepo *repositories.ProductRepository,
	cargoUnitRepo *repositories.CargoUnitRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	billingSvc *services.BillingService,
	bucketDocs string,
	tech TechData,
) *MdfeService {
	return &MdfeService{
		orgRepo:          orgRepo,
		billingSvc:       billingSvc,
		certRepo:         certRepo,
		configRepo:       configRepo,
		mdfeRepo:         mdfeRepo,
		nfeRepo:          nfeRepo,
		cteRepo:          cteRepo,
		eventRepo:        eventRepo,
		vehicleRepo:      vehicleRepo,
		personRepo:       personRepo,
		vehicleSetRepo:   vehicleSetRepo,
		tollProviderRepo: tollProviderRepo,
		productRepo:      productRepo,
		cargoUnitRepo:    cargoUnitRepo,
		clients:          clients,
		workerSvc:        workerSvc,
		bucketDocs:       bucketDocs,
		tech:             tech,
	}
}

// GetEnvironment returns the configured MDF-e environment (1=prod, 2=hom).
func (s *MdfeService) GetEnvironment(ctx context.Context, orgPK string) (int, error) {
	cfg, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	if cfg == nil {
		return 0, problem.BadRequest("configure o MDF-e em Configuração Fiscal antes de usar este recurso")
	}
	return intAttr(cfg, "environment", 2), nil
}

// GetMDFe searches both prod and hom partitions for the given access key.
func (s *MdfeService) GetMDFe(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{EnvProd, EnvHom} {
		pk := fmt.Sprintf("%s#%s", env, orgPK)
		item, err := s.mdfeRepo.Get(ctx, pk, accessKey)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

// ListMDFes resolves the environment, builds the partition key, and delegates to the repo.
func (s *MdfeService) ListMDFes(ctx context.Context, orgPK string, opts repositories.NFeListOpts) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	pk := fmt.Sprintf("%s#%s", envToPrefix(env), orgPK)
	return s.mdfeRepo.ListNFes(ctx, pk, opts)
}

// GetMDFeXML downloads the authorized MDF-e XML from S3.
func (s *MdfeService) GetMDFeXML(ctx context.Context, orgPK, accessKey string) ([]byte, error) {
	mdfe, err := s.GetMDFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if mdfe == nil {
		return nil, ErrMDFeNotFound
	}
	s3Key := strAttr(mdfe, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML do MDF-e ainda não disponível")
	}
	return downloadS3(ctx, s.clients, s.bucketDocs, s3Key)
}

// ListMDFeEvents lists all events for a document.
func (s *MdfeService) ListMDFeEvents(ctx context.Context, accessKey string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	return s.eventRepo.GetDocumentEvents(ctx, accessKey, limit, startKey)
}

// GetEventXML downloads the event XML from S3 and returns the event_type.
func (s *MdfeService) GetEventXML(ctx context.Context, accessKey, eventSK string) ([]byte, string, error) {
	event, err := s.eventRepo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return nil, "", err
	}
	if event == nil {
		return nil, "", problem.NotFound("evento não encontrado")
	}
	s3Key := strAttr(event, "xml_s3_key")
	if s3Key == "" {
		return nil, "", problem.NotFound("XML do evento ainda não disponível")
	}
	data, err := downloadS3(ctx, s.clients, s.bucketDocs, s3Key)
	return data, strAttr(event, "event_type"), err
}

// --- internal helpers ---

func envToPrefix(environment int) string { return services.EnvToPrefix(environment) }

func sefazEnvFor(environment int) string {
	if environment == 1 {
		return SefazEnvProd
	}
	return SefazEnvHom
}

// indReentregaSim é o único valor aceito por indReentrega no leiaute.
const indReentregaSim = "1"

func strAttr(item map[string]types.AttributeValue, key string) string {
	v, ok := item[key].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

func intAttr(item map[string]types.AttributeValue, key string, def int) int {
	v, ok := item[key].(*types.AttributeValueMemberN)
	if !ok {
		return def
	}
	var n int
	fmt.Sscanf(v.Value, "%d", &n)
	return n
}

// extractEmitUF reads the UF from org.person.addresses[0].state_federation.
func extractEmitUF(org map[string]types.AttributeValue) string {
	personAttr, ok := org["person"].(*types.AttributeValueMemberM)
	if !ok {
		return ""
	}
	if addrsAttr, ok := personAttr.Value["addresses"].(*types.AttributeValueMemberL); ok && len(addrsAttr.Value) > 0 {
		if addr, ok := addrsAttr.Value[0].(*types.AttributeValueMemberM); ok {
			if uf, ok := addr.Value["state_federation"].(*types.AttributeValueMemberS); ok {
				return uf.Value
			}
		}
	}
	if addrAttr, ok := personAttr.Value["address"].(*types.AttributeValueMemberM); ok {
		if uf, ok := addrAttr.Value["state_federation"].(*types.AttributeValueMemberS); ok {
			return uf.Value
		}
	}
	return ""
}

// dhEvento returns the current time in BRT with offset, as required by SEFAZ events.
func dhEvento() string {
	brt := time.FixedZone("BRT", -3*60*60)
	return time.Now().In(brt).Format("2006-01-02T15:04:05-07:00")
}

// envPrefixFromPK extracts the environment prefix ("prod"/"hom") from a doc PK.
func envPrefixFromPK(pk string) string {
	return strings.SplitN(pk, "#", 2)[0]
}

// downloadS3 fetches an object's bytes from the documents bucket.
func downloadS3(ctx context.Context, clients *awsclient.Clients, bucket, s3Key string) ([]byte, error) {
	return services.DownloadS3(ctx, clients, bucket, s3Key)
}
