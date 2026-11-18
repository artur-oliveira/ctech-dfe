package nfcoms

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// Emit resolves the registries the request points to (subscriber, person, plans) and issues the NFCom.
func (s *NfcomService) Emit(ctx context.Context, orgPK string, req NfcomEmitRequest, userID, userName string) (map[string]types.AttributeValue, error) {
	body, err := s.Resolve(ctx, orgPK, req)
	if err != nil {
		return nil, err
	}
	return s.emitBody(ctx, orgPK, body, userID, userName)
}

// emitBody validates the resolved body, reserves the next number and the quota, builds the NFCom and either queues it
// through the transactional outbox (online) or signs it now and keeps it for the retransmission (offline contingency).
func (s *NfcomService) emitBody(ctx context.Context, orgPK string, req NfcomEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := validateEmitBody(req); err != nil {
		return nil, err
	}
	orgItem, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if orgItem == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	// The issuer's document, off the record: a company id carries none.
	emitterDoc, isPJ := services.IssuerDocAV(orgItem, orgPK)
	if emitterDoc == "" || !isPJ {
		return nil, problem.BadRequest("a NFCom exige um emitente com CNPJ")
	}
	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, dfeproblem.NoCertificate("certificado digital não encontrado")
	}
	cert := certs[0]

	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if configItem == nil {
		return nil, problem.BadRequest("configure a NFCom em Configuração Fiscal antes de emitir")
	}
	environment := intAttr(configItem, "environment", 2)
	envPrefix := services.EnvToPrefix(environment)
	serie := intAttr(configItem, fmt.Sprintf("%s_current_serie", envPrefix), 1)
	currentNumber := intAttr(configItem, fmt.Sprintf("%s_current_number", envPrefix), 0)

	emitUF := extractEmitUF(orgItem)
	if emitUF == "" {
		return nil, problem.BadRequest("UF do emitente não configurada")
	}

	// A forma de emissão é resolvida antes da chave (tpEmis está na chave).
	tpEmis, offline := resolveNFComEmission(configItem)
	contingencyReason := ""
	if offline {
		var cfgReason *string
		if r := strAttr(configItem, repositories.AttrContingencyReason); r != "" {
			cfgReason = &r
		}
		if contingencyReason, err = resolveContingencyReason(req.ContingencyReason, cfgReason); err != nil {
			return nil, err
		}
	}

	now := uftz.Now(emitUF) // dhEmi e AAMM da chave são horário local da UF do emitente
	accessKey := GenerateAccessKey(emitUF, emitterDoc, serie, currentNumber+1, now, tpEmis)
	if accessKey == "" {
		return nil, problem.BadRequest("UF do emitente inválida: " + emitUF)
	}
	// Offline: o parâmetro sign do QR Code é a assinatura da chave de acesso (egress SignText).
	qrSign := ""
	if offline {
		if qrSign, err = services.SignTextViaEgress(ctx, s.egress, orgPK, emitterDoc, s3PrefixNfcom, accessKey); err != nil {
			return nil, err
		}
	}

	nfcomBody := BuildNFCom(buildParams{
		org: orgItem, orgPK: orgPK, accessKey: accessKey, serie: serie, number: currentNumber + 1,
		environment: environment, now: now, req: req, tpEmis: tpEmis, qrSign: qrSign,
		contingencyReason: contingencyReason, tech: s.tech,
	})
	// Portão local (422) antes de debitar a cota e de gravar o outbox: regras do MOC promovidas e schema.
	if err := gateNFCom(nfcomBody); err != nil {
		return nil, err
	}
	dfecheck.ShadowNFCom(accessKey, nfcomBody)

	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	record := buildRecord(pk, accessKey, orgPK, orgItem, currentNumber+1, serie, now, req, nfcomBody, userID, userName)
	encoded, err := repositories.EncodeItem(record)
	if err != nil {
		return nil, problem.InternalServer("failed to encode NFCom record")
	}

	reservation, err := s.billingSvc.PrepareUsageReservation(ctx, orgPK, services.MeterNFCom, envPrefix == EnvProd)
	if err != nil {
		return nil, err
	}
	workerMsg := services.WorkerMessage{
		DocPK: pk, AccessKey: accessKey, TableName: tableNfcoms, S3Prefix: s3PrefixNfcom, ExpectedFileName: accessKey,
		CNPJ: emitterDoc, UF: emitUF, SefazEnvironment: sefazEnvFor(environment),
		CertS3Key: strAttr(cert, "s3_key"), CertPassword: strAttr(cert, "password"),
		DocType: s3PrefixNfcom, SefazService: sefazServiceAutorizacao, Body: nfcomBody,
		BillingOrganizationID: reservation.OrganizationID, BillingPeriod: reservation.Period,
		BillingSubscriptionID: reservation.SubscriptionID, BillingPriceID: reservation.PriceID,
		BillingMeter: reservation.Meter, BillingExempt: reservation.Exempt,
	}
	var extraTx []types.TransactWriteItem
	if offline {
		// Offline: sign now, keep the signed XML pending (contingency index), transmit later. No outbox row.
		signed, err := services.SignOnlyViaEgress(ctx, s.egress, orgPK, emitterDoc, emitUF, sefazEnvFor(environment), s3PrefixNfcom, nfcomBody)
		if err != nil {
			return nil, err
		}
		if err := markOfflinePending(encoded, workerMsg, signed, now); err != nil {
			return nil, err
		}
	} else {
		outboxTx, operationID, err := s.workerSvc.BuildOutboxTx(workerMsg)
		if err != nil {
			return nil, err
		}
		encoded["operation_id"] = &types.AttributeValueMemberS{Value: operationID}
		extraTx = append(extraTx, outboxTx)
	}
	// The quota is claimed before the write and before anything reaches SEFAZ (a limit counted on authorized
	// documents would be passable by concurrent requests); the worker gives the slot back on a terminal rejection.
	if reservation.Tx != nil {
		extraTx = append(extraTx, *reservation.Tx)
	}
	if err := s.nfcomRepo.TransactReserveAndCreate(ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded, extraTx...); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número da NFCom. Tente novamente.")
		}
		return nil, err
	}
	return encoded, nil
}

