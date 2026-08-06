package nfses

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// NfseEventBody é o corpo de POST /v1.0/nfses/{id}/events. As chaves são em
// inglês; `id_ev_manif_rej` e `cpf_ag_trib` mantêm o nome normativo do campo do
// leiaute, a mesma regra do corpo de emissão.
type NfseEventBody struct {
	EventType           string `json:"event_type" validate:"required,len=6,numeric"`
	SequenceNumber      int    `json:"sequence_number" validate:"omitempty,gte=1,lte=999"`
	ReasonCode          string `json:"reason_code" validate:"omitempty,max=2"`
	ReasonDescription   string `json:"reason_description" validate:"omitempty,max=255"`
	SubstituteAccessKey string `json:"substitute_access_key" validate:"omitempty,len=50,numeric"`
	CPFAgTrib           string `json:"cpf_ag_trib" validate:"omitempty,cpf"`
	IDEvManifRej        string `json:"id_ev_manif_rej" validate:"omitempty,max=60"`
}

// Erros de evento reusados pelas rotas.
var (
	ErrNfseNotAuthorized = problem.BadRequest("a NFS-e ainda não foi autorizada pelo fisco")
	ErrNfseEventNotFound = problem.NotFound("evento não encontrado")
	ErrNfseEventNoXML    = problem.NotFound("XML do evento ainda não disponível")
)

// validateEventType rejeita o que o contribuinte não pode emitir. Os eventos
// privativos do fisco (105104, 105105, 205204, 305101-305103) só chegam pela
// distribuição do ADN (spec §5.3). O conjunto é o mesmo que o go-dfe usa ao
// serializar — uma fonte só.
func validateEventType(t string) error {
	if !nfse.ContribuinteEvents[t] {
		return problem.BadRequest("evento " + t + " não pode ser emitido pelo contribuinte")
	}
	return nil
}

// buildEventRequest monta o pedido neutro. Substituição é redirecionada: não é
// pedido de registro de evento, é uma nova DPS com o grupo subst.
//
// As regras de motivo vêm de nfse.EventsRequiring*Motivo, os mesmos mapas que o
// go-dfe consulta — validar aqui troca um evento rejeitado assíncrono por um
// 400 síncrono, sem duplicar a regra do leiaute.
func buildEventRequest(chave string, body NfseEventBody, inscFederal string, environment int) (nfse.EventRequest, error) {
	if body.EventType == nfse.EventCancelamentoPorSubst {
		return nfse.EventRequest{}, problem.BadRequest(
			"cancelamento por substituição é gerado pelo fisco a partir de uma nova emissão: use POST /nfses/{id}/substitute")
	}
	if err := validateEventType(body.EventType); err != nil {
		return nfse.EventRequest{}, err
	}
	if nfse.EventsRequiringMotivo[body.EventType] && body.ReasonCode == "" {
		return nfse.EventRequest{}, problem.BadRequest("reason_code é obrigatório para o evento " + body.EventType)
	}
	if nfse.EventsRequiringXMotivo[body.EventType] && body.ReasonDescription == "" {
		return nfse.EventRequest{}, problem.BadRequest("reason_description é obrigatório para o evento " + body.EventType)
	}

	seq := body.SequenceNumber
	if seq == 0 {
		seq = 1
	}
	ev := nfse.EventRequest{
		ChaveAcesso: chave, TipoEvento: body.EventType, NSeqEvento: seq,
		TpAmb: environment, VerAplic: appVersion, DhEvento: time.Now().UTC(),
		ChSubstituta: body.SubstituteAccessKey,
		CPFAgTrib:    body.CPFAgTrib, IDEvManifRej: body.IDEvManifRej,
	}
	// TCInfPedReg exige escolha única entre CNPJ e CPF do autor.
	if len(inscFederal) == lenCPF {
		ev.CPFAutor = inscFederal
	} else {
		ev.CNPJAutor = inscFederal
	}
	if body.ReasonCode != "" {
		ev.Motivo = &nfse.EventMotivo{Codigo: body.ReasonCode, Descricao: body.ReasonDescription}
	}
	return ev, nil
}

// buildEventWorkerBody serializa o pedido no formato que nfse.Dispatch lê
// (go-dfe/nfse/dispatch.go): as chaves "provider" e "event".
func buildEventWorkerBody(provider string, ev nfse.EventRequest) (map[string]any, error) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return nil, problem.InternalServer("failed to encode nfse event")
	}
	var evMap map[string]any
	if err := json.Unmarshal(raw, &evMap); err != nil {
		return nil, problem.InternalServer("failed to decode nfse event")
	}
	return map[string]any{
		nfse.BodyKeyProvider: provider,
		nfse.BodyKeyEvent:    evMap,
	}, nil
}

// eventContext é o contexto resolvido uma vez por evento: a linha da NFS-e, o
// certificado e o ambiente. Espelha o eventContext de mdfes.
type eventContext struct {
	item        map[string]types.AttributeValue
	pk          string
	idDPS       string
	accessKey   string
	provider    string
	environment int
	sefazEnv    string
	inscFederal string
	certS3Key   string
	certPass    string
}

