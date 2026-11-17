package bpes

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

// Emit validates the request, reserves the next number and the quota, builds the BPe and either queues it through
// the transactional outbox (online) or signs it now and keeps it for the retransmission (offline contingency).
func (s *BpeService) Emit(ctx context.Context, orgPK string, req BpeEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if req.Variant != "" && req.Variant != VariantNormal {
		return nil, problem.BadRequest("a variante " + req.Variant + " do BP-e ainda não está disponível para emissão")
	}
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
		return nil, problem.BadRequest("o BP-e exige um emitente com CNPJ")
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
		return nil, problem.BadRequest("configure o BP-e em Configuração Fiscal antes de emitir")
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
	tpEmis, offline := resolveBPeEmission(configItem)
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
		if qrSign, err = services.SignTextViaEgress(ctx, s.egress, orgPK, emitterDoc, s3PrefixBpe, accessKey); err != nil {
			return nil, err
		}
	}

	bpeBody := BuildBPe(buildParams{
		org: orgItem, orgPK: orgPK, accessKey: accessKey, serie: serie, number: currentNumber + 1,
		environment: environment, now: now, req: req, tpEmis: tpEmis, qrSign: qrSign,
		contingencyReason: contingencyReason, tech: s.tech,
	})
	// Portão local (422) antes de debitar a cota e de gravar o outbox: regras do MOC promovidas e schema.
	if err := gateBPe(specBPe, bpeBody); err != nil {
		return nil, err
	}
	dfecheck.ShadowBPe(specBPe, accessKey, bpeBody)

	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	record := buildRecord(pk, accessKey, orgPK, orgItem, currentNumber+1, serie, now, req, bpeBody, userID, userName)
	encoded, err := repositories.EncodeItem(record)
	if err != nil {
		return nil, problem.InternalServer("failed to encode BP-e record")
	}

	reservation, err := s.billingSvc.PrepareUsageReservation(ctx, orgPK, services.MeterBPe, envPrefix == EnvProd)
	if err != nil {
		return nil, err
	}
	workerMsg := services.WorkerMessage{
		DocPK: pk, AccessKey: accessKey, TableName: tableBpes, S3Prefix: s3PrefixBpe, ExpectedFileName: accessKey,
		CNPJ: emitterDoc, UF: emitUF, SefazEnvironment: sefazEnvFor(environment),
		CertS3Key: strAttr(cert, "s3_key"), CertPassword: strAttr(cert, "password"),
		DocType: s3PrefixBpe, SefazService: sefazServiceAutorizacao, Body: bpeBody,
		BillingOrganizationID: reservation.OrganizationID, BillingPeriod: reservation.Period,
		BillingSubscriptionID: reservation.SubscriptionID, BillingPriceID: reservation.PriceID,
		BillingMeter: reservation.Meter, BillingExempt: reservation.Exempt,
	}
	var extraTx []types.TransactWriteItem
	if offline {
		// Offline: sign now, keep the signed XML pending (contingency index), transmit later. No outbox row.
		signed, err := services.SignOnlyViaEgress(ctx, s.egress, orgPK, emitterDoc, emitUF, sefazEnvFor(environment), s3PrefixBpe, bpeBody)
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
	if err := s.bpeRepo.TransactReserveAndCreate(ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded, extraTx...); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número do BP-e. Tente novamente.")
		}
		return nil, err
	}
	return encoded, nil
}

// buildRecord is the DynamoDB row of the new BP-e (same index attributes as the other documents).
func buildRecord(
	pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue,
	number, serie int, now time.Time, req BpeEmitBody, body map[string]any, userID, userName string,
) map[string]any {
	// Off the record, like every other issuer document on this path.
	emitterDoc, _ := services.IssuerDocAV(orgItem, orgPK)
	total := ""
	if bpe, ok := body["BPe"].(map[string]any); ok {
		if inf, ok := bpe["infBPe"].(map[string]any); ok {
			if v, ok := inf["infValorBPe"].(map[string]any); ok {
				total, _ = v["vBP"].(string)
			}
		}
	}
	passenger := ""
	if req.Passage.Passenger != nil {
		passenger = req.Passage.Passenger.Name
	}
	return map[string]any{
		"pk": pk, "sk": accessKey, "incoming": 0,
		"year": now.Year(), "month": int(now.Month()), "day": now.Day(),
		"status": StatusPending, "sefaz_status": nil, "sefaz_motive": nil,
		"emit_cpf_cnpj": emitterDoc, "emit_name": strAttr(orgItem, "name"),
		"number": number, "serie": serie, "variant": VariantNormal, "modal": req.Modal,
		"passenger_name": passenger, "from_name": req.Passage.FromName, "to_name": req.Passage.ToName,
		"boarding": req.Passage.Boarding, "total": total, "trips": len(req.Trips),
		"dh_emi": now.Format(layoutDateTimeTZ), "xml_s3_key": nil,
		"created_at": now.UTC().Format(time.RFC3339), "user_id": userID, "user_name": userName,
	}
}
