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

const attrEmitInput = "emit_input"

// Comprimento das inscrições federais, usado para decidir CPF vs CNPJ.
const (
	lenCPF  = 11
	lenCNPJ = 14
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
	Competence   string `json:"competence" validate:"required,isodate"`

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
	ServiceID   string  `json:"service_id" dynamodbav:"service_id" validate:"required"`
	Description *string `json:"description" dynamodbav:"description,omitempty" validate:"omitempty,max=2000"`
	Value       *string `json:"value" dynamodbav:"value,omitempty" validate:"omitempty,money"`
	TaxRate     *string `json:"tax_rate" dynamodbav:"tax_rate,omitempty" validate:"omitempty,money"`
	CTribMun    *string `json:"c_trib_mun" dynamodbav:"c_trib_mun,omitempty" validate:"omitempty,max=20"`
}

// NfseEmitInputSnapshot preserva as referências de catálogo e os overrides
// escolhidos pelo usuário. O documento fiscal resolvido continua em payload;
// este snapshot existe somente para reabrir/duplicar a emissão sem tentar
// inferir entidades a partir de códigos fiscais ou nomes.
type NfseEmitInputSnapshot struct {
	TpEmit           int             `json:"tp_emit" dynamodbav:"tp_emit"`
	MotivoEmisTI     int             `json:"motivo_emis_ti,omitempty" dynamodbav:"motivo_emis_ti,omitempty"`
	ChNFSeRej        string          `json:"ch_nfse_rej,omitempty" dynamodbav:"ch_nfse_rej,omitempty"`
	ProviderPersonID *string         `json:"provider_person_id,omitempty" dynamodbav:"provider_person_id,omitempty"`
	CustomerID       *string         `json:"customer_id,omitempty" dynamodbav:"customer_id,omitempty"`
	IntermediaryID   *string         `json:"intermediary_id,omitempty" dynamodbav:"intermediary_id,omitempty"`
	Service          NfseServiceItem `json:"service" dynamodbav:"service"`
	AdditionalInfo   *string         `json:"additional_info,omitempty" dynamodbav:"additional_info,omitempty"`
}

