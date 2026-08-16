// Package nfses implementa a emissão, os eventos e as consultas de NFS-e.
// Diferente de NF-e/CT-e/MDF-e, NFS-e é de competência municipal: não há UF
// autorizadora e o identificador da linha é o id_dps, não a chave de acesso —
// a chave só existe depois da resposta do fisco.
package nfses

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Status do ciclo de vida da NFS-e (spec §3.4).
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusAuthorized = "authorized"
	StatusRejected   = "rejected"
	StatusCancelled  = services.StatusCancelled
	StatusError      = "error"
)

// Roteamento no worker/go-dfe. O provider vem da config da organização e é
// repassado no Body do comando; o worker não decide provider.
const (
	DocTypeNfse  = "nfse"
	S3PrefixNfse = "nfse"
)

// Atributos dos XMLs gravados pelo worker. `xml_s3_key` é o mesmo nome usado
// por nfes/nfces/ctes/mdfes (o documento autorizado); `dps_xml_s3_key` é
// específico da NFS-e, porque aqui o documento que assinamos (a DPS) e o que o
// fisco devolve (a NFS-e) são XMLs distintos.
const (
	attrXMLS3Key    = "xml_s3_key"
	attrDPSXMLS3Key = "dps_xml_s3_key"
)

// Erros reusados pelas rotas. Detalhe em português: chega ao usuário.
var (
	ErrNfseNoOrg         = problem.NotFound("organização não encontrada")
	ErrNfseNotFound      = problem.NotFound("NFS-e não encontrada")
	ErrNfseNoConfig      = problem.BadRequest("configure a NFS-e em Configuração Fiscal antes de emitir")
	ErrNfseNoCertificate = problem.NoCertificate("certificado digital não encontrado")
	ErrNfseNoAbrasf      = problem.BadRequest("configuração ABRASF incompleta: informe o endpoint do município")
)

// NfseService orquestra emissão, eventos e consultas de NFS-e.
type NfseService struct {
	orgRepo      *repositories.OrganizationRepository
	certRepo     *repositories.CertificateRepository
	personRepo   *repositories.PersonRepository
	configRepo   *repositories.NfseConfigRepository
	serviceRepo  *repositories.ServiceRepository
	nfseRepo     *repositories.NfseRepository
	eventRepo    *repositories.DocumentEventRepository
	distRepo     *repositories.NfseDistributionRepository
	workerSvc    *services.WorkerService
	extSvc       *services.ExternalService
	billingSvc   *services.BillingService
	clients      *awsclient.Clients
	cacheBackend cache.Backend
	bucketDocs   string
}

func NewNfseService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfseConfigRepository,
	serviceRepo *repositories.ServiceRepository,
	nfseRepo *repositories.NfseRepository,
	eventRepo *repositories.DocumentEventRepository,
	distRepo *repositories.NfseDistributionRepository,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	billingSvc *services.BillingService,
	clients *awsclient.Clients,
	cacheBackend cache.Backend,
	bucketDocs string,
) *NfseService {
	return &NfseService{
		orgRepo:      orgRepo,
		billingSvc:   billingSvc,
		certRepo:     certRepo,
		personRepo:   personRepo,
		configRepo:   configRepo,
		serviceRepo:  serviceRepo,
		nfseRepo:     nfseRepo,
		eventRepo:    eventRepo,
		distRepo:     distRepo,
		workerSvc:    workerSvc,
		extSvc:       extSvc,
		clients:      clients,
		cacheBackend: cacheBackend,
		bucketDocs:   bucketDocs,
	}
}

// emitPreflight valida organização e config antes de qualquer escrita ou
// chamada externa. Separado de Emit para ser testável sem AWS.
func (s *NfseService) emitPreflight(org, cfg map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	if org == nil {
		return nil, ErrNfseNoOrg
	}
	if cfg == nil {
		return nil, ErrNfseNoConfig
	}
	if strAttr(cfg, "provider") == nfse.ProviderAbrasf204 && mapAttr(cfg, "abrasf") == nil {
		return nil, ErrNfseNoAbrasf
	}
	return cfg, nil
}

// docPK monta a partição do documento: {env}#{org_pk}, o mesmo formato dos
// demais doc types.
func docPK(envPrefix, orgPK string) string {
	return fmt.Sprintf("%s#%s", envPrefix, orgPK)
}

