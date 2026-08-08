// Package nfes implements the NF-e business logic layer.
// XML building and SEFAZ communication remain in the Python Lambda (Scenario A).
// This service handles: DynamoDB record management, SQS event publishing,
// environment resolution, and document/event retrieval.
package nfes

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	EnvProd = services.EnvProd
	EnvHom  = services.EnvHom

	StatusPending       = "pending"
	StatusAuthorized    = "authorized"
	StatusCancelPending = "cancel_pending"

	TpEventoCancelamento        = "110111"
	TpEventoCCe                 = "110110"
	TpEventoCienciaOperacao     = "210210"
	TpEventoConfirmacaoOperacao = "210200"
	TpEventoDesconhecimento     = "210220"
	TpEventoNaoRealizacao       = "210240"

	descCCe1     = "Carta de Correção"
	descCCe2     = "Carta de Correcao"
	xCondUsoCCe1 = "A Carta de Correção é disciplinada pelo § 1º-A do art. 7º do Convênio S/N, de 15 de dezembro de 1970 e pode ser utilizada para regularização de erro ocorrido na emissão de documento fiscal, desde que o erro não esteja relacionado com: I - as variáveis que determinam o valor do imposto tais como: base de cálculo, alíquota, diferença de preço, quantidade, valor da operação ou da prestação; II - a correção de dados cadastrais que implique mudança do remetente ou do destinatário; III - a data de emissão ou de saída."
	xCondUsoCCe2 = "A Carta de Correcao e disciplinada pelo paragrafo 1o-A do art. 7o do Convenio S/N, de 15 de dezembro de 1970 e pode ser utilizada para regularizacao de erro ocorrido na emissao de documento fiscal, desde que o erro nao esteja relacionado com: I - as variaveis que determinam o valor do imposto tais como: base de calculo, aliquota, diferenca de preco, quantidade, valor da operacao ou da prestacao; II - a correcao de dados cadastrais que implique mudanca do remetente ou do destinatario; III - a data de emissao ou de saida."

	NotIncoming = 0

	SefazEnvProd = services.SefazEnvProd
	SefazEnvHom  = services.SefazEnvHom
)

// ErrNFeNotFound is returned when a NF-e cannot be found in any partition.
var ErrNFeNotFound = problem.NotFound("NF-e não encontrada")

// ErrNFCeNotFound is returned when an NFC-e cannot be found in any partition.
var ErrNFCeNotFound = problem.NotFound("NFC-e não encontrada")

// NfeService manages NF-e lifecycle.
type NfeService struct {
	orgRepo        *repositories.OrganizationRepository
	certRepo       *repositories.CertificateRepository
	personRepo     *repositories.PersonRepository
	configRepo     *repositories.NfeConfigRepository
	productRepo    *repositories.ProductRepository
	taxProfileRepo *repositories.TaxProfileRepository
	operationRepo  *repositories.OperationRepository
	nfeRepo        *repositories.NfeRepository
	eventRepo      *repositories.DocumentEventRepository
	vehicleRepo    *repositories.VehicleRepository
	clients        *awsclient.Clients
	workerSvc      *services.WorkerService
	extSvc         *services.ExternalService
	bucketDocs     string
	tech           TechData
}

func NewNfeService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfeConfigRepository,
	productRepo *repositories.ProductRepository,
	taxProfileRepo *repositories.TaxProfileRepository,
	operationRepo *repositories.OperationRepository,
	nfeRepo *repositories.NfeRepository,
	eventRepo *repositories.DocumentEventRepository,
	vehicleRepo *repositories.VehicleRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	bucketDocs string,
	tech TechData,
) *NfeService {
	return &NfeService{
		orgRepo:        orgRepo,
		certRepo:       certRepo,
		personRepo:     personRepo,
		configRepo:     configRepo,
		productRepo:    productRepo,
		taxProfileRepo: taxProfileRepo,
		operationRepo:  operationRepo,
		nfeRepo:        nfeRepo,
		eventRepo:      eventRepo,
		vehicleRepo:    vehicleRepo,
		clients:        clients,
		workerSvc:      workerSvc,
		extSvc:         extSvc,
		bucketDocs:     bucketDocs,
		tech:           tech,
	}
}

// GetEnvironment returns the configured NF-e environment (1=prod, 2=hom).
func (s *NfeService) GetEnvironment(ctx context.Context, orgPK string) (int, error) {
	config, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	if config == nil {
		return 0, problem.BadRequest("configure a NF-e em Configuração Fiscal antes de usar este recurso")
	}
	return intAttr(config, "environment", 2), nil
}