// buildRecord is the DynamoDB row of the new NFCom (same index attributes as the other documents).
func buildRecord(
	pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue,
	number, serie int, now time.Time, req NfcomEmitBody, body map[string]any, userID, userName string,
) map[string]any {
	// Off the record, like every other issuer document on this path.
	emitterDoc, _ := services.IssuerDocAV(orgItem, orgPK)
	total := ""
	if nfcom, ok := body["NFCom"].(map[string]any); ok {
		if inf, ok := nfcom["infNFCom"].(map[string]any); ok {
			if t, ok := inf["total"].(map[string]any); ok {
				total, _ = t["vNF"].(string)
			}
		}
	}
	customerDoc := ""
	switch {
	case req.Customer.CNPJ != nil:
		customerDoc = docAlnum(*req.Customer.CNPJ)
	case req.Customer.CPF != nil:
		customerDoc = onlyDigits(*req.Customer.CPF)
	}
	return map[string]any{
		"pk": pk, "sk": accessKey, "incoming": 0,
		"year": now.Year(), "month": int(now.Month()), "day": now.Day(),
		"status": StatusPending, "sefaz_status": nil, "sefaz_motive": nil,
		"emit_cpf_cnpj": emitterDoc, "emit_name": strAttr(orgItem, "name"),
		"number": number, "serie": serie,
		"customer_name": req.Customer.Name, "customer_doc": customerDoc,
		"subscriber_code": req.Subscriber.Code, "total": total, "items": len(req.Items),
		"dh_emi": now.Format(layoutDateTimeTZ), "xml_s3_key": nil,
		"created_at": now.UTC().Format(time.RFC3339), "user_id": userID, "user_name": userName,
	}
}
