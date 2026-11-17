package bpes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// eventContext bundles what dispatching a SEFAZ event for an BPe needs.
type eventContext struct {
	variant     string
	doc         map[string]types.AttributeValue
	pk          string
	environment int
	cnpj        string
	emitUF      string
	sefazEnv    string
	certS3Key   string
	certPass    string
}

// emitUFFromAccessKey derives the 2-letter emitter UF from the cUF in the access key.
func emitUFFromAccessKey(accessKey string) string {
	if len(accessKey) < 2 {
		return ""
	}
	return services.UFFromCode[accessKey[0:2]]
}

// resolveEventContext loads the BPe, requires it authorized and resolves the certificate and environment.
func (s *BpeService) resolveEventContext(ctx context.Context, orgPK, accessKey string) (*eventContext, error) {
	doc, err := s.GetBPe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrBPeNotFound
	}
	if strAttr(doc, "status") != StatusAuthorized {
		return nil, problem.BadRequest("apenas BP-e autorizados permitem esta operação")
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
	issuerDoc, _ := services.IssuerDocAV(org, orgPK)
	pk := strAttr(doc, "pk")
	environment := 2
	if envPrefixFromPK(pk) == EnvProd {
		environment = 1
	}
	variant := strAttr(doc, "variant")
	if variant == "" {
		variant = VariantNormal
	}
	return &eventContext{
		variant: variant, doc: doc, pk: pk, environment: environment, cnpj: issuerDoc, emitUF: emitUFFromAccessKey(accessKey),
		sefazEnv: sefazEnvFor(environment), certS3Key: strAttr(certs[0], "s3_key"), certPass: strAttr(certs[0], "password"),
	}, nil
}

// eventAllowed diz se o evento vale para a variante do BP-e (MOC I20a a I20c): o TM não aceita não embarque nem excesso
// de bagagem; o TA não aceita alteração de poltrona nem excesso de bagagem; o BP-e comum aceita os quatro. O cancelamento
// vale para todos.
func eventAllowed(variant, tpEvento string) bool {
	switch tpEvento {
	case TpEventoCancelamento:
		return true
	case TpEventoNaoEmbarque:
		return variant != VariantTM
	case TpEventoAlteracaoPoltrona:
		return variant != VariantTA
	case TpEventoExcessoBagagem:
		return variant == VariantNormal
	case TpEventoFornecimentoNaoRealizado:
		return variant == VariantTA
	}
	return false
}

// send valida a elegibilidade, monta o evento sobre o protocolo do BP-e autorizado e o despacha. newDocStatus só é
// preenchido pelo cancelamento: os demais eventos não mudam o status do documento.
func (s *BpeService) send(
	ctx context.Context, orgPK, accessKey, tpEvento string, seq int, newDocStatus string, userID, userName string,
	det func(nProt string) map[string]any,
) (map[string]types.AttributeValue, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if !eventAllowed(ec.variant, tpEvento) {
		return nil, problem.UnprocessableEntity("o evento " + tpEvento + " não se aplica a este tipo de BP-e")
	}
	nProt := strAttr(ec.doc, "sefaz_protocol")
	if nProt == "" {
		return nil, problem.BadRequest("protocolo de autorização não encontrado no BP-e")
	}
	body := buildEventEnvelope(ec, accessKey, tpEvento, seq, det(nProt))
	return s.dispatchEvent(ctx, ec, accessKey, tpEvento, seq, body, newDocStatus, userID, userName)
}

func justificationOK(j string) error {
	if n := len([]rune(j)); n < minJustification || n > 255 {
		return problem.BadRequest("justificativa deve ter de 15 a 255 caracteres")
	}
	return nil
}

// Cancel marks the BP-e cancel_pending and dispatches the cancellation event (110111). Os prazos (antes do embarque, 168 h
// no TM) são do autorizador; a rejeição volta pelo worker e o documento retorna a autorizado.
func (s *BpeService) Cancel(ctx context.Context, orgPK, accessKey, justification string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := justificationOK(justification); err != nil {
		return nil, err
	}
	return s.send(ctx, orgPK, accessKey, TpEventoCancelamento, seq, StatusCancelPending, userID, userName,
		func(nProt string) map[string]any { return cancelDet(nProt, justification) })
}

// NoShow registra que o passageiro não embarcou (110115), até 24 h depois do embarque (prazo do autorizador).
func (s *BpeService) NoShow(ctx context.Context, orgPK, accessKey, justification string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := justificationOK(justification); err != nil {
		return nil, err
	}
	return s.send(ctx, orgPK, accessKey, TpEventoNaoEmbarque, seq, "", userID, userName,
		func(nProt string) map[string]any { return noShowDet(nProt, justification) })
}

// ChangeSeat troca a poltrona sem alterar valor nem data de embarque (110116).
func (s *BpeService) ChangeSeat(ctx context.Context, orgPK, accessKey, seat string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.send(ctx, orgPK, accessKey, TpEventoAlteracaoPoltrona, seq, "", userID, userName,
		func(nProt string) map[string]any { return seatChangeDet(nProt, seat) })
}

