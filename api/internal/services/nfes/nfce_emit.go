package nfes

// nfce_emit.go implements NFC-e issuance (modelo 65).

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// NfceEmitBody is the JSON body for POST /nfces. NFC-e has no recipient address,
// transport, or billing/duplicatas — the consumer is optional (CPF only).
type NfceEmitBody struct {
	ConsumerCPF    *string          `json:"consumer_cpf" validate:"omitempty,cpf"`
	Products       []NfeProductItem `json:"products" validate:"required,min=1,dive"`
	Payments       []NfePaymentItem `json:"payments" validate:"omitempty,dive"`
	AdditionalInfo *string          `json:"additional_info" validate:"omitempty,max=5000"`
	NatOp          *string          `json:"nat_op" validate:"omitempty,max=60"`
	// OperationID é opcional e normalmente ausente: a venda de balcão usa a
	// operação padrão da organização, sem passo nem campo novo na tela.
	OperationID *string `json:"operation_id" validate:"omitempty"`
}

// nfceFinNFe / nfceIndFinal / nfceIndPres are fixed for NFC-e:
// finalidade normal, consumidor final, operação presencial.
const (
	nfceFinNFe   = "1"
	nfceIndFinal = "1"
	nfceIndPres  = "1"
	nfceTpNF     = "1" // saída
)

