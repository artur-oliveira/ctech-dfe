package ctes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
	"gopkg.aoctech.app/dfe/go-dfe/uftz"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// Códigos dos 422 da emissão (problem.code). A UI ramifica por eles.
const (
	CodeRNTRCRequired         = "rntrc_required"
	CodeCFOPRequired          = "cfop_required"
	CodeICMSRequired          = "icms_required"
	CodeICMSRateRequired      = "icms_rate_required"
	CodeICMSSNNotAllowed      = "icms_sn_not_allowed"
	CodeTakerPersonRequired   = "taker_person_required"
	CodePersonNotFound        = "person_not_found"
	CodeWeightRequired        = "weight_required"
	CodePredominantRequired   = "predominant_product_required"
	CodeComponentsExceedTotal = "components_exceed_total"
	CodeFreightTotalInvalid   = "freight_total_invalid"
	CodeCityUnknown           = "city_unknown"
)

// CST aceitos (icms.cst). SN é o ICMSSN do Simples Nacional.
const (
	cst00 = "00"
	cst20 = "20"
	cst40 = "40"
	cst41 = "41"
	cst51 = "51"
	cst60 = "60"
	cst90 = "90"
	cstSN = "SN"
	// indSN: única opção do leiaute (1 = Simples Nacional).
	indSNSim = "1"
)

// crtSimples são os CRT do Simples Nacional (1 SN, 2 excesso de sublimite, 4 MEI): sem ICMS destacado o
// padrão é o ICMSSN. Para o regime normal (3) nenhuma alíquota é presumida.
var crtSimples = map[int]bool{1: true, 2: true, 4: true}

// CteDeriveBody é o corpo de POST /ctes/derive.
type CteDeriveBody struct {
	NFeKeys []string `json:"nfe_keys" validate:"required,min=1,max=50,unique,dive,dfe_access_key"`
}

// CteEmitBody é o corpo de POST /ctes.
type CteEmitBody struct {
	NFeKeys []string   `json:"nfe_keys" validate:"required,min=1,max=50,unique,dive,dfe_access_key"`
	Taker   CteTaker   `json:"taker" validate:"required"`
	Freight CteFreight `json:"freight" validate:"required"`
	ICMS    *CteICMS   `json:"icms"`
	// IBSCBS opcional: nada é presumido (CST, classificação, base e alíquotas do usuário).
	IBSCBS *CteIBSCBS `json:"ibs_cbs"`
	// AuthorizedXML: autorizados a baixar o XML (autXML, até 10).
	AuthorizedXML []CteAuthorizedXML `json:"authorized_xml" validate:"omitempty,max=10,dive"`
	Overrides     *CteOverrides      `json:"overrides"`
	// CFOP da prestação (5351 a 5360, 6351 a 6360, 7358, 5932, 6932...). Regra fiscal: nunca presumido.
	CFOP           *string `json:"cfop" validate:"omitempty,cfop_cte"`
	AdditionalInfo *string `json:"additional_info" validate:"omitempty,max=2000"`
}

type CteTaker struct {
	Role     string  `json:"role" validate:"required,oneof=remetente destinatario expedidor recebedor outro"`
	PersonID *string `json:"person_id"`
}

type CteFreight struct {
	Total      string         `json:"total" validate:"required,money2"`
	Components []CteComponent `json:"components" validate:"omitempty,max=30,dive"`
}

type CteComponent struct {
	// xNome do Comp tem 15 caracteres no leiaute.
	Name  string `json:"name" validate:"required,max=15"`
	Value string `json:"value" validate:"required,money2"`
}

type CteICMS struct {
	CST    string  `json:"cst" validate:"required,oneof=00 20 40 41 51 60 90 SN"`
	PICMS  *string `json:"p_icms" validate:"omitempty,percent"`
	PRedBC *string `json:"p_red_bc" validate:"omitempty,percent"`
}

type CteOverrides struct {
	SenderPersonID    *string           `json:"sender_person_id"`
	RecipientPersonID *string           `json:"recipient_person_id"`
	Cargo             *CteCargoOverride `json:"cargo"`
	Origin            *CteCityOverride  `json:"origin"`
	Destination       *CteCityOverride  `json:"destination"`
}

