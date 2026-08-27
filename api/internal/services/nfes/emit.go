package nfes

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// --- Request types ---

// NfeEmitBody is the JSON body for POST /nfes. Structural validation tags
// enforce presence/format; cross-field business rules (receiver_id XOR
// self_issuance, etc.) remain in Emit.
type NfeEmitBody struct {
	ReceiverID *string `json:"receiver_id" validate:"omitempty"`
	// OperationID resolve nat_op, tp_nf, fin_nfe, ind_final, ind_pres, mod_frete,
	// o CFOP dos itens que não o trazem, e as mensagens fiscais. Todo valor
	// explícito no request vence a operação.
	OperationID *string `json:"operation_id" validate:"omitempty"`
	// PaymentTermID expande para payments, cobr_fat e cobr_duplicatas a partir
	// do total do documento. Valores explícitos no request vencem a expansão.
	PaymentTermID  *string            `json:"payment_term_id" validate:"omitempty"`
	SelfIssuance   bool               `json:"self_issuance"`
	Products       []NfeProductItem   `json:"products" validate:"required,min=1,dive"`
	Payments       []NfePaymentItem   `json:"payments" validate:"omitempty,dive"`
	AdditionalInfo *string            `json:"additional_info" validate:"omitempty,max=5000"`
	NatOp          *string            `json:"nat_op" validate:"omitempty,max=60"`
	FinNFe         *string            `json:"fin_nfe" validate:"omitempty,oneof=1 2 3 4"`
	IndFinal       *string            `json:"ind_final" validate:"omitempty,oneof=0 1"`
	IndPres        *string            `json:"ind_pres" validate:"omitempty,oneof=0 1 2 3 4 5 9"`
	TpNF           *string            `json:"tp_nf" validate:"omitempty,oneof=0 1"`
	Transport      *NfeTransportItem  `json:"transport" validate:"omitempty"`
	CobrFat        *NfeFatItem        `json:"cobr_fat" validate:"omitempty"`
	CobrDuplicatas []NfeDuplicataItem `json:"cobr_duplicatas" validate:"omitempty,dive"`
	VTroco         *string            `json:"v_troco" validate:"omitempty,money"`

	Retirada             *NfeLocalBody `json:"retirada" validate:"omitempty"`
	Entrega              *NfeLocalBody `json:"entrega" validate:"omitempty"`
	SaveRetiradaLocation bool          `json:"save_retirada_location"`
	SaveEntregaLocation  bool          `json:"save_entrega_location"`

	// ProcRef são processos referenciados (infAdic/procRef): nº do processo e
	// sua origem. indProc: 0 SEFAZ, 1 Justiça Federal, 2 Justiça Estadual,
	// 3 Secex/RFB, 9 outros.
	ProcRef []NfeProcRefBody `json:"proc_ref" validate:"omitempty,max=100,dive"`

	// NFRefs são os documentos referenciados (ide/NFref). finNFe 2/3/4 é
	// rejeitado sem pelo menos um. `nfe_id` referencia uma nota da própria base
	// e dispensa o resto: chave e tipo saem do registro.
	NFRefs []NfeRefBody `json:"nf_refs" validate:"omitempty,max=500,dive"`
}

// NfeProcRefBody é um processo referenciado (infAdic/procRef).
type NfeProcRefBody struct {
	NProc   string  `json:"n_proc" validate:"required,max=60"`
	IndProc string  `json:"ind_proc" validate:"required,oneof=0 1 2 3 9"`
	TpAto   *string `json:"tp_ato" validate:"omitempty,len=2,number"`
}

// NfeRefBody é uma referência de documento em ide/NFref. Ou `nfe_id` (uma nota
// desta organização, de onde chave e tipo são derivados), ou os campos do
// documento de fora do sistema.
type NfeRefBody struct {
	NfeID *string `json:"nfe_id" validate:"omitempty"`
	Kind  *string `json:"kind" validate:"omitempty,oneof=nfe nfesig nf nfp cte ecf"`
	// Chave de 44 dígitos, para kind nfe/nfesig/cte informados manualmente.
	AccessKey *string `json:"access_key" validate:"omitempty,len=44,number"`
	CUF       *string `json:"c_uf" validate:"omitempty,len=2,number"`
	AAMM      *string `json:"aamm" validate:"omitempty,len=4,number"`
	CNPJ      *string `json:"cnpj" validate:"omitempty,cnpj"`
	CPF       *string `json:"cpf" validate:"omitempty,cpf"`
	IE        *string `json:"ie" validate:"omitempty,max=14"`
	Mod       *string `json:"mod" validate:"omitempty,max=2"`
	Serie     *string `json:"serie" validate:"omitempty,max=3,number"`
	NNF       *string `json:"n_nf" validate:"omitempty,max=9,number"`
	NECF      *string `json:"n_ecf" validate:"omitempty,max=3,number"`
	NCOO      *string `json:"n_coo" validate:"omitempty,max=6,number"`
}

// NfeProductItem is a line item in an NF-e emission request.
type NfeProductItem struct {
	// ProductID é obrigatório; ServiceID é opcional e aponta para o catálogo de
	// serviços da NFS-e (organization_services), reusado na NF-e mista: item da
	// lista, código e alíquota do ISS saem de lá, nunca de um segundo cadastro.
	ProductID string  `json:"product_id" validate:"required"`
	ServiceID *string `json:"service_id" validate:"omitempty"`
	// Vazio é aceito quando a emissão informa uma operação com cfop_suffix —
	// aí o CFOP é resolvido pelas UFs de emitente e destinatário.
	CFOP       string           `json:"cfop" validate:"omitempty,cfop"`
	Quantity   string           `json:"quantity" validate:"required,decimalv"`
	UnitValue  *string          `json:"unit_value" validate:"omitempty,money"`
	Discount   string           `json:"discount" validate:"omitempty,money"`
	VFrete     *string          `json:"v_frete" validate:"omitempty,money"`
	VSeg       *string          `json:"v_seg" validate:"omitempty,money"`
	VOutro     *string          `json:"v_outro" validate:"omitempty,money"`
	VeicChassi *string          `json:"veic_chassi" validate:"omitempty"`
	VeicNSerie *string          `json:"veic_n_serie" validate:"omitempty"`
	VeicNMotor *string          `json:"veic_n_motor" validate:"omitempty"`
	VeicCCor   *string          `json:"veic_c_cor" validate:"omitempty"`
	VeicXCor   *string          `json:"veic_x_cor" validate:"omitempty"`
	Armas      []map[string]any `json:"armas" validate:"omitempty"`
	// PDevol é o percentual devolvido do item (impostoDevol). Só vale em nota
	// de devolução (finNFe=4).
	PDevol *string `json:"p_devol" validate:"omitempty,percent"`
}