func emitInputSnapshot(req NfseEmitBody) NfseEmitInputSnapshot {
	return NfseEmitInputSnapshot{
		TpEmit: req.TpEmit, MotivoEmisTI: req.MotivoEmisTI, ChNFSeRej: req.ChNFSeRej,
		ProviderPersonID: req.ProviderPersonID, CustomerID: req.CustomerID,
		IntermediaryID: req.IntermediaryID, Service: req.Service, AdditionalInfo: req.AdditionalInfo,
	}
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
		nfse.BodyKeyProvider:     provider,
		nfse.BodyKeyMunicipality: doc.CLocEmi,
		nfse.BodyKeyDocument:     docMap,
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

// nfseEmissionTime fixa o instante da DPS no timezone fiscal configurado.
// Configurações anteriores ao campo timezone usam o default documentado para
// continuar emitindo até serem salvas novamente pela UI.
func nfseEmissionTime(now time.Time, timezone string) (time.Time, error) {
	if timezone == "" {
		timezone = defaultTimezone
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, problem.InternalServer("invalid NFS-e timezone configuration")
	}
	return now.In(location), nil
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
	// The issuer's document, off the record: the persisted row and the go-dfe
	// envelope both need it.
	emitDoc, _ := services.IssuerDocAV(orgItem, orgPK)
	if emitDoc == "" {
		return nil, problem.BadRequest("documento do emitente não encontrado")
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
		prestadorItem, err = s.resolvePerson(ctx, orgPK, req.ProviderPersonID, orgItem)
		if err != nil {
			return nil, err
		}
	}

	tomadorItem, err := s.resolvePerson(ctx, orgPK, req.CustomerID, orgItem)
	if err != nil {
		return nil, err
	}
	intermItem, err := s.resolvePerson(ctx, orgPK, req.IntermediaryID, orgItem)
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

	competence, parseErr := time.Parse(time.DateOnly, req.Competence)
	if parseErr != nil {
		return nil, problem.BadRequest("competence deve estar no formato AAAA-MM-DD")
	}
	now := time.Now()
	emissionTime, err := nfseEmissionTime(now, strAttr(configItem, "timezone"))
	if err != nil {
		return nil, err
	}

	doc, err := buildDocument(documentInput{
		Org: orgItem, Config: configItem, Prestador: prestadorItem,
		Tomador: tomadorItem, Intermediario: intermItem, Service: serviceItem,
		Body: req, Serie: serie, Numero: currentNumber, Environment: environment,
		DhEmi: emissionTime.Format(dfeDateTimeLayout),
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
		"dh_emi":        doc.DhEmi,
		"c_loc_emi":     doc.CLocEmi,
		"year":          competence.Year(),
		"month":         int(competence.Month()),
		"emit_cpf_cnpj": emitDoc,
		"emit_name":     strAttr(orgItem, "name"),
		"dest_name":     strAttr(tomadorItem, "name"),
		"dest_cpf_cnpj": personDoc(tomadorItem),
		"total":         doc.Valores.VServPrest.VServ,
		"payload":       docMap,
		attrEmitInput:   emitInputSnapshot(req),
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
	reservation, err := s.billingSvc.PrepareUsageReservation(ctx, orgPK, services.MeterNFSe, envPrefix == services.EnvProd)
	if err != nil {
		return nil, err
	}
	outboxTx, operationID, err := s.workerSvc.BuildOutboxTx(services.WorkerMessage{
		DocPK:            pk,
		AccessKey:        idDPS, // identificador da linha; NFS-e usa id_dps
		TableName:        repositories.TableNfses,
		S3Prefix:         S3PrefixNfse,
		ExpectedFileName: idDPS,
		CNPJ:             emitDoc,
		UF:               "", // competência municipal: não há UF autorizadora
		SefazEnvironment: sefazEnv,
		CertS3Key:        strAttr(cert, "s3_key"),
		CertPassword:     strAttr(cert, "password"),
		DocType:          DocTypeNfse,
		SefazService:     nfse.ServiceRecepcao,
		Body:             workerBody,
		BillingUserID:    reservation.UserID, BillingPeriod: reservation.Period,
		BillingSubscriptionID: reservation.SubscriptionID, BillingPriceID: reservation.PriceID,
		BillingMeter: reservation.Meter, BillingExempt: reservation.Exempt,
	})
	if err != nil {
		return nil, err
	}
	encoded["operation_id"] = &types.AttributeValueMemberS{Value: operationID}

	// The quota is claimed **before** the write, and before anything reaches
	// SEFAZ. Counting authorised documents instead would make the limit
	// asynchronous and passable by two concurrent requests, each reading the same
	// count and each issuing one more. The cost is that a document SEFAZ rejects
	// has spent a slot; the worker gives it back on a terminal rejection.
	extraTx := []types.TransactWriteItem{outboxTx}
	if reservation.Tx != nil {
		extraTx = append(extraTx, *reservation.Tx)
	}
	if err := s.nfseRepo.TransactReserveAndCreate(
		ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded, extraTx...,
	); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número da NFS-e. Tente novamente.")
		}
		return nil, err
	}
	return encoded, nil
}

// resolvePerson devolve nil quando o id é nil (pessoa opcional) e 404 quando o
// id foi informado mas não existe no cadastro. Quando o id é o documento da
// própria organização devolve o item dela: a empresa pode ser tomadora (ou
// intermediária) da própria NFS-e e não existe como pessoa do cadastro.
func (s *NfseService) resolvePerson(ctx context.Context, orgPK string, id *string, orgItem map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	if id == nil {
		return nil, nil
	}
	// The left side is a person's sort key, which really is a document. The
	// right side is the issuer's, which the key no longer carries.
	issuerDoc, _ := services.IssuerDocAV(orgItem, orgPK)
	if issuerDoc != "" && services.StripPKPrefix(*id) == issuerDoc {
		return orgItem, nil
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