type CteCargoOverride struct {
	TotalValue         *string `json:"total_value" validate:"omitempty,money2"`
	TotalWeightKg      *string `json:"total_weight_kg" validate:"omitempty,decimalv"`
	Volumes            *int64  `json:"volumes" validate:"omitempty,gte=0"`
	PredominantProduct *string `json:"predominant_product" validate:"omitempty,max=60"`
}

type CteCityOverride struct {
	CityIBGE string `json:"city_ibge" validate:"required,ibge"`
}

func unprocessable(code, detail string) *problem.Problem {
	return problem.UnprocessableEntity(detail).WithCode(code)
}

// Derive lê as notas e devolve o que o CT-e herda delas.
func (s *CteService) Derive(ctx context.Context, orgPK string, keys []string) (*DeriveResult, error) {
	_, env, err := s.config(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	d, err := s.derive(ctx, orgPK, env, keys)
	if err != nil {
		return nil, err
	}
	return &d.DeriveResult, nil
}

// Emit valida, monta e grava o CT-e (pending) com o outbox na mesma transação da numeração.
func (s *CteService) Emit(ctx context.Context, orgPK string, req CteEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	cfg, environment, err := s.config(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	d, err := s.derive(ctx, orgPK, environment, req.NFeKeys)
	if err != nil {
		return nil, err
	}
	if len(d.notFound) > 0 {
		return nil, unprocessable(WarnNoteNotFound, "notas indisponíveis para o CT-e: "+strings.Join(d.notFound, ", "))
	}
	if len(d.notAuthorized) > 0 {
		return nil, unprocessable(WarnNoteNotAuthorized, "notas não autorizadas: "+strings.Join(d.notAuthorized, ", "))
	}
	if req.CFOP == nil || *req.CFOP == "" {
		return nil, unprocessable(CodeCFOPRequired, "informe o CFOP da prestação do serviço de transporte")
	}

	emit, emitDoc, crt, err := s.issuer(ctx, orgPK)
	if err != nil {
		return nil, err
	}

	in, err := s.resolveInputs(ctx, orgPK, req, d)
	if err != nil {
		return nil, err
	}
	vTPrest, comps, err := resolveComponents(req.Freight)
	if err != nil {
		return nil, err
	}
	icms, err := resolveICMS(req.ICMS, crt, vTPrest)
	if err != nil {
		return nil, err
	}

	ibscbs, err := resolveIBSCBS(req.IBSCBS, crt)
	if err != nil {
		return nil, err
	}
	actorDocs := []string{emit.doc(), in.rem.doc(), in.dest.doc()}
	for _, a := range []*actor{in.exped, in.receb, in.toma4} {
		if a != nil {
			actorDocs = append(actorDocs, a.doc())
		}
	}
	autXML, err := resolveAuthorizedXML(req.AuthorizedXML, actorDocs)
	if err != nil {
		return nil, err
	}

	mdfeCfg, err := s.mdfeConfigRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	rntrc := strAttr(mdfeCfg, rntrcField)
	if rntrc == "" {
		return nil, unprocessable(CodeRNTRCRequired, "cadastre o RNTRC da transportadora em Configuração Fiscal, aba MDF-e, antes de emitir CT-e")
	}

	cst := cstSN
	if req.ICMS != nil {
		cst = req.ICMS.CST
	}
	p := buildParams{
		cfop: *req.CFOP, emit: emit, crt: crt,
		rem: in.rem, dest: in.dest, exped: in.exped, receb: in.receb, takerRole: req.Taker.Role, toma4: in.toma4,
		origin: in.origin, destination: in.destination,
		vTPrest: vTPrest, comps: comps, icms: icms, ibscbs: ibscbs, autXML: autXML,
		vCarga: in.vCarga, proPred: in.proPred, weight: in.weight, volumes: in.volumes, nfeKeys: req.NFeKeys,
		rntrc: rntrc, addInfo: valueOr(req.AdditionalInfo),
	}
	return s.persist(ctx, orgPK, cfg, environment, emitDoc, p, cst, userID, userName)
}

// persist numera, monta, valida (portão) e grava o CT-e pending com o outbox na mesma transação da numeração.
// É o fim comum da emissão normal, do complemento e do substituto: p chega com os dados do documento e recebe aqui
// chave, série, número, hora e forma de emissão.
func (s *CteService) persist(ctx context.Context, orgPK string, cfg map[string]types.AttributeValue, environment int, emitDoc string,
	p buildParams, cst, userID, userName string) (map[string]types.AttributeValue, error) {
	envPrefix := services.EnvToPrefix(environment)
	emit := p.emit
	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, dfeproblem.NoCertificate("certificado digital não encontrado")
	}
	cert := certs[0]

	serie := intAttr(cfg, envPrefix+"_current_serie", 1)
	currentNumber := intAttr(cfg, envPrefix+"_current_number", 0)
	now := uftz.Now(emit.Addr.UF) // dhEmi e AAMM da chave são horário local da UF do emitente
	// tpEmis está na chave: a forma de emissão (normal, off-line 2, SVC 7/8) vem antes dela.
	cfg = services.PrecheckAutoContingency(ctx, s.configRepo, services.ModelCTe, orgPK, emit.Addr.UF, cfg, now)
	em, err := resolveCTeEmission(cfg, emit.Addr.UF)
	if err != nil {
		return nil, err
	}
	if err := checkEmissionForType(p.tpCTe, em); err != nil {
		return nil, err
	}
	model, sefazService := services.ModelCTe, sefazServiceAutorizacao
	if p.os != nil {
		model, sefazService = ModelCTeOS, sefazServiceAutorizacaoOS
	}
	accessKey := services.GenerateAccessKey(emit.Addr.UF, emitDoc, model, serie, currentNumber, now, em.tpEmis)
	if accessKey == "" {
		return nil, problem.BadRequest("UF do emitente inválida: " + emit.Addr.UF)
	}
	qrSign := ""
	if em.offline {
		if qrSign, err = services.SignTextViaEgress(ctx, s.egress, orgPK, emitDoc, s3PrefixCte, accessKey); err != nil {
			return nil, err
		}
	}

	p.accessKey, p.serie, p.number, p.environment, p.now = accessKey, serie, currentNumber, environment, now
	p.tpEmis, p.contingencyAt, p.justification, p.qrSign = em.tpEmis, em.since, em.reason, qrSign
	p.tech, p.csrtID, p.csrt = s.tech, strAttr(cfg, "csrt_id"), strAttr(cfg, "csrt")
	body := BuildCTe(p)
	if p.os != nil {
		dfecheck.ShadowCTeOS(accessKey, body)
	} else {
		dfecheck.ShadowCTe(accessKey, body)
	}
	if err := gateCTe(body); err != nil {
		return nil, err
	}

	pk := envPrefix + "#" + orgPK
	rec := buildRecord(pk, p, cst, userID, userName)
	if p.os != nil {
		rec = osRecord(pk, p, cst, userID, userName)
	}
	switch p.tpCTe {
	case tpCTeComplement:
		complementRecord(rec, p)
	case tpCTeSubstitute:
		substituteRecord(rec, p)
	}
	encoded, err := repositories.EncodeItem(rec)
	if err != nil {
		return nil, problem.InternalServer("failed to encode CT-e record")
	}
	reservation, err := s.billingSvc.PrepareUsageReservation(ctx, orgPK, services.MeterCTe, envPrefix == services.EnvProd)
	if err != nil {
		return nil, err
	}
	workerMsg := services.WorkerMessage{
		DocPK: pk, AccessKey: accessKey, TableName: tableCtes, S3Prefix: s3PrefixCte, ExpectedFileName: accessKey,
		CNPJ: emitDoc, UF: emit.Addr.UF, SefazEnvironment: sefazEnvFor(environment),
		CertS3Key: strAttr(cert, "s3_key"), CertPassword: strAttr(cert, "password"),
		DocType: s3PrefixCte, SefazService: sefazService, Contingency: em.svc, Body: body,
		BillingOrganizationID: reservation.OrganizationID, BillingPeriod: reservation.Period,
		BillingSubscriptionID: reservation.SubscriptionID, BillingPriceID: reservation.PriceID,
		BillingMeter: reservation.Meter, BillingExempt: reservation.Exempt,
	}
	var extraTx []types.TransactWriteItem
	if em.offline {
		// Offline: sign now, keep the signed XML pending (contingency index), transmit later.
		// No outbox row: the retransmission worker sends the stored message.
		signed, err := services.SignOnlyViaEgress(ctx, s.egress, orgPK, emitDoc, emit.Addr.UF, sefazEnvFor(environment), s3PrefixCte, body)
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
	if reservation.Tx != nil {
		extraTx = append(extraTx, *reservation.Tx)
	}
	switch p.tpCTe {
	case tpCTeComplement, tpCTeSubstitute:
		// Referência reversa (related.go): um Put por original, na mesma transação da emissão.
		cteType, originals := cteTypeComplement, p.complementKeys
		if p.tpCTe == tpCTeSubstitute {
			cteType, originals = cteTypeSubstitute, []string{p.sub.chCte}
		}
		refs, err := refPutItems(s.cteRepo.TableName, pk, accessKey, cteType, originals, repositories.NowStr())
		if err != nil {
			return nil, problem.InternalServer("failed to encode CT-e references")
		}
		extraTx = append(extraTx, refs...)
	}
	if err := s.cteRepo.TransactReserveAndCreate(ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded, extraTx...); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número do CT-e. Tente novamente.")
		}
		return nil, err
	}
	return encoded, nil
}