// Emit resolves NFC-e data, builds the enviNFe (modelo 65) with QR Code,
// reserves a fiscal number, persists the record, and dispatches to the worker.
func (s *NfceService) Emit(ctx context.Context, orgPK string, req NfceEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if len(req.Products) == 0 {
		return nil, problem.BadRequest("pelo menos um produto é obrigatório")
	}
	if len(req.Payments) == 0 {
		return nil, problem.BadRequest("pelo menos uma forma de pagamento é obrigatória")
	}

	// Consumer is optional; when present it must be a CPF (pessoa física).
	var consumerCPF string
	if req.ConsumerCPF != nil {
		consumerCPF = onlyDigits(*req.ConsumerCPF)
		if consumerCPF != "" && len(consumerCPF) != 11 {
			return nil, problem.BadRequest("a NFC-e aceita apenas CPF de pessoa física no consumidor")
		}
	}

	orgItem, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if orgItem == nil {
		return nil, problem.NotFound("organização não encontrada")
	}

	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, problem.NoCertificate("certificado digital não encontrado")
	}
	cert := certs[0]

	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if configItem == nil {
		return nil, problem.BadRequest("configure a NFC-e em Configuração Fiscal antes de emitir")
	}

	environment := intAttr(configItem, "environment", 2)
	envPrefix := envToPrefix(environment)
	serie := intAttr(configItem, fmt.Sprintf("%s_current_serie", envPrefix), 1)
	currentNumber := intAttr(configItem, fmt.Sprintf("%s_current_number", envPrefix), 0)
	cscID := fmt.Sprintf("%d", intAttr(configItem, fmt.Sprintf("%s_csc_id", envPrefix), 0))
	csc := strAttr(configItem, fmt.Sprintf("%s_csc", envPrefix))

	// A operação da NFC-e é implícita: a informada no request, senão a padrão da
	// organização. Nenhum passo ou campo novo aparece na tela de balcão.
	operation, err := s.resolveNfceOperation(ctx, orgPK, req.OperationID)
	if err != nil {
		return nil, err
	}

	emitUF := extractEmitUFFromItem(orgItem)

	// NFC-e é sempre operação interna, então o escopo do CFOP resolvido é
	// sempre 5 — passar emitUF nas duas pontas é o que expressa isso.
	items := make([]NfeProductItem, len(req.Products))
	for i, item := range req.Products {
		cfop, err := resolveItemCFOP(item.CFOP, operation, emitUF, emitUF)
		if err != nil {
			return nil, err
		}
		// CFOP tem que ser saída interna (prefixo "5"): NFC-e não vale para
		// operação interestadual nem de entrada.
		if !strings.HasPrefix(cfop, services.CFOPScopeIntraUF) {
			return nil, problem.BadRequest(
				fmt.Sprintf("CFOP %s não é permitido em NFC-e (apenas operações internas de saída — CFOP 5xxx)", cfop))
		}
		item.CFOP = cfop
		items[i] = item
	}

	productItems, totalProducts, totalDiscount, err := resolveProducts(ctx, s.productRepo, s.taxProfileRepo, orgPK, items)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	accessKey, err := generateAccessKey(orgPK, orgItem, serie, currentNumber, now, nfModel65)
	if err != nil {
		return nil, err
	}

	supl, err := buildNFCeSupl(emitUF, environment, accessKey, cscID, csc)
	if err != nil {
		return nil, err
	}

	orgAny, err := unmarshalToAny(orgItem)
	if err != nil {
		return nil, problem.InternalServer("failed to decode org")
	}
	orgAny["sk"] = orgPK

	// Optional consumer (dest) — CPF only, name resolved from persons when known.
	var receiverAny map[string]any
	destName := ""
	destDoc := ""
	if consumerCPF != "" {
		destDoc = consumerCPF
		receiverSK := "CPF_" + consumerCPF
		if person, perr := s.personRepo.Get(ctx, orgPK, receiverSK); perr == nil && person != nil {
			destName = strAttr(person, "name")
		}
		receiverAny = map[string]any{"sk": receiverSK, "name": destName}
	}

	// Payments + troco.
	paymentsAny := make([]map[string]any, 0, len(req.Payments))
	summaryPayments := make([]map[string]any, 0, len(req.Payments))
	totalPaid := decimal.Zero
	for _, p := range req.Payments {
		pm := map[string]any{"payment_type": p.PaymentType, "value": p.Value}
		if p.IndPag != nil {
			pm["ind_pag"] = *p.IndPag
		}
		if p.Card != nil {
			pm["card"] = p.Card
		}
		paymentsAny = append(paymentsAny, pm)
		summaryPayments = append(summaryPayments, map[string]any{"payment_type": p.PaymentType, "value": p.Value})
		totalPaid = totalPaid.Add(d(p.Value))
	}

	totalNFe := totalProducts.Sub(totalDiscount).RoundBank(2)
	var vTroco *string
	if troco := totalPaid.Sub(totalNFe); troco.GreaterThan(decimal.Zero) {
		vTroco = new(q2(troco.RoundBank(2)))
	}

	enviNFe := BuildEnviNFe(
		orgAny, receiverAny, orgPK,
		productItems, paymentsAny,
		currentNumber, serie, environment,
		accessKey, totalProducts, totalDiscount,
		req.AdditionalInfo, now,
		firstNonNil(req.NatOp, operationDefault(operation, opFieldNatOp)),
		nfceFinNFe, nfceIndFinal, nfceIndPres, nfceTpNF,
		nil, nil, nil, vTroco,
		s.tech, nfModel65, supl,
		nil, nil,
	)

	summaryProducts := make([]map[string]any, 0, len(productItems))
	for _, p := range productItems {
		summaryProducts = append(summaryProducts, map[string]any{
			"product_id": p["product_id"], "product_code": p["product_code"],
			"description": p["description"], "ncm": p["ncm"], "cfop": p["cfop"],
			"unit": p["unit"], "quantity": p["quantity"], "unit_value": p["unit_value"],
			"discount": p["discount"], "total": p["total"],
		})
	}

	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	nfceRecord := map[string]any{
		"pk":            pk,
		"sk":            accessKey,
		"incoming":      NotIncoming,
		"year":          now.Year(),
		"month":         int(now.Month()),
		"day":           now.Day(),
		"status":        StatusPending,
		"sefaz_status":  nil,
		"sefaz_motive":  nil,
		"emit_cpf_cnpj": services.StripPKPrefix(orgPK),
		"emit_name":     strAttr(orgItem, "name"),
		"dest_cpf_cnpj": destDoc,
		"dest_name":     destName,
		"number":        currentNumber,
		"serie":         serie,
		"total":         q2(totalNFe),
		"dh_emi":        fmtDhEmi(now),
		"xml_s3_key":    nil,
		"products":      summaryProducts,
		"payments":      summaryPayments,
		"created_at":    now.UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_name":     userName,
	}

	nfceEncoded, err := repositories.EncodeItem(nfceRecord)
	if err != nil {
		return nil, problem.InternalServer("failed to encode NFC-e record")
	}

	sefazEnv := SefazEnvHom
	if environment == 1 {
		sefazEnv = SefazEnvProd
	}
	workerMsg := services.WorkerMessage{
		DocPK:            pk,
		AccessKey:        accessKey,
		TableName:        "nfces",
		S3Prefix:         "nfce",
		ExpectedFileName: accessKey,
		CNPJ:             services.StripPKPrefix(orgPK),
		UF:               emitUF,
		SefazEnvironment: sefazEnv,
		CertS3Key:        strAttr(cert, "s3_key"),
		CertPassword:     strAttr(cert, "password"),
		DocType:          "nfce",
		SefazService:     "NFeAutorizacao",
		Body:             enviNFe,
	}
	outboxTx, operationID, err := s.workerSvc.BuildOutboxTx(workerMsg)
	if err != nil {
		return nil, err
	}
	nfceEncoded["operation_id"] = &types.AttributeValueMemberS{Value: operationID}

	if err := s.nfceRepo.TransactReserveAndCreate(
		ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, nfceEncoded, outboxTx,
	); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número da NFC-e. Tente novamente.")
		}
		return nil, err
	}

	return nfceEncoded, nil
}

// onlyDigits strips all non-digit characters.
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
