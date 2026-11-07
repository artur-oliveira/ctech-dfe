// Package ctes implementa o CT-e rodoviário (modelo 57, tpCTe 0, tpServ 0, modal 01) emitido a partir de
// NF-e já existentes: derivação das notas (Derive), emissão assíncrona pelo outbox (Emit), lista e detalhe.
// Spec: docs/specs/2026-11-05-cte-emissao-rodoviaria.md.
package ctes

import (
	"context"
	"errors"
	"fmt"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	// StatusPending é o registro recém-emitido, à espera do worker.
	StatusPending    = "pending"
	statusAuthorized = "authorized"

	// Roteamento do worker (DocType "cte", worker cte-emission) e armazenamento.
	tableCtes               = "ctes"
	s3PrefixCte             = "cte"
	sefazServiceAutorizacao = "CTeRecepcaoSinc"

	// layoutDateTimeTZ é o dhEmi da SEFAZ (AAAA-MM-DDTHH:MM:SS±TZD).
	layoutDateTimeTZ = "2006-01-02T15:04:05-07:00"

	// MaxNFeKeys é o máximo de notas por CT-e aceito pelo derive e pela emissão.
	MaxNFeKeys = 50

	// modelNFe é o único modelo de nota aceito como documento transportado.
	modelNFe = services.ModelNFe

	// Campo do RNTRC da transportadora na configuração do MDF-e (organization_mdfe_configs).
	rntrcField = "rntrc"
)

// ErrCTeNotFound: chave fora do cadastro de CT-e da organização.
var ErrCTeNotFound = problem.NotFound("CT-e não encontrado")

// TechData carrega o responsável técnico (infRespTec).
type TechData struct {
	CNPJ    string
	Name    string
	Email   string
	Phone   string
	Version string
}

// CteService cuida do ciclo de vida do CT-e.
type CteService struct {
	orgRepo        *repositories.OrganizationRepository
	certRepo       *repositories.CertificateRepository
	configRepo     *repositories.CteConfigRepository
	mdfeConfigRepo *repositories.MdfeConfigRepository
	cteRepo        *repositories.CteRepository
	eventRepo      *repositories.DocumentEventRepository
	documentSvc    *documents.Service
	nfeRepo        *repositories.NfeRepository
	personRepo     *repositories.PersonRepository
	clients        *awsclient.Clients
	workerSvc      *services.WorkerService
	billingSvc     *services.BillingService
	bucketDocs     string
	tech           TechData
	// egress chama o go-dfe-egress nas consultas síncronas (situação).
	egress dfeCaller
}

func NewCteService(
	orgRepo *repositories.OrganizationRepository,
	certRepo *repositories.CertificateRepository,
	configRepo *repositories.CteConfigRepository,
	mdfeConfigRepo *repositories.MdfeConfigRepository,
	cteRepo *repositories.CteRepository,
	eventRepo *repositories.DocumentEventRepository,
	nfeRepo *repositories.NfeRepository,
	personRepo *repositories.PersonRepository,
	clients *awsclient.Clients,
	workerSvc *services.WorkerService,
	billingSvc *services.BillingService,
	documentSvc *documents.Service,
	bucketDocs string,
	tech TechData,
	extSvc *services.ExternalService,
) *CteService {
	var egress dfeCaller
	if extSvc != nil {
		egress = extSvc
	}
	return &CteService{
		orgRepo: orgRepo, certRepo: certRepo, configRepo: configRepo, mdfeConfigRepo: mdfeConfigRepo,
		cteRepo: cteRepo, eventRepo: eventRepo, documentSvc: documentSvc, nfeRepo: nfeRepo, personRepo: personRepo, clients: clients,
		workerSvc: workerSvc, billingSvc: billingSvc, bucketDocs: bucketDocs, tech: tech, egress: egress,
	}
}

// config devolve a configuração do CT-e e o ambiente (1 produção, 2 homologação).
func (s *CteService) config(ctx context.Context, orgPK string) (map[string]types.AttributeValue, int, error) {
	cfg, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, 0, err
	}
	if cfg == nil {
		return nil, 0, problem.UnprocessableEntity("configure o CT-e em Configuração Fiscal antes de usar este recurso").WithCode("cte_config_required")
	}
	return cfg, intAttr(cfg, "environment", 2), nil
}

// GetCTe procura a chave nas partições de produção e de homologação.
func (s *CteService) GetCTe(ctx context.Context, orgPK, accessKey string) (map[string]types.AttributeValue, error) {
	for _, env := range []string{services.EnvProd, services.EnvHom} {
		item, err := s.cteRepo.Get(ctx, env+"#"+orgPK, accessKey)
		if err != nil || item != nil {
			return item, err
		}
	}
	return nil, nil
}

// ListCTes lista os CT-e do ambiente configurado (emitidos e recebidos, como as NF-e).
func (s *CteService) ListCTes(ctx context.Context, orgPK string, opts repositories.NFeListOpts) (*repositories.QueryResult, error) {
	_, env, err := s.config(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return s.cteRepo.ListNFes(ctx, services.EnvToPrefix(env)+"#"+orgPK, opts)
}

// loadedNote é uma chave pedida com o que o cadastro e o XML dizem dela.
type loadedNote struct {
	key  string
	item map[string]types.AttributeValue // nil: fora do cadastro
	nfe  *nfexml.NFe                     // nil: sem XML legível
}

// loadNotes lê cada NF-e do cadastro (emitidas e recebidas) e o XML autorizado no S3.
func (s *CteService) loadNotes(ctx context.Context, pk string, keys []string) ([]loadedNote, error) {
	out := make([]loadedNote, 0, len(keys))
	for _, k := range keys {
		n := loadedNote{key: k}
		item, err := s.nfeRepo.Get(ctx, pk, k)
		if err != nil {
			return nil, err
		}
		n.item = item
		if s3Key := strAttr(item, "xml_s3_key"); s3Key != "" {
			data, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, s3Key)
			if err != nil {
				return nil, err
			}
			nfe, err := nfexml.ParseNFe(data)
			if err != nil && !errors.Is(err, nfexml.ErrNotNFe) {
				return nil, problem.UnprocessableEntity("XML da NF-e " + k + " é inválido").WithCode("note_xml_invalid")
			}
			n.nfe = nfe
		}
		out = append(out, n)
	}
	return out, nil
}

// person lê uma pessoa do cadastro. id é o SK do cadastro (CNPJ_..., CPF_...) ou o documento.
func (s *CteService) person(ctx context.Context, orgPK, id string) (sk string, item map[string]any, err error) {
	sk = personSK(id)
	if sk == "" {
		return "", nil, nil
	}
	av, err := s.personRepo.Get(ctx, orgPK, sk)
	if err != nil || av == nil {
		return sk, nil, err
	}
	if err := attributevalue.UnmarshalMap(av, &item); err != nil {
		return sk, nil, fmt.Errorf("ctes: pessoa %s: %w", sk, err)
	}
	return sk, item, nil
}