// GetNFe searches both prod and hom partitions for the given access key.
func (s *NfeService) GetNFe(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{EnvProd, EnvHom} {
		pk := fmt.Sprintf("%s#%s", env, orgPK)
		item, err := s.nfeRepo.Get(ctx, pk, accessKey)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

// ListNFes resolves the environment, builds the partition key, and delegates to the repo.
func (s *NfeService) ListNFes(ctx context.Context, orgPK string, opts repositories.NFeListOpts) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	envPrefix := envToPrefix(env)
	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	return s.nfeRepo.ListNFes(ctx, pk, opts)
}

// Cancel marks the NF-e as cancel_pending and dispatches to the SEFAZ worker.
func (s *NfeService) Cancel(ctx context.Context, orgPK, accessKey, justification string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
	nfe, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if nfe == nil {
		return nil, problem.NotFound("NF-e não encontrada")
	}
	if strAttr(nfe, "status") != StatusAuthorized {
		return nil, problem.BadRequest("apenas NF-e autorizadas podem ser canceladas")
	}
	sefazProtocol := strAttr(nfe, "sefaz_protocol")
	if sefazProtocol == "" {
		return nil, problem.BadRequest("protocolo de autorização não encontrado na NF-e")
	}

	ectx, err := resolveEventContext(ctx, s.orgRepo, s.certRepo, orgPK, nfe)
	if err != nil {
		return nil, err
	}

	event, err := s.eventRepo.CreateEvent(ctx, accessKey, TpEventoCancelamento, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}

	if _, err := s.nfeRepo.Update(ctx, ectx.pk, accessKey, map[string]any{
		"status":        StatusCancelPending,
		"cancel_reason": justification,
		"updated_at":    repositories.NowStr(),
	}); err != nil {
		return nil, err
	}

	eventSK := strAttr(event, "sk")
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ectx.pk, AccessKey: accessKey,
		TableName: "nfes", S3Prefix: "nfe",
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, TpEventoCancelamento, sequenceNumber),
		CNPJ:             ectx.cnpj, UF: ectx.emitUF,
		SefazEnvironment: ectx.sefazEnv,
		CertS3Key:        ectx.cert.s3Key, CertPassword: ectx.cert.password,
		DocType: "nfe", SefazService: "RecepcaoEvento",
		Body:            buildCancelBody(accessKey, ectx.cnpj, ectx.environment, sefazProtocol, justification, sequenceNumber),
		EventsTableName: aws.String("nfe_events"),
		EventType:       aws.String(TpEventoCancelamento),
		SequenceNumber:  &sequenceNumber,
		EventSK:         aws.String(eventSK),
	}); err != nil {
		return nil, err
	}

	nfe["status"] = &types.AttributeValueMemberS{Value: StatusCancelPending}
	return nfe, nil
}

// CorrectionLetter sends a CC-e event.
func (s *NfeService) CorrectionLetter(ctx context.Context, orgPK, accessKey, correctionText string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
	nfe, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if nfe == nil {
		return nil, problem.NotFound("NF-e não encontrada")
	}
	if strAttr(nfe, "status") != StatusAuthorized {
		return nil, problem.BadRequest("carta de correção só pode ser enviada para NF-e autorizadas")
	}

	ectx, err := resolveEventContext(ctx, s.orgRepo, s.certRepo, orgPK, nfe)
	if err != nil {
		return nil, err
	}

	event, err := s.eventRepo.CreateEvent(ctx, accessKey, TpEventoCCe, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}

	eventSK := strAttr(event, "sk")
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ectx.pk, AccessKey: accessKey,
		TableName: "nfes", S3Prefix: "nfe",
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, TpEventoCCe, sequenceNumber),
		CNPJ:             ectx.cnpj, UF: ectx.emitUF,
		SefazEnvironment: ectx.sefazEnv,
		CertS3Key:        ectx.cert.s3Key, CertPassword: ectx.cert.password,
		DocType: "nfe", SefazService: "RecepcaoEvento",
		Body:            buildCCeBody(accessKey, ectx.cnpj, ectx.environment, correctionText, sequenceNumber),
		EventsTableName: aws.String("nfe_events"),
		EventType:       aws.String(TpEventoCCe),
		SequenceNumber:  &sequenceNumber,
		EventSK:         aws.String(eventSK),
	}); err != nil {
		return nil, err
	}

	return nfe, nil
}