// GetNfse aceita id_dps (SK direta) ou chave de acesso (via GSI) no mesmo
// parâmetro: a UI navega pelos dois e não sabe qual tem em mãos.
func (s *NfseService) GetNfse(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if configItem == nil {
		return nil, ErrNfseNoConfig
	}
	pk := docPK(services.EnvToPrefix(intAttr(configItem, "environment", 2)), orgPK)

	item, err := s.nfseRepo.Get(ctx, pk, id)
	if err != nil {
		return nil, err
	}
	if item != nil {
		return item, nil
	}
	item, err = s.nfseRepo.GetByAccessKey(ctx, pk, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNfseNotFound
	}
	return item, nil
}

// orgPartition resolves the environment-prefixed partition key for org-wide
// queries (listings), where there is no document row to read `pk` from.
func (s *NfseService) orgPartition(ctx context.Context, orgPK string) (string, error) {
	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return "", err
	}
	if configItem == nil {
		return "", ErrNfseNoConfig
	}
	return docPK(services.EnvToPrefix(intAttr(configItem, "environment", 2)), orgPK), nil
}

// ListNfses lista as NFS-e da organização no ambiente configurado.
func (s *NfseService) ListNfses(ctx context.Context, orgPK string, opts repositories.NfseListOpts) (*repositories.QueryResult, error) {
	pk, err := s.orgPartition(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.nfseRepo.ListNfses(ctx, pk, opts)
}

// ListDistributions lista os documentos recebidos pela distribuição do ADN.
// Não passa por DistributionService: aquele serviço é construído em volta do
// DistDFe SOAP (ultNSU/maxNSU, body distDFeInt), e o ADN pagina por NSU
// sequencial em REST — encaixar NFS-e lá exigiria furar a abstração dele.
func (s *NfseService) ListDistributions(ctx context.Context, orgPK string, opts repositories.DistributionListOpts) (*repositories.QueryResult, error) {
	pk, err := s.orgPartition(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.distRepo.ListDistributions(ctx, pk, opts)
}

// GetNfseXML devolve o XML da NFS-e autorizada, gravado pelo worker em
// {org_pk}/nfse/{id_dps}.xml (spec §6).
func (s *NfseService) GetNfseXML(ctx context.Context, orgPK, id string) ([]byte, error) {
	return s.documentXML(ctx, orgPK, id, attrXMLS3Key, "XML da NFS-e ainda não disponível")
}

// GetDPSXML devolve a DPS enviada, gravada em {org_pk}/nfse/{id_dps}/dps.xml.
// É o documento que assinamos — útil para auditoria de uma rejeição.
func (s *NfseService) GetDPSXML(ctx context.Context, orgPK, id string) ([]byte, error) {
	return s.documentXML(ctx, orgPK, id, attrDPSXMLS3Key, "XML da DPS ainda não disponível")
}

func (s *NfseService) documentXML(ctx context.Context, orgPK, id, attr, missing string) ([]byte, error) {
	item, err := s.GetNfse(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	s3Key := strAttr(item, attr)
	if s3Key == "" {
		return nil, problem.NotFound(missing)
	}
	return services.DownloadS3(ctx, s.clients, s.bucketDocs, s3Key)
}

// GetDANFSE é proxy da DANFSE do ADN: o PDF não é gerado nem armazenado por
// nós. Depende do provider — ABRASF 2.04 não tem PDF padronizado.
func (s *NfseService) GetDANFSE(ctx context.Context, orgPK, id string) ([]byte, error) {
	item, err := s.GetNfse(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if p := danfseSupported(strAttr(item, "provider")); p != nil {
		return nil, p
	}
	accessKey := strAttr(item, "access_key")
	if accessKey == "" {
		return nil, ErrNfseNotAuthorized
	}
	result, err := s.callGoDfe(ctx, orgPK, nfse.ServiceDANFSE, map[string]any{
		nfse.BodyKeyAccessKey: accessKey,
	})
	if err != nil {
		return nil, err
	}
	if len(result.PDF) == 0 {
		return nil, problem.NotFound("DANFSE não disponível para esta NFS-e")
	}
	return result.PDF, nil
}