// NfePaymentItem is a payment method in an NF-e emission request.
type NfePaymentItem struct {
	PaymentType string         `json:"payment_type" validate:"required,oneof=01 02 03 04 05 10 11 12 13 14 15 16 17 18 19 20 21 22 90 99"`
	Value       string         `json:"value" validate:"required,money"`
	IndPag      *string        `json:"ind_pag" validate:"omitempty,oneof=0 1"`
	DPag        *string        `json:"d_pag" validate:"omitempty"`
	Card        map[string]any `json:"card" validate:"omitempty"`
	// TerminalID aponta um organization_payment_terminals: CNPJReceb, idTermPag,
	// CNPJPag/UFPag e a bandeira default saem do cadastro.
	TerminalID *string `json:"terminal_id" validate:"omitempty"`
	// XPag descreve a forma de pagamento quando tPag é 99 (outros).
	XPag *string `json:"x_pag" validate:"omitempty,max=60"`
}

// NfeTransportItem holds transport data for an NF-e emission request.
type NfeTransportItem struct {
	ModFrete        string  `json:"mod_frete" validate:"required,oneof=0 1 2 3 4 9"`
	TransportaPK    *string `json:"transporta_pk" validate:"omitempty"`
	TransportaCNPJ  *string `json:"transporta_cnpj" validate:"omitempty,cnpj"`
	TransportaCPF   *string `json:"transporta_cpf" validate:"omitempty,cpf"`
	TransportaNome  *string `json:"transporta_nome" validate:"omitempty,max=60"`
	TransportaIE    *string `json:"transporta_ie" validate:"omitempty,max=20"`
	TransportaEnder *string `json:"transporta_ender" validate:"omitempty,max=255"`
	TransportaMun   *string `json:"transporta_mun" validate:"omitempty,max=120"`
	TransportaUF    *string `json:"transporta_uf" validate:"omitempty,uf"`
	VeiculoSK       *string `json:"veiculo_sk" validate:"omitempty"`
	VeiculoPlaca    *string `json:"veiculo_placa" validate:"omitempty,placa"`
	VeiculoUF       *string `json:"veiculo_uf" validate:"omitempty,uf"`
	VeiculoRNTRC    *string `json:"veiculo_rntrc" validate:"omitempty,rntrc"`
	// Vols são os volumes transportados (transp/vol). Ausente, o builder deriva
	// um volume único com o peso somado dos itens.
	Vols []NfeVolBody `json:"vols" validate:"omitempty,max=5000,dive"`
	// Reboques do veículo transportador (transp/reboque, máx 5 pelo XSD).
	Reboques []NfeReboqueBody `json:"reboques" validate:"omitempty,max=5,dive"`
}

// NfeVolBody é um volume transportado (transp/vol).
type NfeVolBody struct {
	QVol   *string  `json:"q_vol" validate:"omitempty,max=15,number"`
	Esp    *string  `json:"esp" validate:"omitempty,max=60"`
	Marca  *string  `json:"marca" validate:"omitempty,max=60"`
	NVol   *string  `json:"n_vol" validate:"omitempty,max=60"`
	PesoL  *string  `json:"peso_l" validate:"omitempty,weight3"`
	PesoB  *string  `json:"peso_b" validate:"omitempty,weight3"`
	Lacres []string `json:"lacres" validate:"omitempty,max=5000,dive,max=60"`
}

// NfeReboqueBody é um reboque do veículo transportador (transp/reboque).
type NfeReboqueBody struct {
	Placa string  `json:"placa" validate:"required,placa"`
	UF    string  `json:"uf" validate:"required,uf"`
	RNTC  *string `json:"rntc" validate:"omitempty,max=20"`
}

// NfeFatItem is the invoice header (cobr.fat) in an NF-e emission request.
type NfeFatItem struct {
	NFat  *string `json:"n_fat" validate:"omitempty,max=60"`
	VOrig *string `json:"v_orig" validate:"omitempty,money"`
	VDesc *string `json:"v_desc" validate:"omitempty,money"`
	VLiq  *string `json:"v_liq" validate:"omitempty,money"`
}

// NfeDuplicataItem is a billing installment (cobr.dup) in an NF-e emission request.
type NfeDuplicataItem struct {
	NDup  *string `json:"n_dup" validate:"omitempty,max=60"`
	DVenc *string `json:"d_venc" validate:"omitempty"`
	VDup  string  `json:"v_dup" validate:"required,money"`
}

// NfeLocalBody is a TLocal-shaped address (local de retirada/entrega) — a
// lighter shape than TEndereco (AddressBody): no CEP/postal code in the XSD.
type NfeLocalBody struct {
	CNPJ    *string `json:"cnpj" validate:"omitempty,cnpj"`
	CPF     *string `json:"cpf" validate:"omitempty,cpf"`
	XNome   *string `json:"x_nome" validate:"omitempty,max=60"`
	XLgr    string  `json:"x_lgr" validate:"required,max=255"`
	Nro     string  `json:"nro" validate:"required,max=60"`
	XCpl    *string `json:"x_cpl" validate:"omitempty,max=60"`
	XBairro string  `json:"x_bairro" validate:"required,max=60"`
	CMun    string  `json:"c_mun" validate:"required,ibge"`
	XMun    string  `json:"x_mun" validate:"required,max=60"`
	UF      string  `json:"uf" validate:"required,uf"`
	Fone    *string `json:"fone" validate:"omitempty,phonebr"`
	Email   *string `json:"email" validate:"omitempty,email"`
}