// Manifestation sends a manifestation event (destinatário operations).
func (s *NfeService) Manifestation(ctx context.Context, orgPK, accessKey, eventType string, sequenceNumber int, justification *string, userID, userName string) (map[string]types.AttributeValue, error) {
	nfe, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if nfe == nil {
		return nil, problem.NotFound("NF-e não encontrada")
	}

	ectx, err := resolveEventContext(ctx, s.orgRepo, s.certRepo, orgPK, nfe)
	if err != nil {
		return nil, err
	}

	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}

	eventSK := strAttr(event, "sk")
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ectx.pk, AccessKey: accessKey,
		TableName: "nfes", S3Prefix: "nfe",
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, sequenceNumber),
		CNPJ:             ectx.cnpj, UF: "AN", // manifestation always uses UF "AN"
		SefazEnvironment: ectx.sefazEnv,
		CertS3Key:        ectx.cert.s3Key, CertPassword: ectx.cert.password,
		DocType: "nfe", SefazService: "RecepcaoEvento",
		Body:            buildManifestBody(accessKey, ectx.cnpj, ectx.environment, eventType, sequenceNumber, justification),
		EventsTableName: aws.String("nfe_events"),
		EventType:       aws.String(eventType),
		SequenceNumber:  &sequenceNumber,
		EventSK:         aws.String(eventSK),
	}); err != nil {
		return nil, err
	}

	return nfe, nil
}

// GetNFeXML downloads the authorized NF-e XML from S3.
func (s *NfeService) GetNFeXML(ctx context.Context, orgPK, accessKey string) ([]byte, error) {
	nfe, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if nfe == nil {
		return nil, problem.NotFound("NF-e não encontrada")
	}
	s3Key := strAttr(nfe, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML da NF-e ainda não disponível")
	}
	return downloadS3(ctx, s.clients, s.bucketDocs, s3Key)
}

// GetEventXML downloads the event XML from S3 and returns the event_type.
func (s *NfeService) GetEventXML(ctx context.Context, accessKey, eventSK string) ([]byte, string, error) {
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

// ListNFeEvents lists all events for a document.
func (s *NfeService) ListNFeEvents(ctx context.Context, accessKey string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	return s.eventRepo.GetDocumentEvents(ctx, accessKey, limit, startKey)
}

// --- internal helpers ---

type certRef struct {
	s3Key    string
	password string
}

type nfeEventContext struct {
	org         map[string]types.AttributeValue
	cert        certRef
	now         time.Time
	pk          string
	envPrefix   string
	environment int
	cnpj        string
	emitUF      string
	sefazEnv    string
}

// resolveEventContext resolves the org, certificate, environment and partition
// key needed to dispatch a SEFAZ event. Shared by NF-e and NFC-e services.
func resolveEventContext(ctx context.Context, orgRepo *repositories.OrganizationRepository, certRepo *repositories.CertificateRepository, orgPK string, nfe map[string]types.AttributeValue) (*nfeEventContext, error) {
	org, err := orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}

	certs, err := certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, problem.NoCertificate("certificado digital não encontrado")
	}

	pk := strAttr(nfe, "pk")
	envPrefix := strings.SplitN(pk, "#", 2)[0]
	environment := 2
	if envPrefix == EnvProd {
		environment = 1
	}
	sefazEnv := SefazEnvHom
	if environment == 1 {
		sefazEnv = SefazEnvProd
	}

	cnpj := services.StripPKPrefix(orgPK)
	emitUF := extractEmitUF(org)

	cert := certs[0]
	return &nfeEventContext{
		org: org, cert: certRef{
			s3Key:    strAttr(cert, "s3_key"),
			password: strAttr(cert, "password"),
		},
		now: time.Now().UTC(), pk: pk, envPrefix: envPrefix,
		environment: environment, cnpj: cnpj, emitUF: emitUF, sefazEnv: sefazEnv,
	}, nil
}

// downloadS3 fetches an object's bytes. Shared by NF-e and NFC-e services.
func downloadS3(ctx context.Context, clients *awsclient.Clients, bucket, s3Key string) ([]byte, error) {
	return services.DownloadS3(ctx, clients, bucket, s3Key)
}