// issuer resolve o emitente (a organização), o documento dele e o CRT.
func (s *CteService) issuer(ctx context.Context, orgPK string) (emit actor, emitDoc string, crt int, err error) {
	orgItem, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return actor{}, "", 0, err
	}
	if orgItem == nil {
		return actor{}, "", 0, problem.NotFound("organização não encontrada")
	}
	emitDoc, isPJ := services.IssuerDocAV(orgItem, orgPK)
	if emitDoc == "" {
		return actor{}, "", 0, problem.BadRequest("documento do emitente não encontrado")
	}
	orgPerson := personMap(orgItem)
	emit = personActor(emitDoc, strAttr(orgItem, "name"), orgPerson)
	emit.CNPJ, emit.CPF = "", ""
	if isPJ {
		emit.CNPJ = emitDoc
	} else {
		emit.CPF = emitDoc
	}
	if emit.Addr.UF == "" {
		return actor{}, "", 0, problem.BadRequest("UF do emitente não configurada")
	}
	return emit, emitDoc, anyInt(orgPerson, "crt", 1), nil
}

// emitInputs são remetente, destinatário, expedidor, recebedor, tomador, percurso e carga já resolvidos.
type emitInputs struct {
	rem, dest           actor
	exped, receb, toma4 *actor
	origin, destination Place
	vCarga, weight      decimal.Decimal
	volumes             int64
	proPred             string
}