// Emit resolves all NF-e data, builds the full enviNFe structure, atomically reserves
// a fiscal number, stores the DynamoDB record, and dispatches to worker via SQS.
func (s *NfeService) Emit(ctx context.Context, orgPK string, req NfeEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	if req.SelfIssuance && req.ReceiverID != nil {
		return nil, problem.BadRequest("informe receiver_id ou self_issuance=true, não ambos")
	}
	if !req.SelfIssuance && req.ReceiverID == nil {
		return nil, problem.BadRequest("receiver_id é obrigatório quando self_issuance=false")
	}
	if len(req.Products) == 0 {
		return nil, problem.BadRequest("pelo menos um produto é obrigatório")
	}
	// A checagem de pagamento vive depois da expansão da condição de pagamento:
	// quem manda payment_term_id não manda payments.

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
		return nil, problem.BadRequest("configure a NF-e em Configuração Fiscal antes de emitir")
	}

	environment := intAttr(configItem, "environment", 2)
	envPrefix := envToPrefix(environment)
	serie := intAttr(configItem, fmt.Sprintf("%s_current_serie", envPrefix), 1)
	currentNumber := intAttr(configItem, fmt.Sprintf("%s_current_number", envPrefix), 0)

	var receiverItem map[string]types.AttributeValue
	var receiverSK string
	if req.SelfIssuance {
		receiverItem = orgItem
		receiverSK = orgPK
	} else {
		receiverItem, err = s.personRepo.Get(ctx, orgPK, *req.ReceiverID)
		if err != nil {
			return nil, err
		}
		if receiverItem == nil {
			return nil, problem.NotFound("pessoa não encontrada: " + *req.ReceiverID)
		}
		receiverSK = *req.ReceiverID
	}

	operation, err := loadOperation(ctx, s.operationRepo, orgPK, req.OperationID)
	if err != nil {
		return nil, err
	}

	emitUF := extractEmitUFFromItem(orgItem)
	destUF := extractEmitUFFromItem(receiverItem)
	items := make([]NfeProductItem, len(req.Products))
	for i, item := range req.Products {
		cfop, err := resolveItemCFOP(item.CFOP, operation, emitUF, destUF)
		if err != nil {
			return nil, err
		}
		item.CFOP = cfop
		items[i] = item
	}

	productItems, totalProducts, totalDiscount, err := resolveProducts(ctx, s.productRepo, s.taxProfileRepo, orgPK, destUF, items)
	if err == nil {
		err = applyServiceDefaults(ctx, s.serviceRepo, orgPK, items, productItems)
	}
	if err != nil {
		return nil, err
	}

	// C0: a forma de emissão é resolvida antes da chave (tpEmis está na chave).
	// Hoje sempre normal; a seleção automática entra com a máquina de estados de
	// contingência (fase C2 do plano).
	mode := NormalEmission(nfModel55)
	if err := mode.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	accessKey, err := generateAccessKey(orgPK, orgItem, serie, currentNumber, now, nfModel55, mode.TpEmis)
	if err != nil {
		return nil, err
	}

	resolvedTransport, err := s.resolveTransport(ctx, orgPK, req.Transport)
	if err != nil {
		return nil, err
	}

	nfRefs, err := s.resolveNFRefs(ctx, orgPK, envPrefix, req.NFRefs)
	if err != nil {
		return nil, err
	}

	terminals, err := resolvePaymentTerminals(ctx, s.paymentTerminalRepo, orgPK, req.Payments)
	if err != nil {
		return nil, err
	}

	// Unmarshal DynamoDB items to plain maps for BuildEnviNFe
	orgAny, err := unmarshalToAny(orgItem)
	if err != nil {
		return nil, problem.InternalServer("failed to decode org")
	}
	// Attach sk so BuildEnviNFe can determine isDestPJ
	orgAny["sk"] = orgPK

	receiverAny, err := unmarshalToAny(receiverItem)
	if err != nil {
		return nil, problem.InternalServer("failed to decode receiver")
	}
	receiverAny["sk"] = receiverSK

	// Condição de pagamento: a do request, senão a que a operação define.
	// A expansão só entra onde o request não trouxe nada — pagamento explícito
	// vence sempre.
	termID := firstNonNil(req.PaymentTermID, operationDefault(operation, "payment_term_id"))
	term, err := loadPaymentTerm(ctx, s.paymentTermRepo, orgPK, termID)
	if err != nil {
		return nil, err
	}
	expandedPayments, expandedFat, expandedDups, err := ExpandPaymentTerm(
		term, totalProducts.Sub(totalDiscount).RoundBank(2), now)
	if err != nil {
		return nil, err
	}
	if len(req.Payments) == 0 {
		req.Payments = expandedPayments
	}
	if req.CobrFat == nil {
		req.CobrFat = expandedFat
	}
	if len(req.CobrDuplicatas) == 0 {
		req.CobrDuplicatas = expandedDups
	}
	if len(req.Payments) == 0 {
		return nil, problem.BadRequest("pelo menos uma forma de pagamento é obrigatória")
	}

	// Resolve optional billing nodes
	var cobrFatAny map[string]any
	if req.CobrFat != nil {
		cobrFatAny = map[string]any{
			"n_fat":  ptrStr(req.CobrFat.NFat),
			"v_orig": ptrStr(req.CobrFat.VOrig),
			"v_desc": ptrStr(req.CobrFat.VDesc),
			"v_liq":  ptrStr(req.CobrFat.VLiq),
		}
	}
	var cobrDupAny []map[string]any
	for _, dup := range req.CobrDuplicatas {
		cobrDupAny = append(cobrDupAny, map[string]any{
			"n_dup":  ptrStr(dup.NDup),
			"d_venc": ptrStr(dup.DVenc),
			"v_dup":  dup.VDup,
		})
	}

	// Payment maps for builder (includes full payment data for pag node)
	paymentsAny := make([]map[string]any, 0, len(req.Payments))
	summaryPayments := make([]map[string]any, 0, len(req.Payments))
	for _, p := range req.Payments {
		pm := map[string]any{"payment_type": p.PaymentType, "value": p.Value}
		if p.IndPag != nil {
			pm["ind_pag"] = *p.IndPag
		}
		if p.DPag != nil {
			pm["d_pag"] = *p.DPag
		}
		if p.TerminalID != nil {
			pm["terminal_id"] = terminalSK(*p.TerminalID)
		}
		if p.XPag != nil {
			pm["x_pag"] = *p.XPag
		}
		if p.Card != nil {
			pm["card"] = p.Card
		}
		paymentsAny = append(paymentsAny, pm)
		summaryPayments = append(summaryPayments, map[string]any{
			"payment_type": p.PaymentType,
			"value":        p.Value,
		})
	}

	// Escada: valor no request → operação → default do leiaute.
	finNFe := strOrDefault(ptrStr(firstNonNil(req.FinNFe, operationDefault(operation, opFieldFinNFe))), "1")
	indFinal := strOrDefault(ptrStr(firstNonNil(req.IndFinal, operationDefault(operation, opFieldIndFinal))), "1")
	indPres := strOrDefault(ptrStr(firstNonNil(req.IndPres, operationDefault(operation, opFieldIndPres))), "1")
	tpNF := strOrDefault(ptrStr(firstNonNil(req.TpNF, operationDefault(operation, opFieldTpNF))), "1")
	natOp := firstNonNil(req.NatOp, operationDefault(operation, opFieldNatOp))

	if finNFeExigeRef[finNFe] && len(nfRefs) == 0 {
		return nil, problem.BadRequest("finNFe " + finNFe + " exige pelo menos um documento em nf_refs")
	}
	if err := validateDevolucao(finNFe, req.Products); err != nil {
		return nil, err
	}

	// Mensagens fiscais da operação, com os placeholders já interpolados.
	interpVars := map[string]string{
		services.PlaceholderVNF:     q2(totalProducts.Sub(totalDiscount)),
		services.PlaceholderCliente: anyStr(receiverAny, "name", ""),
		services.PlaceholderNatOp:   ptrStr(natOp),
	}
	infCpl, err := interpolateOperationText(operation, opFieldInfCpl, interpVars)
	if err != nil {
		return nil, err
	}
	additionalInfo := firstNonNil(req.AdditionalInfo, infCpl)

	infAdFisco, err := interpolateOperationText(operation, opFieldInfAdFisco, interpVars)
	if err != nil {
		return nil, err
	}

	// Build full enviNFe structure
	enviNFe := BuildEnviNFe(
		orgAny, receiverAny, orgPK,
		productItems, paymentsAny,
		currentNumber, serie, environment,
		accessKey, totalProducts, totalDiscount,
		additionalInfo, now,
		natOp, finNFe, indFinal, indPres, tpNF,
		resolvedTransport, cobrFatAny, cobrDupAny, req.VTroco,
		s.tech, nfModel55, nil,
		req.Retirada, req.Entrega,
		mode,
		docExtras{
			NFRefs: nfRefs,
			Vols: buildVols(transportVols(req.Transport),
				ptrStr(operationDefault(operation, opFieldVolEsp)),
				ptrStr(operationDefault(operation, opFieldVolMarca))),
			Reboques:         buildReboques(transportReboques(req.Transport)),
			InfAdFisco:       ptrStr(infAdFisco),
			ObsCont:          operationObs(operation, opFieldObsCont, interpVars),
			ObsFisco:         operationObs(operation, opFieldObsFisco, interpVars),
			ProcRef:          buildProcRef(req.ProcRef),
			PaymentTerminals: terminals,
			RetTrib:          operationGroup(operation, opFieldRetTrib),
			FinNFe4:          finNFe == finNFeDevolucao,
			CsrtID:           strAttr(configItem, csrtIDField),
			Csrt:             strAttr(configItem, csrtField),
		},
	)

	// Summary products for DynamoDB record
	summaryProducts := make([]map[string]any, 0, len(productItems))
	for _, p := range productItems {
		summaryProducts = append(summaryProducts, map[string]any{
			"product_id":   p["product_id"],
			"product_code": p["product_code"],
			"description":  p["description"],
			"ncm":          p["ncm"],
			"cfop":         p["cfop"],
			"unit":         p["unit"],
			"quantity":     p["quantity"],
			"unit_value":   p["unit_value"],
			"discount":     p["discount"],
			"total":        p["total"],
		})
	}

	totalNFe := totalProducts.Sub(totalDiscount)
	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	nfeRecord := map[string]any{
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
		"dest_cpf_cnpj": services.StripPKPrefix(receiverSK),
		"dest_name":     strAttr(receiverItem, "name"),
		"number":        currentNumber,
		"serie":         serie,
		"total":         q2(totalNFe.RoundBank(2)),
		"dh_emi":        fmtDhEmi(now),
		"xml_s3_key":    nil,
		"products":      summaryProducts,
		"payments":      summaryPayments,
		"created_at":    now.UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_name":     userName,
	}

	nfeEncoded, err := repositories.EncodeItem(nfeRecord)
	if err != nil {
		return nil, problem.InternalServer("failed to encode NF-e record")
	}

	sefazEnv := SefazEnvHom
	if environment == 1 {
		sefazEnv = SefazEnvProd
	}
	workerMsg := services.WorkerMessage{
		DocPK:            pk,
		AccessKey:        accessKey,
		TableName:        "nfes",
		S3Prefix:         "nfe",
		ExpectedFileName: accessKey,
		CNPJ:             services.StripPKPrefix(orgPK),
		UF:               emitUF,
		SefazEnvironment: sefazEnv,
		CertS3Key:        strAttr(cert, "s3_key"),
		CertPassword:     strAttr(cert, "password"),
		DocType:          "nfe",
		SefazService:     "NFeAutorizacao",
		Body:             enviNFe,
	}
	outboxTx, operationID, err := s.workerSvc.BuildOutboxTx(workerMsg)
	if err != nil {
		return nil, err
	}
	nfeEncoded["operation_id"] = &types.AttributeValueMemberS{Value: operationID}

	// The quota is claimed **before** the write, and before anything reaches
	// SEFAZ. Counting authorised documents instead would make the limit
	// asynchronous and passable by two concurrent requests, each reading the same
	// count and each issuing one more. The cost is that a document SEFAZ rejects
	// has spent a slot; the worker gives it back on a terminal rejection.
	if err := s.billingSvc.Reserve(ctx, orgPK, services.MeterNFe); err != nil {
		return nil, err
	}

	if err := s.nfeRepo.TransactReserveAndCreate(
		ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, nfeEncoded, outboxTx,
	); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número da NF-e. Tente novamente.")
		}
		return nil, err
	}

	// Best-effort — a failure here must never fail an already-committed
	// emission. Saved locations are a UX convenience for next time.
	if req.SaveEntregaLocation && req.Entrega != nil && req.ReceiverID != nil {
		if err := s.appendDeliveryLocation(ctx, orgPK, *req.ReceiverID, req.Entrega); err != nil {
			observability.Warn(ctx, "delivery location save failed", err)
		}
	}
	if req.SaveRetiradaLocation && req.Retirada != nil {
		if err := s.appendPickupLocation(ctx, orgPK, req.Retirada); err != nil {
			observability.Warn(ctx, "pickup location save failed", err)
		}
	}

	return nfeEncoded, nil
}

