// Package nfses implementa a emissão, os eventos e as consultas de NFS-e.
// Diferente de NF-e/CT-e/MDF-e, NFS-e é de competência municipal: não há UF
// autorizadora e o identificador da linha é o id_dps, não a chave de acesso —
// a chave só existe depois da resposta do fisco.
package nfses

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

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
	orgRepo     *repositories.OrganizationRepository
	certRepo    *repositories.CertificateRepository
	personRepo  *repositories.PersonRepository
	configRepo  *repositories.NfseConfigRepository
	serviceRepo *repositories.ServiceRepository
	nfseRepo    *repositories.NfseRepository
	workerSvc   *services.WorkerService
}

func NewNfseService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	personRepo *repositories.PersonRepository,
	configRepo *repositories.NfseConfigRepository,
	serviceRepo *repositories.ServiceRepository,
	nfseRepo *repositories.NfseRepository,
	workerSvc *services.WorkerService,
) *NfseService {
	return &NfseService{
		orgRepo:     orgRepo,
		certRepo:    certRepo,
		personRepo:  personRepo,
		configRepo:  configRepo,
		serviceRepo: serviceRepo,
		nfseRepo:    nfseRepo,
		workerSvc:   workerSvc,
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
