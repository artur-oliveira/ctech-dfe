// Package bpes implements the BP-e (modelo 63, comum, TM e TA) business logic:
// emission through the transactional outbox, offline contingency, cancellation and document/event retrieval.
// XML signing and SEFAZ communication stay in go-dfe-egress (reached through the worker); this service handles the
// DynamoDB records, the number reservation and the event publishing. Spec: docs/specs/2026-11-15-bpe-design.md.
package bpes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
)

const (
	EnvProd = services.EnvProd
	EnvHom  = services.EnvHom

	SefazEnvProd = services.SefazEnvProd
	SefazEnvHom  = services.SefazEnvHom

	// Document lifecycle statuses (the worker writes the same strings).
	StatusPending       = "pending"
	StatusAuthorized    = "authorized"
	StatusRejected      = "rejected"
	StatusCancelled     = "cancelled"
	StatusCancelPending = "cancel_pending"

	// SEFAZ service names (go-dfe routing).
	sefazServiceAutorizacao = "BPeRecepcao"
	// BPeRecepcaoTM é o serviço de recepção do BP-e TM (só SVRS, GZip+Base64).
	sefazServiceAutorizacaoTM = "BPeRecepcaoTM"
	// BPeRecepcaoTA é o serviço de recepção do BP-e TA (só SVRS, GZip+Base64).
	sefazServiceAutorizacaoTA = "BPeRecepcaoTA"
	sefazServiceEvento        = "BPeRecepcaoEvento"

	// DynamoDB / S3 identifiers.
	tableBpes      = "bpes"
	eventsTableKey = "bpe"
	s3PrefixBpe    = "bpe"

	// Eventos do emitente nesta versão: cancelamento, não embarque, alteração de poltrona, excesso de bagagem e, só no TA,
	// fornecimento não realizado (112140). Vinculação de pagamento (110300 e 110301) fica fora.
	TpEventoCancelamento             = "110111"
	TpEventoNaoEmbarque              = "110115"
	TpEventoAlteracaoPoltrona        = "110116"
	TpEventoFornecimentoNaoRealizado = "112140"
	TpEventoExcessoBagagem           = "110117"

	// ModalRodoviario, ModalAquaviario e ModalFerroviario são os valores de modal da API (ide/modal 1, 3 e 4).
	ModalRodoviario  = "rodoviario"
	ModalAquaviario  = "aquaviario"
	ModalFerroviario = "ferroviario"

	minJustification = 15

	// indIEDest of the customer.
	bpeIEContribuinte = "1"
	bpeIEIsento       = "2"
)

// ErrBPeNotFound is returned when an BPe cannot be found in any partition.
var ErrBPeNotFound = problem.NotFound("BP-e não encontrado")

// dfeCaller is what the offline flow uses of ExternalService: the certificate and the go-dfe-egress call.
type dfeCaller = services.EgressCaller

// BpeService manages the BPe lifecycle.
type BpeService struct {
	orgRepo     *repositories.OrganizationRepository
	certRepo    *repositories.CertificateRepository
	configRepo  *repositories.BpeConfigRepository
	bpeRepo     *repositories.BpeRepository
	eventRepo   *repositories.DocumentEventRepository
	clients     *awsclient.Clients
	workerSvc   *services.WorkerService
	billingSvc  *services.BillingService
	documentSvc *documents.Service
	bucketDocs  string
	tech        TechData
	egress      dfeCaller
}

func NewBpeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	configRepo *repositories.BpeConfigRepository,
	bpeRepo *repositories.BpeRepository,
	eventRepo *repositories.DocumentEventRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
	bucketDocs string,
	tech TechData,
	extSvc *services.ExternalService,
) *BpeService {
	var egress dfeCaller
	if extSvc != nil {
		egress = extSvc
	}
	return &BpeService{
		orgRepo: orgRepo, certRepo: certRepo, configRepo: configRepo, bpeRepo: bpeRepo, eventRepo: eventRepo,
		clients: clients, workerSvc: workerSvc, billingSvc: billingSvc, documentSvc: documentSvc,
		bucketDocs: bucketDocs, tech: tech, egress: egress,
	}
}

