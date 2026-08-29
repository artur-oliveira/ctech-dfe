package nfes

// nfce_service.go implements the NFC-e (modelo 65) business logic layer.
// It reuses the shared building blocks (BuildEnviNFe, generateAccessKey,
// resolveProducts, resolveEventContext, event-body builders) with NFC-e-specific
// rules: consumer is optional (pessoa física only), internal operations only,
// restricted CFOPs, QR Code (infNFeSupl), no transport/duplicatas.

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TpEventoCancelamentoSubst is the SEFAZ event for "Cancelamento por Substituição"
// (NFC-e replaced by another NFC-e within the allowed window).
const TpEventoCancelamentoSubst = "110112"

// NfceService manages NFC-e lifecycle.
type NfceService struct {
	orgRepo             *repositories.OrganizationRepository
	certRepo            *repositories.CertificateRepository
	personRepo          *repositories.PersonRepository
	configRepo          *repositories.NfceConfigRepository
	productRepo         *repositories.ProductRepository
	taxProfileRepo      *repositories.TaxProfileRepository
	operationRepo       *repositories.OperationRepository
	paymentTerminalRepo *repositories.PaymentTerminalRepository
	// fuelPumpRepo guarda a leitura do encerrante entre uma venda e a seguinte.
	fuelPumpRepo *repositories.FuelPumpRepository
	nfceRepo     *repositories.NfceRepository
	eventRepo    *repositories.DocumentEventRepository // nfce_events
	clients      *awsclient.Clients
	workerSvc    *services.WorkerService
	billingSvc   *services.BillingService
	documentSvc  *documents.Service
	bucketDocs   string
	tech         TechData
}

func NewNfceService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfceConfigRepository,
	productRepo *repositories.ProductRepository,
	taxProfileRepo *repositories.TaxProfileRepository,
	operationRepo *repositories.OperationRepository,
	paymentTerminalRepo *repositories.PaymentTerminalRepository,
	fuelPumpRepo *repositories.FuelPumpRepository,
	nfceRepo *repositories.NfceRepository,
	eventRepo *repositories.DocumentEventRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
	bucketDocs string,
	tech TechData,
) *NfceService {
	return &NfceService{
		orgRepo: orgRepo, certRepo: certRepo, personRepo: personRepo,
		configRepo: configRepo, productRepo: productRepo, taxProfileRepo: taxProfileRepo, operationRepo: operationRepo,
		paymentTerminalRepo: paymentTerminalRepo, fuelPumpRepo: fuelPumpRepo, nfceRepo: nfceRepo,
		eventRepo: eventRepo, clients: clients, workerSvc: workerSvc, billingSvc: billingSvc, documentSvc: documentSvc,
		bucketDocs: bucketDocs, tech: tech,
	}
}

// GetEnvironment returns the configured NFC-e environment (1=prod, 2=hom).
func (s *NfceService) GetEnvironment(ctx context.Context, orgPK string) (int, error) {
	config, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return 0, err
	}
	if config == nil {
		return 0, problem.BadRequest("configure a NFC-e em Configuração Fiscal antes de usar este recurso")
	}
	return intAttr(config, "environment", 2), nil
}

// GetNFCe searches both prod and hom partitions for the given access key.
func (s *NfceService) GetNFCe(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{EnvProd, EnvHom} {
		pk := fmt.Sprintf("%s#%s", env, orgPK)
		item, err := s.nfceRepo.Get(ctx, pk, accessKey)
		if err != nil {
			return nil, err
		}
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

// ListNFCes resolves the environment, builds the partition key, and delegates to the repo.
func (s *NfceService) ListNFCes(ctx context.Context, orgPK string, opts repositories.NFeListOpts) (*repositories.QueryResult, error) {
	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	pk := fmt.Sprintf("%s#%s", envToPrefix(env), orgPK)
	return s.nfceRepo.ListNFes(ctx, pk, opts)
}

// Cancel marks the NFC-e as cancel_pending and dispatches a 110111 event.
func (s *NfceService) Cancel(ctx context.Context, orgPK, accessKey, justification string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
	nfce, ectx, sefazProtocol, err := s.prepareEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}

	event, err := s.eventRepo.CreateEvent(ctx, accessKey, TpEventoCancelamento, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}

	if _, err := s.nfceRepo.Update(ctx, ectx.pk, accessKey, map[string]any{
		"status":        StatusCancelPending,
		"cancel_reason": justification,
		"updated_at":    repositories.NowStr(),
	}); err != nil {
		return nil, err
	}

	if err := s.publishEvent(ctx, ectx, accessKey, TpEventoCancelamento, sequenceNumber, strAttr(event, "sk"),
		buildCancelBody(accessKey, ectx.docTag, ectx.cnpj, ectx.environment, sefazProtocol, justification, sequenceNumber),
	); err != nil {
		return nil, err
	}

	nfce["status"] = &types.AttributeValueMemberS{Value: StatusCancelPending}
	return nfce, nil
}

// Substitute cancels an NFC-e by substitution (event 110112), referencing the
// access key of the replacement NFC-e (chNFeRef). The replacement must already
// be authorized.
func (s *NfceService) Substitute(ctx context.Context, orgPK, accessKey, substituteKey, justification string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
	if substituteKey == "" || len(substituteKey) != 44 {
		return nil, problem.BadRequest("chave de acesso da NFC-e substituta inválida")
	}
	if substituteKey == accessKey {
		return nil, problem.BadRequest("a NFC-e substituta deve ser diferente da NFC-e cancelada")
	}

	substitute, err := s.GetNFCe(ctx, orgPK, substituteKey)
	if err != nil {
		return nil, err
	}
	if substitute == nil {
		return nil, problem.NotFound("NFC-e substituta não encontrada")
	}
	if strAttr(substitute, "status") != StatusAuthorized {
		return nil, problem.BadRequest("a NFC-e substituta deve estar autorizada")
	}

	nfce, ectx, sefazProtocol, err := s.prepareEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}

	event, err := s.eventRepo.CreateEvent(ctx, accessKey, TpEventoCancelamentoSubst, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}

	if _, err := s.nfceRepo.Update(ctx, ectx.pk, accessKey, map[string]any{
		"status":         StatusCancelPending,
		"cancel_reason":  justification,
		"substitute_key": substituteKey,
		"updated_at":     repositories.NowStr(),
	}); err != nil {
		return nil, err
	}

	if err := s.publishEvent(ctx, ectx, accessKey, TpEventoCancelamentoSubst, sequenceNumber, strAttr(event, "sk"),
		buildSubstituteBody(accessKey, ectx.docTag, ectx.cnpj, ectx.environment, sefazProtocol, substituteKey, justification, sequenceNumber, s.tech.Version),
	); err != nil {
		return nil, err
	}

	nfce["status"] = &types.AttributeValueMemberS{Value: StatusCancelPending}
	return nfce, nil
}