func (s *CteService) resolveInputs(ctx context.Context, orgPK string, req CteEmitBody, d *derivation) (*emitInputs, error) {
	ov := req.Overrides
	if ov == nil {
		ov = &CteOverrides{}
	}
	in := &emitInputs{}
	var err error
	if in.rem, err = s.partyActor(ctx, orgPK, ov.SenderPersonID, d.Sender, d.parsed, func(n *nfexml.NFe) nfexml.Party { return n.Emit }, WarnMixedSender, "notas com remetentes diferentes: informe o remetente ou emita um CT-e por remetente"); err != nil {
		return nil, err
	}
	if in.dest, err = s.partyActor(ctx, orgPK, ov.RecipientPersonID, d.Recipient, d.parsed, func(n *nfexml.NFe) nfexml.Party { return n.Dest }, WarnMixedRecipient, "notas com destinatários diferentes: informe o destinatário ou emita um CT-e por destinatário"); err != nil {
		return nil, err
	}

	switch req.Taker.Role {
	case RoleExpedidor, RoleRecebedor, RoleOutro:
		if req.Taker.PersonID == nil || *req.Taker.PersonID == "" {
			return nil, unprocessable(CodeTakerPersonRequired, "escolha no cadastro a pessoa que paga o frete")
		}
		a, err := s.personActorByID(ctx, orgPK, *req.Taker.PersonID)
		if err != nil {
			return nil, err
		}
		switch req.Taker.Role {
		case RoleExpedidor:
			in.exped = &a
		case RoleRecebedor:
			in.receb = &a
		default:
			in.toma4 = &a
		}
	}

	// Origem e destino: município do remetente e do destinatário, sobrescrevíveis por um município conhecido.
	known := map[string]Place{}
	addPlace := func(a nfexml.Address) {
		if a.CityIBGE != "" {
			known[a.CityIBGE] = Place{UF: a.UF, CityIBGE: a.CityIBGE, City: a.City}
		}
	}
	for _, n := range d.parsed {
		addPlace(n.Emit.Addr)
		addPlace(n.Dest.Addr)
	}
	for _, a := range []*actor{&in.rem, &in.dest, in.exped, in.receb, in.toma4} {
		if a != nil {
			addPlace(a.Addr)
		}
	}
	in.origin = known[in.rem.Addr.CityIBGE]
	in.destination = known[in.dest.Addr.CityIBGE]
	for _, o := range []struct {
		ov  *CteCityOverride
		dst *Place
	}{{ov.Origin, &in.origin}, {ov.Destination, &in.destination}} {
		if o.ov == nil {
			continue
		}
		pl, ok := known[o.ov.CityIBGE]
		if !ok {
			return nil, unprocessable(CodeCityUnknown, "município "+o.ov.CityIBGE+" não aparece nas notas nem nos participantes; escolha um deles")
		}
		*o.dst = pl
	}

	in.vCarga = nfexml.Dec(d.Cargo.TotalValue)
	in.volumes = d.Cargo.Volumes
	in.proPred = d.Cargo.PredominantProduct
	if d.weight != nil {
		in.weight = *d.weight
	}
	if c := ov.Cargo; c != nil {
		if c.TotalValue != nil {
			in.vCarga = nfexml.Dec(*c.TotalValue)
		}
		if c.TotalWeightKg != nil {
			in.weight = nfexml.Dec(*c.TotalWeightKg)
		}
		if c.Volumes != nil {
			in.volumes = *c.Volumes
		}
		if c.PredominantProduct != nil {
			in.proPred = strings.TrimSpace(*c.PredominantProduct)
		}
	}
	if !in.weight.IsPositive() {
		return nil, unprocessable(CodeWeightRequired, "informe o peso da carga: as notas não trazem peso bruto")
	}
	if in.proPred == "" {
		return nil, unprocessable(CodePredominantRequired, "informe o produto predominante da carga")
	}
	return in, nil
}