// GetEnvironment returns the configured BPe environment (1=prod, 2=hom).
func (s *BpeService) GetEnvironment(ctx context.Context, orgPK string) (int, error) {
	cfg, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	if cfg == nil {
		return 0, problem.BadRequest("configure o BP-e em Configuração Fiscal antes de usar este recurso")
	}
	return intAttr(cfg, "environment", 2), nil
}

// GetBPe searches both prod and hom partitions for the given access key.
func (s *BpeService) GetBPe(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{EnvProd, EnvHom} {
		item, err := s.bpeRepo.Get(ctx, fmt.Sprintf("%s#%s", env, orgPK), accessKey)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

// ListBPes resolves the environment, builds the partition key and delegates to the repository.
func (s *BpeService) ListBPes(ctx context.Context, orgPK string, opts repositories.NFeListOpts) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.bpeRepo.ListNFes(ctx, fmt.Sprintf("%s#%s", services.EnvToPrefix(env), orgPK), opts)
}

// GetBPeXML returns a direct URL for the authorized BPe XML in S3.
func (s *BpeService) GetBPeXML(ctx context.Context, orgPK, accessKey string) (*documents.SignedFileDownload, error) {
	doc, err := s.GetBPe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrBPeNotFound
	}
	s3Key := strAttr(doc, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML do BP-e ainda não disponível")
	}
	return s.documentSvc.SignFile(ctx, s3Key, documents.XMLFilename(accessKey), documents.ContentTypeXML)
}

// ListBPeEvents lists all events of a document. The events table is keyed by the access key alone, so the tenant is
// checked first: a key of another organization is a 404, never a list.
func (s *BpeService) ListBPeEvents(ctx context.Context, orgPK, accessKey string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	doc, err := s.GetBPe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrBPeNotFound
	}
	return s.eventRepo.GetDocumentEvents(ctx, accessKey, limit, startKey)
}

// GetEventXML returns a direct URL for an event XML after validating the tenant.
func (s *BpeService) GetEventXML(ctx context.Context, orgPK, accessKey, eventSK string) (*documents.SignedFileDownload, error) {
	doc, err := s.GetBPe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrBPeNotFound
	}
	event, err := s.eventRepo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, problem.NotFound("evento não encontrado")
	}
	s3Key := strAttr(event, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML do evento ainda não disponível")
	}
	return s.documentSvc.SignFile(ctx, s3Key, documents.XMLFilename(strAttr(event, "event_type")+"-"+accessKey), documents.ContentTypeXML)
}

// --- internal helpers ---

func sefazEnvFor(environment int) string {
	if environment == 1 {
		return SefazEnvProd
	}
	return SefazEnvHom
}

func intAttr(item map[string]types.AttributeValue, key string, def int) int {
	v, ok := item[key].(*types.AttributeValueMemberN)
	if !ok {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v.Value, "%d", &n); err != nil {
		return def
	}
	return n
}

// extractEmitUF reads the UF from org.person.addresses[0].state_federation.
func extractEmitUF(org map[string]types.AttributeValue) string {
	return anyStr(services.FirstAddress(personMap(org)), "state_federation")
}

// envPrefixFromPK extracts the environment prefix ("prod"/"hom") from a doc PK.
func envPrefixFromPK(pk string) string {
	for i := 0; i < len(pk); i++ {
		if pk[i] == '#' {
			return pk[:i]
		}
	}
	return pk
}

// authorizerUF é a UF que o egress usa para escolher o autorizador. TM e TA são autorizados só pela SVRS e o evento e a
// consulta seguem o autorizador do documento, não a UF da chave; o comum usa a UF do emitente.
func authorizerUF(variant, emitUF string) string {
	if variant == VariantNormal || variant == "" {
		return emitUF
	}
	return "SVRS" // endpoints.AuthorizerSVRS
}
