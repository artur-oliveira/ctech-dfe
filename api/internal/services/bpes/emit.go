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

// emission descreve o que muda entre as variantes do BP-e; o resto do fluxo de emissão é o mesmo.
type emission struct {
	variant      string // normal, tm ou ta
	spec         string // família do esquema e das regras: bpe, bpetm ou bpeta
	sefazService string // BPeRecepcao, BPeRecepcaoTM ou BPeRecepcaoTA
	hasQR        bool   // o TM não tem QR Code, logo não assina o parâmetro sign na contingência
	reasonFrom   *string
	// substitution é o infBPeSub do substituto (nil no bilhete normal) e facts o que ele repete do original.
	substitution *BpeSubstitution
	facts        docFacts
	params       func(p *buildParams)
	build        func(p buildParams) map[string]any
	record       func(pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue, number, serie int, now time.Time, body map[string]any, userID, userName string) map[string]any
}

// Emit resolve a linha e as pessoas que o pedido aponta (ResolveTicket) e emite o bilhete comum ou o TA.
func (s *BpeService) Emit(ctx context.Context, orgPK string, req BpeTicketRequest, userID, userName string) (map[string]types.AttributeValue, error) {
	variant, normal, ta, err := s.ResolveTicket(ctx, orgPK, req)
	if err != nil {
		return nil, err
	}
	if variant == VariantTA {
		return s.emitTA(ctx, orgPK, ta, userID, userName)
	}
	return s.emitNormal(ctx, orgPK, normal, userID, userName)
}

// EmitTM resolve as linhas do TM (ResolveTM) e emite o bilhete metropolitano.
func (s *BpeService) EmitTM(ctx context.Context, orgPK string, req BpeTMRequest, userID, userName string) (map[string]types.AttributeValue, error) {
	body, err := s.ResolveTM(ctx, orgPK, req)
	if err != nil {
		return nil, err
	}
	return s.emitTM(ctx, orgPK, body, userID, userName)
}

// Emit emite o BP-e comum (modelos rodoviário, aquaviário e ferroviário). A variante TM tem corpo próprio: EmitTM.
func (s *BpeService) emitNormal(ctx context.Context, orgPK string, req BpeEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if req.Variant != "" && req.Variant != VariantNormal {
		return nil, problem.BadRequest("a variante " + req.Variant + " do BP-e ainda não está disponível para emissão por este corpo")
	}
	if err := validateEmitBody(req); err != nil {
		return nil, err
	}
	if err := validateCommonSubstitution(req.Substitution); err != nil {
		return nil, err
	}
	return s.emit(ctx, orgPK, emission{
		variant: VariantNormal, spec: specBPe, sefazService: sefazServiceAutorizacao, hasQR: true,
		reasonFrom: req.ContingencyReason, substitution: req.Substitution, facts: factsOf(req.Origin, req.Destination, req.Buyer),
		params: func(p *buildParams) { p.req = req },
		build:  BuildBPe,
		record: func(pk, key, org string, orgItem map[string]types.AttributeValue, number, serie int, now time.Time, body map[string]any, uid, uname string) map[string]any {
			return buildRecord(pk, key, org, orgItem, number, serie, now, req, body, uid, uname)
		},
	}, userID, userName)
}

// EmitTM emite o BP-e TM (transporte metropolitano), com serviço de recepção e leiaute próprios.
func (s *BpeService) emitTM(ctx context.Context, orgPK string, req BpeTMEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := validateTMBody(req); err != nil {
		return nil, err
	}
	return s.emit(ctx, orgPK, emission{
		variant: VariantTM, spec: specBPeTM, sefazService: sefazServiceAutorizacaoTM, hasQR: false,
		reasonFrom: req.ContingencyReason,
		params:     func(p *buildParams) { p.tm = req; p.variant = VariantTM },
		build:      BuildBPeTM,
		record: func(pk, key, org string, orgItem map[string]types.AttributeValue, number, serie int, now time.Time, body map[string]any, uid, uname string) map[string]any {
			return buildTMRecord(pk, key, org, orgItem, number, serie, now, req, body, uid, uname)
		},
	}, userID, userName)
}

// EmitTA emite o BP-e TA (transporte aéreo), com serviço de recepção e leiaute próprios (BPeRecepcaoTA, só SVRS).
func (s *BpeService) emitTA(ctx context.Context, orgPK string, req BpeTAEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := validateTABody(req, uftz.Now(taYearUF).Year()); err != nil {
		return nil, err
	}
	return s.emit(ctx, orgPK, emission{
		variant: VariantTA, spec: specBPeTA, sefazService: sefazServiceAutorizacaoTA, hasQR: true,
		reasonFrom: req.ContingencyReason, substitution: req.Substitution, facts: factsOf(req.Origin, req.Destination, req.Buyer),
		params: func(p *buildParams) { p.ta = req; p.variant = VariantTA },
		build:  BuildBPeTA,
		record: func(pk, key, org string, orgItem map[string]types.AttributeValue, number, serie int, now time.Time, body map[string]any, uid, uname string) map[string]any {
			return buildTARecord(pk, key, org, orgItem, number, serie, now, req, body, uid, uname)
		},
	}, userID, userName)
}

