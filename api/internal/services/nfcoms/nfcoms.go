// Package nfcoms implements the NFCom (modelo 62, nota fiscal fatura de serviço de comunicação) business logic:
// emission through the transactional outbox, offline contingency, cancellation and document/event retrieval.
// XML signing and SEFAZ communication stay in go-dfe-egress (reached through the worker); this service handles the
// DynamoDB records, the number reservation and the event publishing. Spec: docs/specs/2026-11-15-nfcom-design.md.
package nfcoms

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
	sefazServiceAutorizacao = "NFComRecepcao"
	sefazServiceEvento      = "NFComRecepcaoEvento"

	// DynamoDB / S3 identifiers.
	tableNfcoms    = "nfcoms"
	eventsTableKey = "nfcom"
	s3PrefixNfcom  = "nfcom"

	// TpEventoCancelamento is the only event the issuer sends in v1 (the payment-link events 110300 and 110301
	// are out of scope until their MOC pages are read).
	TpEventoCancelamento = "110111"

	minJustification = 15

	// indIEDest of the customer.
	nfcomIEContribuinte = "1"
	nfcomIEIsento       = "2"
)

// ErrNFComNotFound is returned when an NFCom cannot be found in any partition.
var ErrNFComNotFound = problem.NotFound("NFCom não encontrada")

// dfeCaller is what the offline flow uses of ExternalService: the certificate and the go-dfe-egress call.
type dfeCaller = services.EgressCaller

// NfcomService manages the NFCom lifecycle.
type NfcomService struct {
	orgRepo     *repositories.OrganizationRepository
	certRepo    *repositories.CertificateRepository
	configRepo  *repositories.NfcomConfigRepository
	nfcomRepo   *repositories.NfcomRepository
	eventRepo   *repositories.DocumentEventRepository
	clients     *awsclient.Clients
	workerSvc   *services.WorkerService
	billingSvc  *services.BillingService
	documentSvc *documents.Service
	bucketDocs  string
	tech        TechData
	egress      dfeCaller
}

func NewNfcomService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	configRepo *repositories.NfcomConfigRepository,
	nfcomRepo *repositories.NfcomRepository,
	eventRepo *repositories.DocumentEventRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
	bucketDocs string,
	tech TechData,
	extSvc *services.ExternalService,
) *NfcomService {
	var egress dfeCaller
	if extSvc != nil {
		egress = extSvc
	}
	return &NfcomService{
		orgRepo: orgRepo, certRepo: certRepo, configRepo: configRepo, nfcomRepo: nfcomRepo, eventRepo: eventRepo,
		clients: clients, workerSvc: workerSvc, billingSvc: billingSvc, documentSvc: documentSvc,
		bucketDocs: bucketDocs, tech: tech, egress: egress,
	}
}

// GetEnvironment returns the configured NFCom environment (1=prod, 2=hom).
func (s *NfcomService) GetEnvironment(ctx context.Context, orgPK string) (int, error) {
	cfg, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	if cfg == nil {
		return 0, problem.BadRequest("configure a NFCom em Configuração Fiscal antes de usar este recurso")
	}
	return intAttr(cfg, "environment", 2), nil
}

// GetNFCom searches both prod and hom partitions for the given access key.
func (s *NfcomService) GetNFCom(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{EnvProd, EnvHom} {
		item, err := s.nfcomRepo.Get(ctx, fmt.Sprintf("%s#%s", env, orgPK), accessKey)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

// ListNFComs resolves the environment, builds the partition key and delegates to the repository.
func (s *NfcomService) ListNFComs(ctx context.Context, orgPK string, opts repositories.NFeListOpts) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.nfcomRepo.ListNFes(ctx, fmt.Sprintf("%s#%s", services.EnvToPrefix(env), orgPK), opts)
}

// GetNFComXML returns a direct URL for the authorized NFCom XML in S3.
func (s *NfcomService) GetNFComXML(ctx context.Context, orgPK, accessKey string) (*documents.SignedFileDownload, error) {
	doc, err := s.GetNFCom(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFComNotFound
	}
	s3Key := strAttr(doc, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML da NFCom ainda não disponível")
	}
	return s.documentSvc.SignFile(ctx, s3Key, documents.XMLFilename(accessKey), documents.ContentTypeXML)
}

// ListNFComEvents lists all events of a document. The events table is keyed by the access key alone, so the tenant is
// checked first: a key of another organization is a 404, never a list.
func (s *NfcomService) ListNFComEvents(ctx context.Context, orgPK, accessKey string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	doc, err := s.GetNFCom(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFComNotFound
	}
	return s.eventRepo.GetDocumentEvents(ctx, accessKey, limit, startKey)
}

// GetEventXML returns a direct URL for an event XML after validating the tenant.
func (s *NfcomService) GetEventXML(ctx context.Context, orgPK, accessKey, eventSK string) (*documents.SignedFileDownload, error) {
	doc, err := s.GetNFCom(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFComNotFound
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