const maxSavedLocations = 5

// nfeLocalToMap converts an NfeLocalBody to a plain map using its JSON tags
// (snake_case, matching the API's on-the-wire shape), dropping unset optional
// fields instead of storing them as explicit nulls.
func nfeLocalToMap(l *NfeLocalBody) (map[string]any, error) {
	if l == nil {
		return nil, nil
	}
	b, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range m {
		if v == nil {
			delete(m, k)
		}
	}
	return m, nil
}

// locationDedupKey identifies a saved location by street+number+complement
// (normalized), so re-saving the same place doesn't grow the list forever.
func locationDedupKey(m map[string]any) string {
	lgr, _ := m["x_lgr"].(string)
	nro, _ := m["nro"].(string)
	cpl, _ := m["x_cpl"].(string)
	norm := func(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }
	return norm(lgr) + "|" + norm(nro) + "|" + norm(cpl)
}

// appendLocation adds loc to existing, replacing any prior entry with the
// same dedup key (so a re-save refreshes rather than duplicates) and capping
// the result at max by dropping the oldest entries.
func appendLocation(existing []any, loc map[string]any, max int) []any {
	key := locationDedupKey(loc)
	out := make([]any, 0, len(existing)+1)
	for _, e := range existing {
		em, ok := e.(map[string]any)
		if !ok || locationDedupKey(em) == key {
			continue
		}
		out = append(out, em)
	}
	out = append(out, loc)
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// appendDeliveryLocation best-effort persists loc onto the receiver person's
// delivery_locations for reuse in future NF-e emissions to that destinatário.
func (s *NfeService) appendDeliveryLocation(ctx context.Context, orgPK, receiverSK string, loc *NfeLocalBody) error {
	locMap, err := nfeLocalToMap(loc)
	if err != nil || locMap == nil {
		return err
	}
	current, err := s.personRepo.Get(ctx, orgPK, receiverSK)
	if err != nil || current == nil {
		return err
	}
	currentPlain, err := unmarshalToAny(current)
	if err != nil {
		return err
	}
	raw, _ := currentPlain["delivery_locations"].([]any)
	locs := appendLocation(raw, locMap, maxSavedLocations)
	_, err = s.personRepo.Update(ctx, orgPK, receiverSK, map[string]any{"delivery_locations": locs})
	return err
}

// appendPickupLocation best-effort persists loc onto the organization's
// pickup_locations for reuse in future NF-e emissions (org is always the
// remetente for local de retirada purposes).
func (s *NfeService) appendPickupLocation(ctx context.Context, orgPK string, loc *NfeLocalBody) error {
	locMap, err := nfeLocalToMap(loc)
	if err != nil || locMap == nil {
		return err
	}
	current, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil || current == nil {
		return err
	}
	currentPlain, err := unmarshalToAny(current)
	if err != nil {
		return err
	}
	raw, _ := currentPlain["pickup_locations"].([]any)
	locs := appendLocation(raw, locMap, maxSavedLocations)
	return s.orgRepo.UpdateOrganization(ctx, orgPK, map[string]any{"pickup_locations": locs})
}

// --- helpers ---

// generateAccessKey ports Python _generate_access_key.
// 44-digit key: cUF(2) + AAMM(4) + CNPJ(14) + mod(2) + serie(3) + nNF(9) + tpEmis(1) + cNF(8) + cDV(1).
// model is "55" (NF-e) or "65" (NFC-e).
func generateAccessKey(orgPK string, org map[string]types.AttributeValue, serie, number int, now time.Time, model, tpEmis string) (string, error) {
	uf := extractEmitUFFromItem(org)
	cUF, ok := services.UFCode[uf]
	if !ok {
		return "", problem.BadRequest("UF do emitente não configurada ou inválida: " + uf)
	}
	aamm := now.Format("0601") // Go "06"=2-digit year, "01"=2-digit month
	cnpj := services.StripPKPrefix(orgPK)
	if len(cnpj) > 14 {
		cnpj = cnpj[:14]
	}
	for len(cnpj) < 14 {
		cnpj += "0"
	}
	cNF := fmt.Sprintf("%08d", 10_000_000+rand.Intn(90_000_000))
	// tpEmis ocupa a posição 35 da chave — muda com a forma de emissão.
	key43 := fmt.Sprintf("%s%s%s%s%03d%09d%s%s", cUF, aamm, cnpj, model, serie, number, tpEmis, cNF)
	return key43 + calcDV(key43), nil
}

// calcDV computes mod-11 check digit for the 43-char NF-e key, matching Python _calc_dv.
func calcDV(key43 string) string {
	weights := []int{2, 3, 4, 5, 6, 7, 8, 9}
	total := 0
	wi := 0
	for i := len(key43) - 1; i >= 0; i-- {
		total += int(key43[i]-'0') * weights[wi%len(weights)]
		wi++
	}
	rem := total % 11
	if rem < 2 {
		return "0"
	}
	return strconv.Itoa(11 - rem)
}

// extractEmitUFFromItem reads the UF from org.person.addresses[0].state_federation.
func extractEmitUFFromItem(org map[string]types.AttributeValue) string {
	personAttr, ok := org["person"].(*types.AttributeValueMemberM)
	if !ok {
		return ""
	}
	if addrsAttr, ok := personAttr.Value["addresses"].(*types.AttributeValueMemberL); ok && len(addrsAttr.Value) > 0 {
		if addr, ok := addrsAttr.Value[0].(*types.AttributeValueMemberM); ok {
			if uf, ok := addr.Value["state_federation"].(*types.AttributeValueMemberS); ok {
				return uf.Value
			}
		}
	}
	if addrAttr, ok := personAttr.Value["address"].(*types.AttributeValueMemberM); ok {
		if uf, ok := addrAttr.Value["state_federation"].(*types.AttributeValueMemberS); ok {
			return uf.Value
		}
	}
	return ""
}

// resolveProducts resolves emission line items against the product catalog,
// validates each CFOP is configured on the product, and computes totals.
// Shared by NF-e and NFC-e emission.
// finNFeDevolucao é a finalidade "devolução de mercadoria" — o único caso em
// que o item pode declarar impostoDevol.
const finNFeDevolucao = "4"

// validateDevolucao recusa p_devol fora da nota de devolução: impostoDevol em
// nota normal é XML que só a SEFAZ rejeitaria.
func validateDevolucao(finNFe string, items []NfeProductItem) error {
	if finNFe == finNFeDevolucao {
		return nil
	}
	for _, it := range items {
		if it.PDevol != nil && *it.PDevol != "" {
			return problem.BadRequest("p_devol só é aceito em nota de devolução (fin_nfe = 4)")
		}
	}
	return nil
}

// applyServiceDefaults completa a tributação do item de serviço com o que já
// está no catálogo da NFS-e: item da lista de serviços, alíquota do ISS e
// código do serviço. O que a tributação do produto já define vence — o catálogo
// é default, não override.
func applyServiceDefaults(
	ctx context.Context, serviceRepo *repositories.ServiceRepository, orgPK string,
	items []NfeProductItem, productItems []map[string]any,
) error {
	for i, item := range items {
		if item.ServiceID == nil || *item.ServiceID == "" || serviceRepo == nil {
			continue
		}
		svcAttr, err := serviceRepo.Get(ctx, orgPK, *item.ServiceID)
		if err != nil {
			return err
		}
		if svcAttr == nil {
			return problem.NotFound("serviço não encontrado: " + *item.ServiceID)
		}
		var svc map[string]any
		if err := attributevalue.UnmarshalMap(svcAttr, &svc); err != nil {
			return problem.InternalServer("failed to decode service")
		}
		cfgs, _ := productItems[i]["cfop_config"].([]any)
		if len(cfgs) == 0 {
			continue
		}
		cfg, _ := cfgs[0].(map[string]any)
		if cfg == nil {
			continue
		}
		setDefault(cfg, "issqn_c_list_serv", anyStr(svc, "trib_nacional_code", ""))
		setDefault(cfg, "issqn_c_servico", anyStr(svc, "code", ""))
		if iss, ok := svc["iss"].(map[string]any); ok {
			setDefault(cfg, "issqn_aliq", anyStr(iss, "tax_rate", ""))
		}
		// Sem indISS declarado, o item de serviço é exigível: é o caso normal,
		// e sem ele o grupo ISSQN inteiro não sairia.
		setDefault(cfg, "issqn_ind_iss", issqnIndISSExigivel)
	}
	return nil
}

// issqnIndISSExigivel é indISS=1 (exigível), o default do item de serviço.
const issqnIndISSExigivel = "1"

// setDefault só preenche o que ainda não foi decidido pela tributação.
func setDefault(cfg map[string]any, key, value string) {
	if value == "" {
		return
	}
	if cur, ok := cfg[key].(string); ok && cur != "" {
		return
	}
	cfg[key] = value
}

func resolveProducts(
	ctx context.Context, productRepo *repositories.ProductRepository,
	taxProfileRepo *repositories.TaxProfileRepository, orgPK, destUF string, items []NfeProductItem,
) ([]map[string]any, decimal.Decimal, decimal.Decimal, error) {
	var productItems []map[string]any
	totalProducts := decimal.Zero
	totalDiscount := decimal.Zero

	// Uma passada só pelos produtos, e um BatchGetItem só pelos perfis que eles
	// referenciam — nunca um Get por item dentro do laço.
	products := make([]map[string]any, len(items))
	var profileIDs []string
	for i, item := range items {
		productAttr, err := productRepo.Get(ctx, orgPK, item.ProductID)
		if err != nil {
			return nil, decimal.Zero, decimal.Zero, err
		}
		if productAttr == nil {
			return nil, decimal.Zero, decimal.Zero, problem.NotFound("produto não encontrado: " + item.ProductID)
		}
		var product map[string]any
		if err := attributevalue.UnmarshalMap(productAttr, &product); err != nil {
			return nil, decimal.Zero, decimal.Zero, problem.InternalServer("failed to decode product")
		}
		products[i] = product
		profileIDs = append(profileIDs, profileRefs(product)...)
	}

	profiles, err := loadTaxProfiles(ctx, taxProfileRepo, orgPK, profileIDs)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, err
	}

	for idx, item := range items {
		product := products[idx]

		// A tributação efetiva do item: cfop_config vence overrides, que vencem
		// o perfil. Um CFOP sem tributação em lugar nenhum é erro de cadastro.
		resolvedTax, err := resolveCfopTax(product, profiles, item.CFOP, destUF)
		if err != nil {
			code, _ := product["code"].(string)
			return nil, decimal.Zero, decimal.Zero, problem.BadRequest(
				cfopNotConfiguredError(item.CFOP, code, productCFOPs(product, profiles)),
			)
		}

		qty := d(item.Quantity)
		unitValStr := "0"
		if item.UnitValue != nil {
			unitValStr = *item.UnitValue
		} else if v, ok := product["value"].(string); ok && v != "" {
			unitValStr = v
		}
		unitVal := dn(unitValStr, 10)
		disc := d(item.Discount)
		itemTotal := qty.Mul(unitVal).RoundBank(2).Sub(disc)
		totalProducts = totalProducts.Add(qty.Mul(unitVal))
		totalDiscount = totalDiscount.Add(disc)

		unit := "UN"
		if u, ok := product["unit"].(string); ok && u != "" {
			unit = u
		}

		pi := map[string]any{
			"product_id":   item.ProductID,
			"product_code": product["code"],
			"description":  product["description"],
			"ncm":          product["ncm"],
			"cest":         product["cest"],
			"cfop":         item.CFOP,
			"unit":         unit,
			"taxable_unit": product["taxable_unit"],
			"cean":         product["cean"],
			"origin":       product["origin"],
			// Uma única entrada, já resolvida para o CFOP deste item — os
			// construtores do XML continuam lendo cfop_config exatamente como
			// antes (findCFOPEntry), sem saber que perfis existem.
			"cfop_config":          []any{resolvedTax},
			"conversion_factors":   product["conversion_factors"],
			"net_weight":           product["net_weight"],
			"gross_weight":         product["gross_weight"],
			"c_benef":              product["c_benef"],
			"ext_ipi":              product["ext_ipi"],
			"ind_escala":           product["ind_escala"],
			"cnpj_fab":             product["cnpj_fab"],
			"ind_tot":              product["ind_tot"],
			"icms_aliq_override":   product["icms_aliq_override"],
			"fcp_aliq_override":    product["fcp_aliq_override"],
			"inf_ad_prod":          product["inf_ad_prod"],
			"comb_c_prod_anp":      product["comb_c_prod_anp"],
			"comb_desc_anp":        product["comb_desc_anp"],
			"comb_uf_cons":         product["comb_uf_cons"],
			"comb_codif":           product["comb_codif"],
			"comb_p_glp":           product["comb_p_glp"],
			"comb_p_gnn":           product["comb_p_gnn"],
			"comb_p_gni":           product["comb_p_gni"],
			"comb_v_part":          product["comb_v_part"],
			"comb_p_bio":           product["comb_p_bio"],
			"med_c_prod_anvisa":    product["med_c_prod_anvisa"],
			"med_x_motivo_isencao": product["med_x_motivo_isencao"],
			"med_v_pmc":            product["med_v_pmc"],
			"veic_tp_op":           product["veic_tp_op"],
			"veic_tp_comb":         product["veic_tp_comb"],
			"veic_tp_pint":         product["veic_tp_pint"],
			"veic_tp_veic":         product["veic_tp_veic"],
			"veic_esp_veic":        product["veic_esp_veic"],
			"veic_vin":             product["veic_vin"],
			"veic_cond_veic":       product["veic_cond_veic"],
			"veic_c_mod":           product["veic_c_mod"],
			"veic_c_cor_denatran":  product["veic_c_cor_denatran"],
			"veic_lota":            product["veic_lota"],
			"veic_tp_rest":         product["veic_tp_rest"],
			"veic_ano_mod":         product["veic_ano_mod"],
			"veic_ano_fab":         product["veic_ano_fab"],
			"veic_pot":             product["veic_pot"],
			"veic_cilin":           product["veic_cilin"],
			"veic_cmt":             product["veic_cmt"],
			"veic_dist":            product["veic_dist"],
			"veic_c_cor":           product["veic_c_cor"],
			"veic_x_cor":           product["veic_x_cor"],
			"veic_chassi":          ptrStr(item.VeicChassi),
			"veic_n_serie":         ptrStr(item.VeicNSerie),
			"veic_n_motor":         ptrStr(item.VeicNMotor),
			"veic_c_cor_override":  ptrStr(item.VeicCCor),
			"veic_x_cor_override":  ptrStr(item.VeicXCor),
			"arma_tp_arma":         product["arma_tp_arma"],
			"arma_descr":           product["arma_descr"],
			"armas":                item.Armas,
			"quantity":             item.Quantity,
			"unit_value":           unitVal.String(),
			"discount":             item.Discount,
			"v_frete":              ptrStr(item.VFrete),
			"v_seg":                ptrStr(item.VSeg),
			"v_outro":              ptrStr(item.VOutro),
			"total":                q2(itemTotal.RoundBank(2)),
			"p_devol":              ptrStr(item.PDevol),
			// Selo e enquadramento do IPI vêm do cadastro do produto.
			"ipi_cnpj_prod": product["ipi_cnpj_prod"],
			"ipi_c_selo":    product["ipi_c_selo"],
			"ipi_q_selo":    product["ipi_q_selo"],
			"ipi_c_enq":     product["ipi_c_enq"],
			// Observação fiscal padrão do produto (det/obsItem).
			"obs_item_x_campo": product["obs_item_x_campo"],
			"obs_item_x_texto": product["obs_item_x_texto"],
		}
		productItems = append(productItems, pi)
	}

	return productItems, totalProducts.RoundBank(2), totalDiscount.RoundBank(2), nil
}