// partyActor resolve remetente ou destinatário: pessoa do cadastro quando sobrescrito, senão a parte das notas.
func (s *CteService) partyActor(ctx context.Context, orgPK string, override *string, derived *Party, notes []*nfexml.NFe,
	pick func(*nfexml.NFe) nfexml.Party, mixedCode, mixedMsg string) (actor, error) {
	if override != nil && *override != "" {
		return s.personActorByID(ctx, orgPK, *override)
	}
	if derived == nil {
		return actor{}, unprocessable(mixedCode, mixedMsg)
	}
	p := pick(notes[0])
	return actor{CNPJ: p.CNPJ, CPF: p.CPF, IE: p.IE, Name: p.Name, Phone: p.Addr.Phone, Email: p.Email, Addr: p.Addr}, nil
}

func (s *CteService) personActorByID(ctx context.Context, orgPK, id string) (actor, error) {
	sk, item, err := s.person(ctx, orgPK, id)
	if err != nil {
		return actor{}, err
	}
	if item == nil {
		return actor{}, unprocessable(CodePersonNotFound, "pessoa não encontrada no cadastro: "+id)
	}
	person, _ := item["person"].(map[string]any)
	name, _ := item["name"].(string)
	return personActor(services.StripPKPrefix(sk), name, person), nil
}