// ExcessBaggage registra a cobrança de excesso de bagagem (110117): quantidade (texto livre do leiaute, até 20
// caracteres) e valor total.
func (s *BpeService) ExcessBaggage(ctx context.Context, orgPK, accessKey, quantity, value string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if _, err := nonNegative(value); err != nil {
		return nil, problem.BadRequest("valor do excesso de bagagem inválido")
	}
	return s.send(ctx, orgPK, accessKey, TpEventoExcessoBagagem, seq, "", userID, userName,
		func(nProt string) map[string]any { return excessBaggageDet(nProt, quantity, money(parseDec(value))) })
}

// SupplyNotPerformed registra que o fornecimento pago antecipadamente não foi realizado (112140, só TA): valores de IBS e CBS do
// serviço não fornecido, com duas casas (13v2).
func (s *BpeService) SupplyNotPerformed(ctx context.Context, orgPK, accessKey, vIBS, vCBS string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	for _, v := range []string{vIBS, vCBS} {
		if _, err := nonNegative(v); err != nil {
			return nil, problem.BadRequest("valor de IBS ou CBS inválido")
		}
	}
	return s.send(ctx, orgPK, accessKey, TpEventoFornecimentoNaoRealizado, seq, "", userID, userName,
		func(nProt string) map[string]any {
			return supplyNotPerformedDet(nProt, money(parseDec(vIBS)), money(parseDec(vCBS)))
		})
}

func supplyNotPerformedDet(nProt, vIBS, vCBS string) map[string]any {
	return map[string]any{"evFornecNaoRealizBPe": map[string]any{
		"descEvento": "Fornecimento Não Realizado", "nProt": nProt, "vIBS": vIBS, "vCBS": vCBS,
	}}
}

func cancelDet(nProt, justification string) map[string]any {
	return map[string]any{"evCancBPe": map[string]any{"descEvento": "Cancelamento", "nProt": nProt, "xJust": justification}}
}

func noShowDet(nProt, justification string) map[string]any {
	return map[string]any{"evNaoEmbBPe": map[string]any{"descEvento": "Não Embarque", "nProt": nProt, "xJust": justification}}
}

func seatChangeDet(nProt, seat string) map[string]any {
	return map[string]any{"evAlteracaoPoltrona": map[string]any{"descEvento": "Alteração Poltrona", "nProt": nProt, "poltrona": seat}}
}

func excessBaggageDet(nProt, quantity, value string) map[string]any {
	return map[string]any{"evExcessoBagagem": map[string]any{
		"descEvento": "Excesso Bagagem", "nProt": nProt, "qBagagem": quantity, "vTotBag": value,
	}}
}

func buildEventEnvelope(ec *eventContext, accessKey, tpEvento string, seq int, detEvento map[string]any) map[string]any {
	detEvento["@versaoEvento"] = bpeVersao
	return map[string]any{
		"eventoBPe": map[string]any{
			"@versao": bpeVersao,
			"@xmlns":  bpeXMLNS,
			"infEvento": map[string]any{
				"@Id":        fmt.Sprintf("ID%s%s%03d", tpEvento, accessKey, seq),
				"cOrgao":     accessKey[0:2],
				"tpAmb":      fmt.Sprintf("%d", ec.environment),
				"CNPJ":       ec.cnpj,
				"chBPe":      accessKey,
				"dhEvento":   uftz.Now(ec.emitUF).Format(layoutDateTimeTZ),
				"tpEvento":   tpEvento,
				"nSeqEvento": fmt.Sprintf("%d", seq),
				"detEvento":  detEvento,
			},
		},
	}
}

// dispatchEvent persists the event record, updates the document status and publishes the event to the worker.
func (s *BpeService) dispatchEvent(
	ctx context.Context, ec *eventContext, accessKey, eventType string, seq int,
	body map[string]any, newDocStatus string, userID, userName string,
) (map[string]types.AttributeValue, error) {
	dfecheck.ShadowEvent("bpe", accessKey, body)
	if err := dfecheck.EventProblem("BPe", dfecheck.GateEvent("bpe", body)); err != nil {
		return nil, err
	}
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, seq, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}
	if newDocStatus != "" {
		if _, err := s.bpeRepo.Update(ctx, ec.pk, accessKey, map[string]any{
			"status": newDocStatus, "updated_at": repositories.NowStr(),
		}); err != nil {
			return nil, err
		}
	}
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ec.pk, AccessKey: accessKey, TableName: tableBpes, S3Prefix: s3PrefixBpe,
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, seq),
		CNPJ:             ec.cnpj, UF: ec.emitUF, SefazEnvironment: ec.sefazEnv,
		CertS3Key: ec.certS3Key, CertPassword: ec.certPass,
		DocType: s3PrefixBpe, SefazService: sefazServiceEvento, Body: body,
		EventsTableName: aws.String(eventsTableKey + "_events"), EventType: aws.String(eventType),
		SequenceNumber: &seq, EventSK: aws.String(strAttr(event, "sk")),
	}); err != nil {
		return nil, err
	}
	if newDocStatus != "" {
		ec.doc["status"] = &types.AttributeValueMemberS{Value: newDocStatus}
	}
	return ec.doc, nil
}
