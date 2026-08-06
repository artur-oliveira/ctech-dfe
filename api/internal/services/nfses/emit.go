package nfses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

// Tipo de inscrição federal na posição 8 do idDPS (TSIdDPS).
const (
	tpInscCPF  = "1"
	tpInscCNPJ = "2"
)

// NfseEmitBody é o corpo de POST /v1.0/nfses. Diferente da NF-e, NFS-e tem
// UM serviço por documento — TCServ não é lista.
//
// As chaves JSON são em inglês, como no resto da API; as exceções são os
// códigos do leiaute do DPS (`tp_emit`, `motivo_emis_ti`, `ch_nfse_rej`,
// `c_trib_mun`), que seguem o nome normativo do campo — a mesma regra que a
// NF-e aplica em `tp_nf`/`fin_nfe`/`nat_op`.
type NfseEmitBody struct {
	TpEmit       int    `json:"tp_emit" validate:"required,oneof=1 2 3"`
	MotivoEmisTI int    `json:"motivo_emis_ti" validate:"omitempty,oneof=1 2 3 4"`
	ChNFSeRej    string `json:"ch_nfse_rej" validate:"omitempty,len=50,numeric"`
	Competence   string `json:"competence" validate:"required,datebr"`

	// Quando tp_emit != 1 o prestador é uma pessoa do cadastro.
	ProviderPersonID *string `json:"provider_person_id" validate:"omitempty"`
	CustomerID       *string `json:"customer_id" validate:"omitempty"`
	IntermediaryID   *string `json:"intermediary_id" validate:"omitempty"`

	Service NfseServiceItem `json:"service" validate:"required"`

	// Substituição de NFS-e já emitida (gera o evento 105102 no fisco).
	SubstitutesAccessKey *string `json:"substitutes_access_key" validate:"omitempty,len=50,numeric"`
	SubstitutesReason    *string `json:"substitutes_reason" validate:"omitempty,max=2"`

	AdditionalInfo *string `json:"additional_info" validate:"omitempty,max=2000"`
}