// personActor monta o ator de uma pessoa do cadastro (ou da organização), no formato person.addresses.
func personActor(doc, name string, person map[string]any) actor {
	addr := services.FirstAddress(person)
	a := actor{
		IE: services.FirstIE(person), Name: name, Fantasy: anyStr(person, "fantasy_name"),
		Phone: services.FirstPhone(person), Email: services.FirstEmail(person),
		Addr: nfexml.Address{
			Street: anyStr(addr, "street"), Number: anyStr(addr, "number"), Complement: anyStr(addr, "complement"),
			District: anyStr(addr, "neighborhood"), CityIBGE: anyStr(addr, "city_ibge_code"), City: anyStr(addr, "city"),
			UF: anyStr(addr, "state_federation"), CEP: anyStr(addr, "postal_code"),
		},
	}
	if len(doc) == cnpjLen {
		a.CNPJ = doc
	} else {
		a.CPF = doc
	}
	return a
}

// resolveComponents: sem componentes, um Comp FRETE VALOR com o total; com componentes, FRETE VALOR leva
// o total menos a soma deles e os demais seguem (vTPrest = soma dos Comp = total).
func resolveComponents(f CteFreight) (decimal.Decimal, []map[string]any, error) {
	total := nfexml.Dec(f.Total)
	if !total.IsPositive() {
		return decimal.Zero, nil, unprocessable(CodeFreightTotalInvalid, "o valor do frete deve ser maior que zero")
	}
	sum := decimal.Zero
	rest := make([]map[string]any, 0, len(f.Components))
	for _, c := range f.Components {
		v := nfexml.Dec(c.Value)
		sum = sum.Add(v)
		rest = append(rest, map[string]any{"xNome": strings.TrimSpace(c.Name), "vComp": q2(v)})
	}
	if sum.GreaterThan(total) {
		return decimal.Zero, nil, unprocessable(CodeComponentsExceedTotal, "a soma dos componentes passa do valor total do frete")
	}
	comps := append([]map[string]any{{"xNome": compFreteValor, "vComp": q2(total.Sub(sum))}}, rest...)
	return total, comps, nil
}

// resolveICMS monta imp/ICMS. vBC = vTPrest com redução opcional; vICMS = vBC x pICMS; 2 casas, arredondamento
// bancário. Nenhuma alíquota é presumida.
func resolveICMS(in *CteICMS, crt int, vTPrest decimal.Decimal) (map[string]any, error) {
	if in == nil {
		if !crtSimples[crt] {
			return nil, unprocessable(CodeICMSRequired, "informe o CST e a alíquota do ICMS da prestação")
		}
		in = &CteICMS{CST: cstSN}
	}
	hundred := decimal.NewFromInt(100)
	pRed := decimal.Zero
	if in.PRedBC != nil {
		pRed = nfexml.Dec(*in.PRedBC)
	}
	// pRedBC só existe em ICMS20 e ICMS90 no XSD; em outro CST a base reduzida ficaria sem a redução declarada.
	if pRed.IsPositive() && in.CST != cst20 && in.CST != cst90 {
		return nil, unprocessable(CodeICMSRateRequired, "o percentual de redução da base só vale para os CST 20 e 90")
	}
	vBC := vTPrest.Mul(decimal.NewFromInt(1).Sub(pRed.Div(hundred))).RoundBank(2)
	rate := func() (decimal.Decimal, error) {
		if in.PICMS == nil || !nfexml.Dec(*in.PICMS).IsPositive() {
			return decimal.Zero, unprocessable(CodeICMSRateRequired, "informe a alíquota do ICMS para o CST "+in.CST)
		}
		return nfexml.Dec(*in.PICMS), nil
	}

	switch in.CST {
	case cstSN:
		if !crtSimples[crt] {
			return nil, unprocessable(CodeICMSSNNotAllowed, "ICMS do Simples Nacional só vale para emitente optante do Simples")
		}
		return map[string]any{"ICMSSN": map[string]any{"CST": cst90, "indSN": indSNSim}}, nil
	case cst40, cst41, cst51:
		return map[string]any{"ICMS45": map[string]any{"CST": in.CST}}, nil
	}
	p, err := rate()
	if err != nil {
		return nil, err
	}
	vICMS := q2(vBC.Mul(p).Div(hundred))
	switch in.CST {
	case cst00:
		return map[string]any{"ICMS00": map[string]any{"CST": cst00, "vBC": q2(vBC), "pICMS": q2(p), "vICMS": vICMS}}, nil
	case cst20:
		if !pRed.IsPositive() {
			return nil, unprocessable(CodeICMSRateRequired, "informe o percentual de redução da base para o CST 20")
		}
		return map[string]any{"ICMS20": map[string]any{"CST": cst20, "pRedBC": q2(pRed), "vBC": q2(vBC), "pICMS": q2(p), "vICMS": vICMS}}, nil
	case cst60:
		return map[string]any{"ICMS60": map[string]any{"CST": cst60, "vBCSTRet": q2(vBC), "pICMSSTRet": q2(p), "vICMSSTRet": vICMS}}, nil
	}
	n := map[string]any{"CST": cst90, "vBC": q2(vBC), "pICMS": q2(p), "vICMS": vICMS}
	if pRed.IsPositive() {
		n["pRedBC"] = q2(pRed)
	}
	return map[string]any{"ICMS90": n}, nil
}