func (s *NfeService) resolveTransport(ctx context.Context, orgPK string, t *NfeTransportItem) (map[string]any, error) {
	if t == nil {
		return nil, nil
	}
	td := map[string]any{
		"mod_frete":        t.ModFrete,
		"transporta_pk":    ptrStr(t.TransportaPK),
		"transporta_cnpj":  ptrStr(t.TransportaCNPJ),
		"transporta_cpf":   ptrStr(t.TransportaCPF),
		"transporta_nome":  ptrStr(t.TransportaNome),
		"transporta_ie":    ptrStr(t.TransportaIE),
		"transporta_ender": ptrStr(t.TransportaEnder),
		"transporta_mun":   ptrStr(t.TransportaMun),
		"transporta_uf":    ptrStr(t.TransportaUF),
		"veiculo_sk":       ptrStr(t.VeiculoSK),
		"veiculo_placa":    ptrStr(t.VeiculoPlaca),
		"veiculo_uf":       ptrStr(t.VeiculoUF),
		"veiculo_rntrc":    ptrStr(t.VeiculoRNTRC),
	}

	if t.TransportaPK != nil {
		carrier, err := s.personRepo.Get(ctx, orgPK, *t.TransportaPK)
		if err != nil {
			return nil, err
		}
		if carrier != nil {
			isPJ := strings.HasPrefix(*t.TransportaPK, "CNPJ_")
			doc := services.StripPKPrefix(*t.TransportaPK)
			if isPJ {
				td["transporta_cnpj"] = doc
				td["transporta_cpf"] = ""
			} else {
				td["transporta_cnpj"] = ""
				td["transporta_cpf"] = doc
			}
			td["transporta_nome"] = strAttr(carrier, "name")

			var carrierMap map[string]any
			if err := attributevalue.UnmarshalMap(carrier, &carrierMap); err == nil {
				if person, ok := carrierMap["person"].(map[string]any); ok {
					addr := services.FirstAddress(person)
					td["transporta_uf"] = addr["state_federation"]
					if regs, ok := person["state_registrations"].([]any); ok && len(regs) > 0 {
						if reg, ok := regs[0].(map[string]any); ok {
							td["transporta_ie"] = reg["state_registration"]
						}
					}
					// Perfil de ICMS retido pelo remetente (transp/retTransp):
					// é invariante da transportadora, não da nota.
					if ret, ok := person["freight_retention"].(map[string]any); ok {
						td["freight_retention"] = ret
					}
				}
			}
		}
	}

	if t.VeiculoSK != nil && s.vehicleRepo != nil {
		vehicle, err := s.vehicleRepo.Get(ctx, orgPK, *t.VeiculoSK)
		if err != nil {
			return nil, err
		}
		if vehicle != nil {
			td["veiculo_placa"] = strAttr(vehicle, "plate")
			td["veiculo_uf"] = strAttr(vehicle, "plate_uf")
			var vehicleMap map[string]any
			if err := attributevalue.UnmarshalMap(vehicle, &vehicleMap); err == nil {
				if owner, ok := vehicleMap["owner"].(map[string]any); ok {
					td["veiculo_rntrc"] = owner["rntrc"]
				}
			}
		}
	}

	return td, nil
}

