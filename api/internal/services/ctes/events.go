package ctes

// events.go: eventos do CT-e pela fila (CTeRecepcaoEvento): cancelamento (110111), carta de correção (110110),
// prestação de serviço em desacordo (610110) e seu cancelamento (610111). EPEC não existe para o CT-e (o MOC o
// remove do ambiente normal, rejeição 629). O resultado volta pelo worker cte-event, que grava em cte_events e
// avisa pelo SNS. Regras: MOC CT-e, RecepcaoEventoGeral e RecepcaoEventosEspecificos.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/documents"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	sefazServiceEvento = "CTeRecepcaoEvento"
	eventsTableKey     = "cte"

	// Tipos de evento do CT-e (eventoCTe_v4.00).
	TpEventoCancelamento  = "110111"
	TpEventoCCe           = "110110"
	TpEventoDesacordo     = "610110"
	TpEventoCancDesacordo = "610111"

	// StatusCancelPending é o CT-e com cancelamento enviado e ainda sem resposta da SEFAZ.
	StatusCancelPending = "cancel_pending"

	eventStatusPending = "pending"
	eventStatusSuccess = "success"

	// Limites do MOC. nSeqEvento: cancelamento só 1 (O01, rejeição 636); CC-e e desacordo 1 a 20.
	MaxEventSequence = 20
	minJustification = 15 // TJust (tiposGeralCTe_v4.00.xsd)
	maxJustification = 255
	maxCorrectionGrp = 20
	maxCorrectionFld = 20
	maxCorrectionVal = 500
	maxCorrectionItm = 99
	minObs, maxObs   = 15, 255

	// indDesacordoOper: único valor do indicador de prestação em desacordo (MOC, evPrestDesacordo).
	indDesacordoOper = "1"

	// Descrições fixas do evento (MOC, 2ª coluna "Descrição do Evento").
	descCancelamento  = "Cancelamento"
	descCCe           = "Carta de Correção"
	descDesacordo     = "Prestação do Serviço em Desacordo"
	descCancDesacordo = "Cancelamento Prestação do Serviço em Desacordo"

	// xCondUso da carta de correção (literal do MOC, Convênio SINIEF 06/89 art. 58-B).
	xCondUsoCCe = "A Carta de Correção é disciplinada pelo Art. 58-B do CONVÊNIO/SINIEF 06/89: Fica permitida a utilização de carta de correção, para regularização de erro ocorrido na emissão de documentos fiscais relativos à prestação de serviço de transporte, desde que o erro não esteja relacionado com: I - as variáveis que determinam o valor do imposto tais como: base de cálculo, alíquota, diferença de preço, quantidade, valor da prestação;II - a correção de dados cadastrais que implique mudança do emitente, tomador, remetente ou do destinatário;III - a data de emissão ou de saída."

	// Códigos (problem.code) de 422 dos eventos.
	CodeNotIssuer      = "not_cte_issuer"
	CodeNotTaker       = "not_cte_taker"
	CodeNotAuthorized  = "cte_not_authorized"
	CodeNoProtocol     = "cte_protocol_missing"
	CodeEventInvalid   = "event_invalid"
	CodeEventNotFound  = "event_not_found"
	CodeEventNotActive = "event_not_authorized"
)

var protocolRe = regexp.MustCompile(`^\d{15}$`)

// Correction é um item da carta de correção (infCorrecao).
type Correction struct {
	Group      string `json:"group" validate:"required,min=1,max=20"`
	Field      string `json:"field" validate:"required,min=1,max=20"`
	Value      string `json:"value" validate:"required,min=1,max=500"`
	ItemNumber int    `json:"item_number" validate:"omitempty,gte=1,lte=99"`
}

// eventContext junta o que o despacho de um evento precisa do CT-e e da organização.
type eventContext struct {
	cte         map[string]types.AttributeValue
	pk          string
	environment int
	cnpj        string
	docTag      string
	emitUF      string
	sefazEnv    string
	certS3Key   string
	certPass    string
}