// buildRecord é a linha da tabela ctes (mesmas chaves e GSIs das outras tabelas de documento).
func buildRecord(pk string, p buildParams, cst, userID, userName string) map[string]any {
	party := func(a actor) map[string]any { return map[string]any{"doc": a.doc(), "name": a.Name} }
	place := func(pl Place) map[string]any {
		return map[string]any{"uf": pl.UF, "city_ibge": pl.CityIBGE, "city": pl.City}
	}
	taker := party(p.takerActor())
	taker["role"] = p.takerRole
	return map[string]any{
		"pk": pk, "sk": p.accessKey, "incoming": 0,
		"year": p.now.Year(), "month": int(p.now.Month()), "day": p.now.Day(),
		"status": StatusPending, "sefaz_status": nil, "sefaz_motive": nil,
		"emit_cpf_cnpj": p.emit.doc(), "emit_name": p.emit.Name,
		"dest_cpf_cnpj": p.dest.doc(), "dest_name": p.dest.Name,
		"number": p.number, "serie": p.serie, "cfop": p.cfop,
		"dh_emi": p.now.Format(layoutDateTimeTZ),
		"total":  q2(p.vTPrest), "freight_total": q2(p.vTPrest),
		"sender": party(p.rem), "recipient": party(p.dest), "taker": taker,
		"nfe_keys": p.nfeKeys, "origin": place(p.origin), "destination": place(p.destination),
		"cargo": map[string]any{
			"total_value": q2(p.vCarga), "total_weight_kg": p.weight.StringFixed(3),
			"volumes": p.volumes, "predominant_product": p.proPred,
		},
		"icms_cst":   cst,
		"xml_s3_key": nil,
		"created_at": p.now.UTC().Format(time.RFC3339),
		"user_id":    userID, "user_name": userName,
	}
}

func sefazEnvFor(environment int) string {
	if environment == 1 {
		return services.SefazEnvProd
	}
	return services.SefazEnvHom
}

// --- atributos ---

func personMap(item map[string]types.AttributeValue) map[string]any {
	var out map[string]any
	if err := attributevalue.UnmarshalMap(item, &out); err != nil {
		return map[string]any{}
	}
	if p, ok := out["person"].(map[string]any); ok {
		return p
	}
	return map[string]any{}
}

func strAttr(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func intAttr(item map[string]types.AttributeValue, key string, def int) int {
	v, ok := item[key].(*types.AttributeValueMemberN)
	if !ok {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v.Value, "%d", &n); err != nil {
		return def
	}
	return n
}

func anyStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func anyInt(m map[string]any, key string, def int) int {
	switch n := m[key].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return def
}

func valueOr(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