func unmarshalToAny(item map[string]types.AttributeValue) (map[string]any, error) {
	var result map[string]any
	err := attributevalue.UnmarshalMap(item, &result)
	return result, err
}

func fmtDhEmi(t time.Time) string {
	return t.Format("2006-01-02T15:04:05-07:00")
}

// ptrStr dereferences a *string, returning "" for nil.
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// transportVols / transportReboques leem as listas do transporte sem forçar o
// chamador a checar nil — transport é opcional na emissão.
func transportVols(t *NfeTransportItem) []NfeVolBody {
	if t == nil {
		return nil
	}
	return t.Vols
}

func transportReboques(t *NfeTransportItem) []NfeReboqueBody {
	if t == nil {
		return nil
	}
	return t.Reboques
}

// terminalSK normaliza o id do terminal para o SK do cadastro, aceitando tanto
// o uuid quanto o SK completo vindo da UI.
func terminalSK(id string) string {
	if strings.HasPrefix(id, repositories.SKPrefixPaymentTerminal) {
		return id
	}
	return repositories.SKPrefixPaymentTerminal + id
}

// resolvePaymentTerminals lê num BatchGet só todos os terminais citados pelos
// pagamentos. Um id inexistente é erro de request, não silêncio. Função livre e
// não método: NF-e e NFC-e usam a mesma resolução.
func resolvePaymentTerminals(ctx context.Context, repo *repositories.PaymentTerminalRepository, orgPK string, payments []NfePaymentItem) (map[string]map[string]any, error) {
	var ids []string
	for _, p := range payments {
		if p.TerminalID != nil && *p.TerminalID != "" {
			ids = append(ids, strings.TrimPrefix(*p.TerminalID, repositories.SKPrefixPaymentTerminal))
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	raw, err := repo.BatchGet(ctx, orgPK, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]map[string]any, len(raw))
	for id, item := range raw {
		m, err := unmarshalToAny(item)
		if err != nil {
			return nil, problem.InternalServer("failed to decode payment terminal")
		}
		out[repo.SK(id)] = m
	}
	for _, id := range ids {
		if _, ok := out[repo.SK(id)]; !ok {
			return nil, problem.NotFound("terminal de pagamento não encontrado: " + id)
		}
	}
	return out, nil
}