// resolveEventContext exige NFS-e autorizada COM chave de acesso: o pedido de
// registro de evento é endereçado à chave de 50 dígitos, não ao id_dps.
func (s *NfseService) resolveEventContext(ctx context.Context, orgPK, id string) (*eventContext, error) {
	item, err := s.GetNfse(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if strAttr(item, "status") != StatusAuthorized {
		return nil, ErrNfseNotAuthorized
	}
	accessKey := strAttr(item, "access_key")
	if accessKey == "" {
		return nil, ErrNfseNotAuthorized
	}

	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, ErrNfseNoCertificate
	}
	cert := certs[0]

	pk := strAttr(item, "pk")
	environment := 2
	if strings.HasPrefix(pk, services.EnvProd+"#") {
		environment = 1
	}
	sefazEnv := services.SefazEnvHom
	if environment == 1 {
		sefazEnv = services.SefazEnvProd
	}

	return &eventContext{
		item: item, pk: pk, idDPS: strAttr(item, "sk"), accessKey: accessKey,
		provider: strAttr(item, "provider"), environment: environment, sefazEnv: sefazEnv,
		inscFederal: services.StripPKPrefix(orgPK),
		certS3Key:   strAttr(cert, "s3_key"), certPass: strAttr(cert, "password"),
	}, nil
}

// SendEvent grava o evento em nfse_events (pk = id_dps, spec §3.5) e enfileira
// o pedido de registro para o worker.
func (s *NfseService) SendEvent(ctx context.Context, orgPK, id string, body NfseEventBody, userID, userName string) (map[string]types.AttributeValue, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}

	ev, err := buildEventRequest(ec.accessKey, body, ec.inscFederal, ec.environment)
	if err != nil {
		return nil, err
	}
	workerBody, err := buildEventWorkerBody(ec.provider, ev)
	if err != nil {
		return nil, err
	}

	event, err := s.eventRepo.CreateEvent(
		ctx, ec.idDPS, ev.TipoEvento, ev.NSeqEvento, StatusPending, nil, nil, nil, userID, userName,
	)
	if err != nil {
		return nil, err
	}
	eventSK := strAttr(event, "sk")

	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK:            ec.pk,
		AccessKey:        ec.idDPS, // identificador da linha em nfses
		TableName:        repositories.TableNfses,
		S3Prefix:         S3PrefixNfse,
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", ec.idDPS, ev.TipoEvento, ev.NSeqEvento),
		CNPJ:             ec.inscFederal,
		UF:               "", // competência municipal: não há UF autorizadora
		SefazEnvironment: ec.sefazEnv,
		CertS3Key:        ec.certS3Key,
		CertPassword:     ec.certPass,
		DocType:          DocTypeNfse,
		SefazService:     nfse.ServiceEvento,
		Body:             workerBody,
		EventsTableName:  aws.String(repositories.TableNfseEvents),
		EventType:        aws.String(ev.TipoEvento),
		SequenceNumber:   &ev.NSeqEvento,
		EventSK:          aws.String(eventSK),
	}); err != nil {
		return nil, err
	}
	return event, nil
}

// Cancel é o evento 101101. O motivo é obrigatório em código e descrição
// (TE101101 exige cMotivo e xMotivo).
func (s *NfseService) Cancel(ctx context.Context, orgPK, id, reasonCode, reasonDescription string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.SendEvent(ctx, orgPK, id, NfseEventBody{
		EventType:         nfse.EventCancelamento,
		SequenceNumber:    sequenceNumber,
		ReasonCode:        reasonCode,
		ReasonDescription: reasonDescription,
	}, userID, userName)
}

// Substitute NÃO emite evento: substituição é uma nova DPS com o grupo subst
// apontando para a chave da nota original. O fisco gera o evento 105102 e
// cancela a original por conta própria (manual do contribuinte, seção 1.3.2).
func (s *NfseService) Substitute(ctx context.Context, orgPK, id string, req NfseEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	original, err := s.GetNfse(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if strAttr(original, "status") != StatusAuthorized {
		return nil, ErrNfseNotAuthorized
	}
	accessKey := strAttr(original, "access_key")
	if accessKey == "" {
		return nil, ErrNfseNotAuthorized
	}
	if req.SubstitutesReason == nil || *req.SubstitutesReason == "" {
		return nil, problem.BadRequest("substitutes_reason é obrigatório na substituição")
	}
	req.SubstitutesAccessKey = &accessKey
	return s.Emit(ctx, orgPK, req, userID, userName)
}

// ListEvents lista os eventos da NFS-e. O id pode ser id_dps ou chave de
// acesso: GetNfse resolve os dois e a partição de nfse_events é sempre o id_dps.
func (s *NfseService) ListEvents(ctx context.Context, orgPK, id string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	item, err := s.GetNfse(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	return s.eventRepo.GetDocumentEvents(ctx, strAttr(item, "sk"), limit, startKey)
}

// GetEventXML baixa o XML do evento do S3 e devolve também o event_type, que a
// rota usa para nomear o arquivo.
func (s *NfseService) GetEventXML(ctx context.Context, orgPK, id, eventSK string) ([]byte, string, error) {
	item, err := s.GetNfse(ctx, orgPK, id)
	if err != nil {
		return nil, "", err
	}
	event, err := s.eventRepo.GetEvent(ctx, strAttr(item, "sk"), eventSK)
	if err != nil {
		return nil, "", err
	}
	if event == nil {
		return nil, "", ErrNfseEventNotFound
	}
	s3Key := strAttr(event, "xml_s3_key")
	if s3Key == "" {
		return nil, "", ErrNfseEventNoXML
	}
	data, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, s3Key)
	return data, strAttr(event, "event_type"), err
}