// NfseServiceItem referencia o catálogo e permite sobrescrever valor,
// alíquota e descrição por emissão — o mesmo padrão de resolveProducts.
type NfseServiceItem struct {
	ServiceID   string  `json:"service_id" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Value       *string `json:"value" validate:"omitempty,money"`
	TaxRate     *string `json:"tax_rate" validate:"omitempty,money"`
	Quantity    *string `json:"quantity" validate:"omitempty,money"`
	CTribMun    *string `json:"c_trib_mun" validate:"omitempty,max=20"`
}

// BuildIDDPS delega para a regra normativa que vive no go-dfe. NÃO
// reimplemente: a api e o go-dfe TÊM que produzir o mesmo identificador,
// porque um é a SK da linha e o outro é o Id assinado no infDPS.
func BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string {
	return nacional.BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie, nDPS)
}

// buildWorkerBody serializa o documento no formato que nfse.Dispatch lê
// (go-dfe/nfse/dispatch.go): as chaves "provider" e "document".
func buildWorkerBody(provider string, doc nfse.Document) (map[string]any, error) {
	docMap, err := documentAsMap(doc)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		nfse.BodyKeyProvider: provider,
		nfse.BodyKeyDocument: docMap,
	}, nil
}

// documentAsMap devolve o modelo neutro como mapa JSON — o mesmo objeto que
// vai no Body do comando do worker e no atributo payload da linha.
func documentAsMap(doc nfse.Document) (map[string]any, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, problem.InternalServer("failed to encode nfse document")
	}
	var docMap map[string]any
	if err := json.Unmarshal(raw, &docMap); err != nil {
		return nil, problem.InternalServer("failed to decode nfse document")
	}
	return docMap, nil
}

// Emit espelha NfeService.Emit: carrega o contexto de cadastro, monta o
// documento neutro, calcula o id_dps e grava documento + comando do worker
// numa transação única que também reserva o número da DPS.
func (s *NfseService) Emit(ctx context.Context, orgPK string, req NfseEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	orgItem, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if _, err := s.emitPreflight(orgItem, configItem); err != nil {
		return nil, err
	}

	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, ErrNfseNoCertificate
	}
	cert := certs[0]

	environment := intAttr(configItem, "environment", 2)
	envPrefix := services.EnvToPrefix(environment)
	serie := strAttr(configItem, "serie")
	currentNumber := intAttr(configItem, fmt.Sprintf("%s_current_number", envPrefix), 0)

	// tp_emit 1: a própria organização é o prestador. 2 e 3: o prestador é
	// pessoa do cadastro, com o próprio regime tributário (spec §3.2).
	prestadorItem := orgItem
	if req.TpEmit != tpEmitPrestador {
		if req.ProviderPersonID == nil {
			return nil, problem.BadRequest("provider_person_id é obrigatório quando tp_emit != 1")
		}
		prestadorItem, err = s.resolvePerson(ctx, orgPK, req.ProviderPersonID)
		if err != nil {
			return nil, err
		}
	}

	tomadorItem, err := s.resolvePerson(ctx, orgPK, req.CustomerID)
	if err != nil {
		return nil, err
	}
	intermItem, err := s.resolvePerson(ctx, orgPK, req.IntermediaryID)
	if err != nil {
		return nil, err
	}

	serviceItem, err := s.serviceRepo.Get(ctx, orgPK, req.Service.ServiceID)
	if err != nil {
		return nil, err
	}
	if serviceItem == nil {
		return nil, problem.NotFound("serviço não encontrado no catálogo: " + req.Service.ServiceID)
	}

	doc, err := buildDocument(documentInput{
		Org: orgItem, Config: configItem, Prestador: prestadorItem,
		Tomador: tomadorItem, Intermediario: intermItem, Service: serviceItem,
		Body: req, Serie: serie, Numero: currentNumber, Environment: environment,
	})
	if err != nil {
		return nil, err
	}

	tpInsc, inscFederal := tpInscCNPJ, doc.Prestador.CNPJ
	if doc.Prestador.CPF != "" {
		tpInsc, inscFederal = tpInscCPF, doc.Prestador.CPF
	}
	idDPS := BuildIDDPS(doc.CLocEmi, tpInsc, inscFederal, serie, currentNumber)

	provider := strAttr(configItem, "provider")
	workerBody, err := buildWorkerBody(provider, doc)
	if err != nil {
		return nil, err
	}
	// O mesmo mapa serve ao comando do worker e ao atributo payload da linha.
	docMap := workerBody[nfse.BodyKeyDocument]

	now := time.Now()
	pk := docPK(envPrefix, orgPK)
	// access_key NÃO é gravada aqui: só existe na resposta do fisco (spec
	// §3.4). Gravar vazio poluiria a GSI access-key-index.
	record := map[string]any{
		"pk":            pk,
		"sk":            idDPS,
		"provider":      provider,
		"status":        StatusPending,
		"tp_emit":       req.TpEmit,
		"serie":         serie,
		"number":        currentNumber,
		"competence":    req.Competence,
		"c_loc_emi":     doc.CLocEmi,
		"year":          now.Year(),
		"month":         int(now.Month()),
		"emit_cpf_cnpj": services.StripPKPrefix(orgPK),
		"emit_name":     strAttr(orgItem, "name"),
		"dest_name":     strAttr(tomadorItem, "name"),
		"dest_cpf_cnpj": strAttr(tomadorItem, "cpf_cnpj"),
		"total":         doc.Valores.VServPrest.VServ,
		"payload":       docMap,
		"created_at":    now.UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_name":     userName,
	}
	if req.MotivoEmisTI != 0 {
		record["c_motivo_emis_ti"] = req.MotivoEmisTI
	}

	encoded, err := repositories.EncodeItem(record)
	if err != nil {
		return nil, problem.InternalServer("failed to encode NFS-e record")
	}

	sefazEnv := services.SefazEnvHom
	if environment == 1 {
		sefazEnv = services.SefazEnvProd
	}
	outboxTx, operationID, err := s.workerSvc.BuildOutboxTx(services.WorkerMessage{
		DocPK:            pk,
		AccessKey:        idDPS, // identificador da linha; NFS-e usa id_dps
		TableName:        repositories.TableNfses,
		S3Prefix:         S3PrefixNfse,
		ExpectedFileName: idDPS,
		CNPJ:             services.StripPKPrefix(orgPK),
		UF:               "", // competência municipal: não há UF autorizadora
		SefazEnvironment: sefazEnv,
		CertS3Key:        strAttr(cert, "s3_key"),
		CertPassword:     strAttr(cert, "password"),
		DocType:          DocTypeNfse,
		SefazService:     nfse.ServiceRecepcao,
		Body:             workerBody,
	})
	if err != nil {
		return nil, err
	}
	encoded["operation_id"] = &types.AttributeValueMemberS{Value: operationID}

	if err := s.nfseRepo.TransactReserveAndCreate(
		ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded, outboxTx,
	); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número da NFS-e. Tente novamente.")
		}
		return nil, err
	}
	return encoded, nil
}

// resolvePerson devolve nil quando o id é nil (pessoa opcional) e 404 quando o
// id foi informado mas não existe no cadastro.
func (s *NfseService) resolvePerson(ctx context.Context, orgPK string, id *string) (map[string]types.AttributeValue, error) {
	if id == nil {
		return nil, nil
	}
	item, err := s.personRepo.Get(ctx, orgPK, *id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("pessoa não encontrada: " + *id)
	}
	return item, nil
}