// GetNFCeXML downloads the authorized NFC-e XML from S3.
func (s *NfceService) GetNFCeXML(ctx context.Context, orgPK, accessKey string) ([]byte, error) {
	nfce, err := s.GetNFCe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if nfce == nil {
		return nil, problem.NotFound("NFC-e não encontrada")
	}
	s3Key := strAttr(nfce, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML da NFC-e ainda não disponível")
	}
	return downloadS3(ctx, s.clients, s.bucketDocs, s3Key)
}

// GetEventXML downloads an NFC-e event XML from S3 and returns the event_type.
func (s *NfceService) GetEventXML(ctx context.Context, accessKey, eventSK string) ([]byte, string, error) {
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

// ListNFCeEvents lists all events for an NFC-e.
func (s *NfceService) ListNFCeEvents(ctx context.Context, accessKey string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	return s.eventRepo.GetDocumentEvents(ctx, accessKey, limit, startKey)
}

// --- internal helpers ---

// prepareEvent loads the NFC-e, validates it is authorized, resolves the event
// context and returns the authorization protocol.
func (s *NfceService) prepareEvent(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, *nfeEventContext, string, error) {
	nfce, err := s.GetNFCe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, nil, "", err
	}
	if nfce == nil {
		return nil, nil, "", problem.NotFound("NFC-e não encontrada")
	}
	if strAttr(nfce, "status") != StatusAuthorized {
		return nil, nil, "", problem.BadRequest("apenas NFC-e autorizadas podem ser canceladas")
	}
	sefazProtocol := strAttr(nfce, "sefaz_protocol")
	if sefazProtocol == "" {
		return nil, nil, "", problem.BadRequest("protocolo de autorização não encontrado na NFC-e")
	}
	ectx, err := resolveEventContext(ctx, s.orgRepo, s.certRepo, orgPK, nfce)
	if err != nil {
		return nil, nil, "", err
	}
	return nfce, ectx, sefazProtocol, nil
}

// publishEvent dispatches an NFC-e SEFAZ event to the worker.
func (s *NfceService) publishEvent(ctx context.Context, ectx *nfeEventContext, accessKey, eventType string, seq int, eventSK string, body map[string]any) error {
	return s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ectx.pk, AccessKey: accessKey,
		TableName: "nfces", S3Prefix: "nfce",
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, seq),
		CNPJ:             ectx.cnpj, UF: ectx.emitUF,
		SefazEnvironment: ectx.sefazEnv,
		CertS3Key:        ectx.cert.s3Key, CertPassword: ectx.cert.password,
		DocType: "nfce", SefazService: "RecepcaoEvento",
		Body:            body,
		EventsTableName: aws.String("nfce_events"),
		EventType:       aws.String(eventType),
		SequenceNumber:  &seq,
		EventSK:         aws.String(eventSK),
	})
}

// buildSubstituteBody produces the envEvento for event 110112 (Cancelamento por
// Substituição). chNFeRef is the access key of the replacement NFC-e.
func buildSubstituteBody(accessKey, docTag, cnpj string, environment int, sefazProtocol, substituteKey, justification string, seq int, verAplic string) map[string]any {
	return map[string]any{
		"envEvento": map[string]any{
			"@versao": "1.00",
			"@xmlns":  nfeXMLNS,
			"idLote":  sefazBatchID(),
			"evento": map[string]any{
				"@versao": "1.00",
				"infEvento": map[string]any{
					"@Id":        fmt.Sprintf("ID%s%s%02d", TpEventoCancelamentoSubst, accessKey, seq),
					"cOrgao":     accessKey[:2],
					"tpAmb":      fmt.Sprintf("%d", environment),
					docTag:       cnpj,
					"chNFe":      accessKey,
					"dhEvento":   dhEvento(),
					"tpEvento":   TpEventoCancelamentoSubst,
					"nSeqEvento": fmt.Sprintf("%d", seq),
					"verEvento":  "1.00",
					"detEvento": map[string]any{
						"@versao":     "1.00",
						"@xmlns":      nfeXMLNS,
						"descEvento":  "Cancelamento por substituicao",
						"cOrgaoAutor": accessKey[:2],
						"tpAutor":     "1",
						"verAplic":    verAplic,
						"nProt":       sefazProtocol,
						"xJust":       justification,
						"chNFeRef":    substituteKey,
					},
				},
			},
		},
	}
}