// buildTARecord is the DynamoDB row of a new BP-e TA (same index attributes as the other documents).
func buildTARecord(
	pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue,
	number, serie int, now time.Time, req BpeTAEmitBody, body map[string]any, userID, userName string,
) map[string]any {
	emitterDoc, _ := services.IssuerDocAV(orgItem, orgPK)
	total := ""
	if doc, ok := body["BPeTA"].(map[string]any); ok {
		if inf, ok := doc["infBPe"].(map[string]any); ok {
			if v, ok := inf["infValorBPe"].(map[string]any); ok {
				total, _ = v["vBP"].(string)
			}
		}
	}
	passenger := ""
	if req.Passage.Passenger != nil {
		passenger = req.Passage.Passenger.Name
	}
	from, to := "", ""
	if len(req.Trips) > 0 {
		from, to = req.Trips[0].FromAirport, req.Trips[len(req.Trips)-1].ToAirport
	}
	return map[string]any{
		"pk": pk, "sk": accessKey, "incoming": 0,
		"year": now.Year(), "month": int(now.Month()), "day": now.Day(),
		"status": StatusPending, "sefaz_status": nil, "sefaz_motive": nil,
		"emit_cpf_cnpj": emitterDoc, "emit_name": strAttr(orgItem, "name"),
		"number": number, "serie": serie, "variant": VariantTA, "modal": "aereo",
		"passenger_name": passenger, "from_name": from, "to_name": to, "purchase_type": purchaseOf(req.PurchaseType),
		"boarding": req.Passage.Boarding, "total": total, "trips": len(req.Trips),
		"dh_emi": now.Format(layoutDateTimeTZ), "xml_s3_key": nil,
		"created_at": now.UTC().Format(time.RFC3339), "user_id": userID, "user_name": userName,
	}
}

// emit reserva número e cota, monta o documento da variante e o enfileira pelo outbox (online) ou o assina agora e o guarda
// para a retransmissão (contingência offline).
func (s *BpeService) emit(ctx context.Context, orgPK string, em emission, userID, userName string) (map[string]types.AttributeValue, error) {
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
		if contingencyReason, err = resolveContingencyReason(em.reasonFrom, cfgReason); err != nil {
			return nil, err
		}
	}

	now := uftz.Now(emitUF) // dhEmi e AAMM da chave são horário local da UF do emitente
	if em.substitution != nil {
		if err := s.checkSubstitution(ctx, orgPK, em.variant, em.substitution, em.facts, now); err != nil {
			return nil, err
		}
	}
	accessKey := GenerateAccessKey(emitUF, emitterDoc, serie, currentNumber+1, now, tpEmis)
	if accessKey == "" {
		return nil, problem.BadRequest("UF do emitente inválida: " + emitUF)
	}
	// Offline: o parâmetro sign do QR Code é a assinatura da chave de acesso (egress SignText). O TM não tem QR Code.
	qrSign := ""
	if offline && em.hasQR {
		if qrSign, err = services.SignTextViaEgress(ctx, s.egress, orgPK, emitterDoc, s3PrefixBpe, accessKey); err != nil {
			return nil, err
		}
	}

	params := buildParams{
		org: orgItem, orgPK: orgPK, accessKey: accessKey, serie: serie, number: currentNumber + 1,
		environment: environment, now: now, variant: em.variant, tpEmis: tpEmis, qrSign: qrSign,
		contingencyReason: contingencyReason, tech: s.tech,
	}
	em.params(&params)
	bpeBody := em.build(params)
	// Portão local (422) antes de debitar a cota e de gravar o outbox: regras do MOC promovidas e schema.
	if err := gateBPe(em.spec, bpeBody); err != nil {
		return nil, err
	}
	dfecheck.ShadowBPe(em.spec, accessKey, bpeBody)

	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	record := em.record(pk, accessKey, orgPK, orgItem, currentNumber+1, serie, now, bpeBody, userID, userName)
	if em.variant != VariantTM {
		for k, v := range em.facts.attrs() {
			record[k] = v
		}
	}
	substitutes := ""
	if em.substitution != nil {
		substitutes = em.substitution.OriginalKey
		record["replaces_access_key"] = substitutes
	}
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
		CNPJ: emitterDoc, UF: authorizerUF(em.variant, emitUF), SefazEnvironment: sefazEnvFor(environment),
		CertS3Key: strAttr(cert, "s3_key"), CertPassword: strAttr(cert, "password"),
		DocType: s3PrefixBpe, SefazService: em.sefazService, Body: bpeBody, SubstitutesAccessKey: substitutes,
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

// buildTMRecord is the DynamoDB row of a new BP-e TM (same index attributes as the other documents).
func buildTMRecord(
	pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue,
	number, serie int, now time.Time, req BpeTMEmitBody, body map[string]any, userID, userName string,
) map[string]any {
	emitterDoc, _ := services.IssuerDocAV(orgItem, orgPK)
	total, qPass := "", ""
	if doc, ok := body["BPeTM"].(map[string]any); ok {
		if inf, ok := doc["infBPe"].(map[string]any); ok {
			if t, ok := inf["total"].(map[string]any); ok {
				total, _ = t["vBP"].(string)
				qPass, _ = t["qPass"].(string)
			}
		}
	}
	items := 0
	for _, g := range req.Groups {
		items += len(g.Items)
	}
	return map[string]any{
		"pk": pk, "sk": accessKey, "incoming": 0,
		"year": now.Year(), "month": int(now.Month()), "day": now.Day(),
		"status": StatusPending, "sefaz_status": nil, "sefaz_motive": nil,
		"emit_cpf_cnpj": emitterDoc, "emit_name": strAttr(orgItem, "name"),
		"number": number, "serie": serie, "variant": VariantTM, "modal": req.Modal,
		"competence": req.Competence, "total": total, "passengers": qPass, "groups": len(req.Groups), "items": items,
		"dh_emi": now.Format(layoutDateTimeTZ), "xml_s3_key": nil,
		"created_at": now.UTC().Format(time.RFC3339), "user_id": userID, "user_name": userName,
	}
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

// purchaseOf devolve o tipo de compra do TA, com normal como padrão.
func purchaseOf(p string) string {
	if p == "" {
		return PurchaseNormal
	}
	return p
}
