package nfcoms

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

// eventContext bundles what dispatching a SEFAZ event for an NFCom needs.
type eventContext struct {
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

// resolveEventContext loads the NFCom, requires it authorized and resolves the certificate and environment.
func (s *NfcomService) resolveEventContext(ctx context.Context, orgPK, accessKey string) (*eventContext, error) {
	doc, err := s.GetNFCom(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrNFComNotFound
	}
	if strAttr(doc, "status") != StatusAuthorized {
		return nil, problem.BadRequest("apenas NFCom autorizadas permitem esta operação")
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
	return &eventContext{
		doc: doc, pk: pk, environment: environment, cnpj: issuerDoc, emitUF: emitUFFromAccessKey(accessKey),
		sefazEnv: sefazEnvFor(environment), certS3Key: strAttr(certs[0], "s3_key"), certPass: strAttr(certs[0], "password"),
	}, nil
}

// Cancel marks the NFCom cancel_pending and dispatches the cancellation event (110111). The 120-hour window is the
// authorizer's (cStat 220); the rejection comes back through the worker and the document returns to authorized.
func (s *NfcomService) Cancel(ctx context.Context, orgPK, accessKey, justification string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if n := len([]rune(justification)); n < minJustification || n > 255 {
		return nil, problem.BadRequest("justificativa de cancelamento deve ter de 15 a 255 caracteres")
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	nProt := strAttr(ec.doc, "sefaz_protocol")
	if nProt == "" {
		return nil, problem.BadRequest("protocolo de autorização não encontrado na NFCom")
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoCancelamento, seq, cancelDet(nProt, justification))
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoCancelamento, seq, body, StatusCancelPending, userID, userName)
}

func cancelDet(nProt, justification string) map[string]any {
	return map[string]any{"evCancNFCom": map[string]any{"descEvento": "Cancelamento", "nProt": nProt, "xJust": justification}}
}

func buildEventEnvelope(ec *eventContext, accessKey, tpEvento string, seq int, detEvento map[string]any) map[string]any {
	detEvento["@versaoEvento"] = nfcomVersao
	return map[string]any{
		"eventoNFCom": map[string]any{
			"@versao": nfcomVersao,
			"@xmlns":  nfcomXMLNS,
			"infEvento": map[string]any{
				"@Id":        fmt.Sprintf("ID%s%s%03d", tpEvento, accessKey, seq),
				"cOrgao":     accessKey[0:2],
				"tpAmb":      fmt.Sprintf("%d", ec.environment),
				"CNPJ":       ec.cnpj,
				"chNFCom":    accessKey,
				"dhEvento":   uftz.Now(ec.emitUF).Format(layoutDateTimeTZ),
				"tpEvento":   tpEvento,
				"nSeqEvento": fmt.Sprintf("%d", seq),
				"detEvento":  detEvento,
			},
		},
	}
}

// dispatchEvent persists the event record, updates the document status and publishes the event to the worker.
func (s *NfcomService) dispatchEvent(
	ctx context.Context, ec *eventContext, accessKey, eventType string, seq int,
	body map[string]any, newDocStatus string, userID, userName string,
) (map[string]types.AttributeValue, error) {
	dfecheck.ShadowEvent("nfcom", accessKey, body)
	if err := dfecheck.EventProblem("NFCom", dfecheck.GateEvent("nfcom", body)); err != nil {
		return nil, err
	}
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, seq, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}
	if newDocStatus != "" {
		if _, err := s.nfcomRepo.Update(ctx, ec.pk, accessKey, map[string]any{
			"status": newDocStatus, "updated_at": repositories.NowStr(),
		}); err != nil {
			return nil, err
		}
	}
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK: ec.pk, AccessKey: accessKey, TableName: tableNfcoms, S3Prefix: s3PrefixNfcom,
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, seq),
		CNPJ:             ec.cnpj, UF: ec.emitUF, SefazEnvironment: ec.sefazEnv,
		CertS3Key: ec.certS3Key, CertPassword: ec.certPass,
		DocType: s3PrefixNfcom, SefazService: sefazServiceEvento, Body: body,
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