// resolveEventContext carrega o CT-e, exige que esteja autorizado e resolve certificado e ambiente.
func (s *CteService) resolveEventContext(ctx context.Context, orgPK, accessKey string) (*eventContext, error) {
	cte, err := s.GetCTe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if cte == nil {
		return nil, ErrCTeNotFound
	}
	if strAttr(cte, "status") != statusAuthorized {
		return nil, unprocessable(CodeNotAuthorized, "apenas CT-e autorizados permitem esta operação")
	}
	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, dfeproblem.NoCertificate("certificado digital não encontrado")
	}
	org, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	issuerDoc, _ := services.IssuerDocAV(org, orgPK)
	pk := strAttr(cte, "pk")
	environment := 2
	if envPrefixFromPK(pk) == services.EnvProd {
		environment = 1
	}
	return &eventContext{
		cte: cte, pk: pk, environment: environment, cnpj: issuerDoc,
		docTag: services.IssuerDocTagAV(org, orgPK), emitUF: ufFromAccessKey(accessKey),
		sefazEnv:  sefazEnvFor(environment),
		certS3Key: strAttr(certs[0], "s3_key"), certPass: strAttr(certs[0], "password"),
	}, nil
}

// requireIssuer: cancelamento e carta de correção são eventos do emitente (rejeição 632 se o autor diverge).
func (ec *eventContext) requireIssuer() error {
	if doc := strAttr(ec.cte, "emit_cpf_cnpj"); doc != "" && doc != ec.cnpj {
		return unprocessable(CodeNotIssuer, "este evento só pode ser registrado pelo emitente do CT-e")
	}
	return nil
}

// requireTaker: prestação em desacordo é evento do tomador (rejeição 755). Registro sem tomador gravado
// (CT-e recebido) segue para a SEFAZ, que confere o autor.
func (ec *eventContext) requireTaker() error {
	taker, _ := ec.cte["taker"].(*types.AttributeValueMemberM)
	if taker == nil {
		return nil
	}
	if doc := strAttr(taker.Value, "doc"); doc != "" && doc != ec.cnpj {
		return unprocessable(CodeNotTaker, "este evento só pode ser registrado pelo tomador do serviço do CT-e")
	}
	return nil
}

func (ec *eventContext) protocol() (string, error) {
	nProt := strAttr(ec.cte, "sefaz_protocol")
	if nProt == "" {
		return "", unprocessable(CodeNoProtocol, "protocolo de autorização não encontrado no CT-e")
	}
	return nProt, nil
}

// nextSequence é o nSeqEvento: o pedido ou, omitido, o seguinte ao maior já registrado para o tipo.
func (s *CteService) nextSequence(ctx context.Context, accessKey, eventType string, requested int) (int, error) {
	if requested > 0 {
		if requested > MaxEventSequence {
			return 0, unprocessable(CodeEventInvalid, fmt.Sprintf("o número sequencial do evento vai de 1 a %d", MaxEventSequence))
		}
		return requested, nil
	}
	res, err := s.eventRepo.GetDocumentEvents(ctx, accessKey, 200, nil)
	if err != nil {
		return 0, err
	}
	maxSeq := 0
	for _, ev := range res.Items {
		if strAttr(ev, "event_type") == eventType {
			maxSeq = max(maxSeq, intAttr(ev, "sequence_number", 0))
		}
	}
	if maxSeq >= MaxEventSequence {
		return 0, unprocessable(CodeEventInvalid, fmt.Sprintf("limite de %d eventos deste tipo atingido", MaxEventSequence))
	}
	return maxSeq + 1, nil
}

// dispatchEvent grava o evento pendente, muda o status do CT-e quando pedido e publica para o worker.
func (s *CteService) dispatchEvent(ctx context.Context, ec *eventContext, accessKey, eventType string, seq int,
	body map[string]any, newDocStatus, userID, userName string) (map[string]types.AttributeValue, error) {
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, seq, eventStatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}
	if newDocStatus != "" {
		if _, err := s.cteRepo.Update(ctx, ec.pk, accessKey, map[string]any{
			"status": newDocStatus, "updated_at": repositories.NowStr(),
		}); err != nil {
			return nil, err
		}
	}
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ec.pk, AccessKey: accessKey, TableName: tableCtes, S3Prefix: s3PrefixCte,
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, seq),
		CNPJ:             ec.cnpj, UF: ec.emitUF, SefazEnvironment: ec.sefazEnv,
		CertS3Key: ec.certS3Key, CertPassword: ec.certPass,
		DocType: s3PrefixCte, SefazService: sefazServiceEvento, Body: body,
		Contingency:     eventContingency(eventType, accessKey),
		EventsTableName: aws.String(eventsTableKey + "_events"), EventType: aws.String(eventType),
		SequenceNumber: &seq, EventSK: aws.String(strAttr(event, "sk")),
	}); err != nil {
		return nil, err
	}
	if newDocStatus != "" {
		ec.cte["status"] = &types.AttributeValueMemberS{Value: newDocStatus}
	}
	return ec.cte, nil
}