// strAttr extracts a string from a DynamoDB attribute map.
func strAttr(item map[string]types.AttributeValue, key string) string {
	v, ok := item[key].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

// intAttr extracts a number attribute as int.
func intAttr(item map[string]types.AttributeValue, key string, def int) int {
	v, ok := item[key].(*types.AttributeValueMemberN)
	if !ok {
		return def
	}
	var n int
	_, _ = fmt.Sscanf(v.Value, "%d", &n)
	return n
}

// envToPrefix converts environment int (1=prod, 2=hom) to string prefix.
func envToPrefix(environment int) string { return services.EnvToPrefix(environment) }

// extractEmitUF extracts the UF from the first address in org.person.addresses.
func extractEmitUF(org map[string]types.AttributeValue) string {
	return extractEmitUFFromItem(org)
}

// --- SEFAZ event body builders ---
// Each function produces the full envEvento JSON sent directly to SEFAZ via py-dfe Lambda.

func buildCancelBody(accessKey, cnpj string, environment int, sefazProtocol, justification string, seq int) map[string]any {
	return map[string]any{
		"envEvento": map[string]any{
			"@versao": "1.00",
			"@xmlns":  "http://www.portalfiscal.inf.br/nfe",
			"idLote":  sefazBatchID(),
			"evento": map[string]any{
				"@versao": "1.00",
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", TpEventoCancelamento, accessKey, seq),
					"cOrgao":     accessKey[:2],
					"tpAmb":      fmt.Sprintf("%d", environment),
					"CNPJ":       cnpj,
					"chNFe":      accessKey,
					"dhEvento":   dhEvento(),
					"tpEvento":   TpEventoCancelamento,
					"nSeqEvento": fmt.Sprintf("%d", seq),
					"verEvento":  "1.00",
					"detEvento": map[string]any{
						"@versao":    "1.00",
						"@xmlns":     "http://www.portalfiscal.inf.br/nfe",
						"descEvento": "Cancelamento",
						"nProt":      sefazProtocol,
						"xJust":      justification,
					},
				},
			},
		},
	}
}

func buildCCeBody(accessKey, cnpj string, environment int, correctionText string, seq int) map[string]any {
	uf := accessKey[:2]
	var dsc string
	var condition string
	if uf == services.UFCode["MT"] {
		dsc = descCCe2
		condition = xCondUsoCCe2
	} else {
		dsc = descCCe1
		condition = xCondUsoCCe1
	}
	return map[string]any{
		"envEvento": map[string]any{
			"@versao": "1.00",
			"@xmlns":  "http://www.portalfiscal.inf.br/nfe",
			"idLote":  sefazBatchID(),
			"evento": map[string]any{
				"@versao": "1.00",
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", TpEventoCCe, accessKey, seq),
					"cOrgao":     uf,
					"tpAmb":      fmt.Sprintf("%d", environment),
					"CNPJ":       cnpj,
					"chNFe":      accessKey,
					"dhEvento":   dhEvento(),
					"tpEvento":   TpEventoCCe,
					"nSeqEvento": fmt.Sprintf("%d", seq),
					"verEvento":  "1.00",
					"detEvento": map[string]any{
						"@versao":    "1.00",
						"@xmlns":     "http://www.portalfiscal.inf.br/nfe",
						"descEvento": dsc,
						"xCorrecao":  correctionText,
						"xCondUso":   condition,
					},
				},
			},
		},
	}
}

var manifestDescEvento = map[string]string{
	TpEventoConfirmacaoOperacao: "Confirmacao da Operacao",
	TpEventoCienciaOperacao:     "Ciencia da Operacao",
	TpEventoDesconhecimento:     "Desconhecimento da Operacao",
	TpEventoNaoRealizacao:       "Operacao nao Realizada",
}

func buildManifestBody(accessKey, cnpj string, environment int, eventType string, seq int, justification *string) map[string]any {
	det := map[string]any{
		"@versao":    "1.00",
		"@xmlns":     "http://www.portalfiscal.inf.br/nfe",
		"descEvento": manifestDescEvento[eventType],
	}
	if justification != nil {
		det["xJust"] = *justification
	}
	return map[string]any{
		"envEvento": map[string]any{
			"@versao": "1.00",
			"@xmlns":  "http://www.portalfiscal.inf.br/nfe",
			"idLote":  sefazBatchID(),
			"evento": map[string]any{
				"@versao": "1.00",
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", eventType, accessKey, seq),
					"cOrgao":     "91",
					"tpAmb":      fmt.Sprintf("%d", environment),
					"CNPJ":       cnpj,
					"chNFe":      accessKey,
					"dhEvento":   dhEvento(),
					"tpEvento":   eventType,
					"nSeqEvento": fmt.Sprintf("%d", seq),
					"verEvento":  "1.00",
					"detEvento":  det,
				},
			},
		},
	}
}

func sefazBatchID() string {
	return fmt.Sprintf("%d", rand.Int63n(999_999_999_999_999)+1)
}

func dhEvento() string {
	brt := time.FixedZone("BRT", -3*60*60)
	return time.Now().In(brt).Format("2006-01-02T15:04:05-07:00")
}