// Cancel pede o cancelamento (110111). O CT-e vai a cancel_pending até a resposta; nSeqEvento é sempre 1.
func (s *CteService) Cancel(ctx context.Context, orgPK, accessKey, justification, userID, userName string) (map[string]types.AttributeValue, error) {
	if n := len([]rune(strings.TrimSpace(justification))); n < minJustification || n > maxJustification {
		return nil, unprocessable(CodeEventInvalid, fmt.Sprintf("a justificativa vai de %d a %d caracteres", minJustification, maxJustification))
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if err := ec.requireIssuer(); err != nil {
		return nil, err
	}
	nProt, err := ec.protocol()
	if err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoCancelamento, 1, "evCancCTe", map[string]any{
		"descEvento": descCancelamento, "nProt": nProt, "xJust": strings.TrimSpace(justification),
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoCancelamento, 1, body, StatusCancelPending, userID, userName)
}

// CorrectionLetter envia a carta de correção (110110) com os itens corrigidos. O CT-e continua autorizado.
func (s *CteService) CorrectionLetter(ctx context.Context, orgPK, accessKey string, corrections []Correction, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if len(corrections) == 0 {
		return nil, unprocessable(CodeEventInvalid, "informe ao menos uma correção")
	}
	infCorrecao := make([]map[string]any, 0, len(corrections))
	for i, c := range corrections {
		if c.Group == "" || len(c.Group) > maxCorrectionGrp || c.Field == "" || len(c.Field) > maxCorrectionFld ||
			c.Value == "" || len([]rune(c.Value)) > maxCorrectionVal || c.ItemNumber < 0 || c.ItemNumber > maxCorrectionItm {
			return nil, unprocessable(CodeEventInvalid, fmt.Sprintf("correção %d fora do leiaute (grupo e campo até %d, valor até %d, item de 1 a %d)", i+1, maxCorrectionGrp, maxCorrectionVal, maxCorrectionItm))
		}
		item := map[string]any{"grupoAlterado": c.Group, "campoAlterado": c.Field, "valorAlterado": c.Value}
		if c.ItemNumber > 0 {
			item["nroItemAlterado"] = fmt.Sprintf("%d", c.ItemNumber)
		}
		infCorrecao = append(infCorrecao, item)
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if err := ec.requireIssuer(); err != nil {
		return nil, err
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoCCe, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoCCe, seq, "evCCeCTe", map[string]any{
		"descEvento": descCCe, "infCorrecao": infCorrecao, "xCondUso": xCondUsoCCe,
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoCCe, seq, body, "", userID, userName)
}

// ServiceDisagreement registra a prestação de serviço em desacordo (610110), evento do tomador. obs é opcional.
func (s *CteService) ServiceDisagreement(ctx context.Context, orgPK, accessKey, obs string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	obs = strings.TrimSpace(obs)
	if obs != "" {
		if n := len([]rune(obs)); n < minObs || n > maxObs {
			return nil, unprocessable(CodeEventInvalid, fmt.Sprintf("as observações vão de %d a %d caracteres", minObs, maxObs))
		}
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if err := ec.requireTaker(); err != nil {
		return nil, err
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoDesacordo, seq); err != nil {
		return nil, err
	}
	det := map[string]any{"descEvento": descDesacordo, "indDesacordoOper": indDesacordoOper}
	if obs != "" {
		det["xObs"] = obs
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoDesacordo, seq, "evPrestDesacordo", det)
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoDesacordo, seq, body, "", userID, userName)
}

// CancelServiceDisagreement cancela um evento de desacordo (610111). O evento a cancelar é escolhido pelo sk
// na lista de eventos; o protocolo vem do registro dele, nunca digitado.
func (s *CteService) CancelServiceDisagreement(ctx context.Context, orgPK, accessKey, eventSK string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if err := ec.requireTaker(); err != nil {
		return nil, err
	}
	target, err := s.eventRepo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, problem.NotFound("evento não encontrado").WithCode(CodeEventNotFound)
	}
	nProt := strAttr(target, "sefaz_protocol")
	if strAttr(target, "event_type") != TpEventoDesacordo || strAttr(target, "status") != eventStatusSuccess || !protocolRe.MatchString(nProt) {
		return nil, unprocessable(CodeEventNotActive, "o evento escolhido não é uma prestação em desacordo autorizada")
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoCancDesacordo, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoCancDesacordo, seq, "evCancPrestDesacordo", map[string]any{
		"descEvento": descCancDesacordo, "nProtEvPrestDes": nProt,
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoCancDesacordo, seq, body, "", userID, userName)
}

// buildEventEnvelope monta o eventoCTe em volta do detalhe (rootTag é o grupo do evento dentro de detEvento).
// Id = "ID" + tpEvento + chCTe + nSeqEvento com 2 dígitos (N06, rejeição 628). cOrgao é a UF da chave.
func buildEventEnvelope(ec *eventContext, accessKey, tpEvento string, seq int, rootTag string, det map[string]any) map[string]any {
	infEvento := map[string]any{
		"@Id":        fmt.Sprintf("ID%s%s%02d", tpEvento, accessKey, seq),
		"cOrgao":     accessKey[0:2],
		"tpAmb":      fmt.Sprintf("%d", ec.environment),
		"chCTe":      accessKey,
		"dhEvento":   time.Now().In(time.FixedZone("BRT", -3*60*60)).Format(layoutDateTimeTZ),
		"tpEvento":   tpEvento,
		"nSeqEvento": fmt.Sprintf("%d", seq),
		"detEvento":  map[string]any{"@versaoEvento": cteVersao, rootTag: det},
	}
	infEvento[ec.docTag] = ec.cnpj
	return map[string]any{"eventoCTe": map[string]any{"@versao": cteVersao, "@xmlns": cteXMLNS, "infEvento": infEvento}}
}

// ListCTeEvents lista os eventos de um CT-e da organização.
func (s *CteService) ListCTeEvents(ctx context.Context, orgPK, accessKey string, limit int, startKey map[string]types.AttributeValue) (*repositories.QueryResult, error) {
	if cte, err := s.GetCTe(ctx, orgPK, accessKey); err != nil {
		return nil, err
	} else if cte == nil {
		return nil, ErrCTeNotFound
	}
	return s.eventRepo.GetDocumentEvents(ctx, accessKey, limit, startKey)
}

// GetEventXML devolve a URL assinada do XML de um evento, após conferir o CT-e da organização.
func (s *CteService) GetEventXML(ctx context.Context, orgPK, accessKey, eventSK string) (*documents.SignedFileDownload, error) {
	if cte, err := s.GetCTe(ctx, orgPK, accessKey); err != nil {
		return nil, err
	} else if cte == nil {
		return nil, ErrCTeNotFound
	}
	event, err := s.eventRepo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, problem.NotFound("evento não encontrado").WithCode(CodeEventNotFound)
	}
	s3Key := strAttr(event, "xml_s3_key")
	if s3Key == "" {
		return nil, problem.NotFound("XML do evento ainda não disponível")
	}
	return s.documentSvc.SignFile(ctx, s3Key, documents.XMLFilename(strAttr(event, "event_type")+"-"+accessKey), documents.ContentTypeXML)
}

// eventContingency routes the cancellation of a CT-e authorized on the SVC to that same SVC (the SVC accepts
// only the cancellation, rule N09); every other event goes to the normal authorizer.
func eventContingency(eventType, accessKey string) string {
	if eventType != TpEventoCancelamento {
		return ""
	}
	return svcAuthorizerForKey(accessKey)
}
