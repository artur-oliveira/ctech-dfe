package ctes

// os.go: CT-e OS (modelo 67, cteOS_v4.00.xsd), tpCTe 0, modal rodoviário (01), emissão normal ou em contingência.
// Spec: docs/specs/2026-11-05-cte-emissao-rodoviaria.md (seção CT-e OS) e MOC CT-e, regras H001 a H133.
// Três tipos de serviço (ide/tpServ): 6 transporte de pessoas, 7 transporte de valores e 8 excesso de bagagem.
// Diferente do CT-e 57, não há notas: o usuário informa o serviço, o tomador (sempre, para o indIEToma ficar
// definido), o percurso e os dados do tipo. Nada fiscal é presumido (CFOP, ICMS e tomador são do usuário).

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// Tipos de serviço do CT-e OS (corpo da API) e o ide/tpServ de cada um.
const (
	OsServicePeople  = "people"
	OsServiceValues  = "values"
	OsServiceBaggage = "baggage"

	tpServOsPeople  = "6"
	tpServOsValues  = "7"
	tpServOsBaggage = "8"

	sefazServiceAutorizacaoOS = "CTeRecepcaoOS"
	// ModelCTeOS é o modelo do CT-e OS na chave de acesso.
	ModelCTeOS = validationModCTeOS

	// Fretamento (rodoOS/infFretamento/tpFretamento): 1 eventual, 2 contínuo.
	fretamentoEventual = 1

	// Componentes da GTV-e (infGTVe/Comp/tpComp); 6 é "outros" e exige xComp (H052).
	gtveCompOthers = 6

	// osTotalTolerance é a tolerância de arredondamento da soma dos componentes da GTV-e (H053).
	osTotalTolerance = "0.10"
	// osMaxRouteUFs é o limite de infPercurso do leiaute.
	osMaxRouteUFs = 25
	// osXDescServMax é o tamanho de xDescServ (1 a 30).
	osXDescServMax = 30
	// osBPeAgeMonths: a chave do BP-e referenciado não pode ser anterior a 6 meses (H029a, 934).
	osBPeAgeMonths = 6
)

// validationModCTeOS repete o modelo 67 de validation.AccessKeyModCTeOS sem importar o pacote só por isso.
const validationModCTeOS = "67"

// Códigos (problem.code) de 422 do CT-e OS.
const (
	CodeOsEmitterNotCNPJ      = "os_emitter_cnpj_required"
	CodeOsTakerRequired       = "os_taker_required"
	CodeOsPlaceRequired       = "os_place_required"
	CodeOsPlaceUFMismatch     = "os_place_uf_mismatch"
	CodeOsRoadRequired        = "os_road_required"
	CodeOsRoadNotAllowed      = "os_road_not_allowed"
	CodeOsRegistrationNeeded  = "os_state_registration_required"
	CodeOsTAFNeeded           = "os_taf_required"
	CodeOsCharterRequired     = "os_charter_required"
	CodeOsTripAtRequired      = "os_trip_at_required"
	CodeOsTripAtPast          = "os_trip_at_past"
	CodeOsDocRefsRequired     = "os_doc_refs_required"
	CodeOsDocRefsNotAllowed   = "os_doc_refs_not_allowed"
	CodeOsGTVeRequired        = "os_gtve_required"
	CodeOsGTVeNotAllowed      = "os_gtve_not_allowed"
	CodeOsGTVeInvalid         = "os_gtve_invalid"
	CodeOsGTVeTotalMismatch   = "os_gtve_total_mismatch"
	CodeOsINSSRequired        = "os_inss_required"
	CodeOsICMS60NotAllowed    = "os_icms_cst_not_allowed"
	CodeOsBPeKeyInvalid       = "os_bpe_key_invalid"
	CodeOsInvalidFederalTaxes = "os_federal_taxes_invalid"
)

// CteOsEmitBody é o corpo de POST /ctes/os.
type CteOsEmitBody struct {
	// ServiceType: people (6), values (7) ou baggage (8).
	ServiceType string `json:"service_type" validate:"required,oneof=people values baggage"`
	// Taker é a pessoa do cadastro que paga o serviço (toma). Sempre obrigatório.
	Taker CteOsTaker `json:"taker" validate:"required"`
	// Service descreve o serviço (infServico): xDescServ e a quantidade (passageiros, volumes ou malotes).
	Service CteOsService `json:"service" validate:"required"`
	// Origin e Destination: obrigatórios para pessoas (origem e destino) e para valores (destino).
	Origin      *CteOsPlace `json:"origin"`
	Destination *CteOsPlace `json:"destination"`
	// RouteUFs é o percurso (infPercurso), em ordem origem para destino.
	RouteUFs []string `json:"route_ufs" validate:"omitempty,max=25,dive,len=2"`
	// Road é o modal rodoviário OS: obrigatório para pessoas e excesso de bagagem, proibido para valores (H015, H018).
	Road *CteOsRoad `json:"road"`
	// DocRefs são os documentos referenciados do excesso de bagagem (H028); proibidos nos outros tipos.
	DocRefs []CteOsDocRef `json:"doc_refs" validate:"omitempty,max=500,dive"`
	// GTVe são as GTV-e referenciadas do transporte de valores (H045a); proibidas nos outros tipos.
	GTVe []CteOsGTVe `json:"gtve" validate:"omitempty,max=500,dive"`
	// FederalTaxes é o infTribFed; vINSS é obrigatório para pessoas com tomador pessoa jurídica (H057).
	FederalTaxes *CteOsFederalTaxes `json:"federal_taxes"`
	CteChargeBody
}

type CteOsTaker struct {
	PersonID string `json:"person_id" validate:"required"`
}

type CteOsService struct {
	Description string  `json:"description" validate:"required,min=1,max=30"`
	Quantity    *string `json:"quantity" validate:"omitempty,decimalv"`
}

// CteOsPlace é um município (o API não tem tabela de nomes de município do IBGE: a UI traz o picker).
type CteOsPlace struct {
	CityIBGE string `json:"city_ibge" validate:"required,ibge"`
	City     string `json:"city" validate:"required,min=2,max=60"`
	UF       string `json:"uf" validate:"required,len=2"`
}

type CteOsRoad struct {
	// TAF (12 dígitos) ou StateRegistration (NroRegEstadual, 25 dígitos): um deles. Pessoas: interestadual exige
	// TAF (H024) e interna exige o registro estadual (H023).
	TAF               *string       `json:"taf" validate:"omitempty,len=12,numeric"`
	StateRegistration *string       `json:"state_registration" validate:"omitempty,len=25,numeric"`
	Vehicle           *CteOsVehicle `json:"vehicle"`
	// Charter (infFretamento): obrigatório para pessoas (H025), proibido nos outros.
	Charter *CteOsCharter `json:"charter"`
}

type CteOsVehicle struct {
	Plate   string  `json:"plate" validate:"required,min=7,max=7,alphanum"`
	Renavam *string `json:"renavam" validate:"omitempty,min=9,max=11,numeric"`
	UF      *string `json:"uf" validate:"omitempty,len=2"`
}

type CteOsCharter struct {
	// Type: 1 eventual (exige trip_at, H026) ou 2 contínuo.
	Type   int     `json:"type" validate:"required,oneof=1 2"`
	TripAt *string `json:"trip_at" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
}

// CteOsDocRef é um documento referenciado do excesso de bagagem: chave de BP-e, ou número, série, emissão e valor.
type CteOsDocRef struct {
	BPeKey    *string `json:"bpe_key" validate:"omitempty,len=44"`
	Number    *string `json:"number" validate:"omitempty,min=1,max=20"`
	Serie     *string `json:"serie" validate:"omitempty,max=3"`
	Subserie  *string `json:"subserie" validate:"omitempty,max=3"`
	IssuedOn  *string `json:"issued_on" validate:"omitempty,datetime=2006-01-02"`
	DocAmount *string `json:"amount" validate:"omitempty,money2"`
}

type CteOsGTVe struct {
	Key        string          `json:"key" validate:"required,dfe_access_key=64"`
	Components []CteOsGTVeComp `json:"components" validate:"required,min=1,max=6,dive"`
}

type CteOsGTVeComp struct {
	// Type (tpComp): 1 a 6 (6 é outros e exige name).
	Type  int     `json:"type" validate:"required,gte=1,lte=6"`
	Value string  `json:"value" validate:"required,money2"`
	Name  *string `json:"name" validate:"omitempty,max=15"`
}

type CteOsFederalTaxes struct {
	PIS    *string `json:"pis" validate:"omitempty,money2"`
	COFINS *string `json:"cofins" validate:"omitempty,money2"`
	IR     *string `json:"ir" validate:"omitempty,money2"`
	INSS   *string `json:"inss" validate:"omitempty,money2"`
	CSLL   *string `json:"csll" validate:"omitempty,money2"`
}

var osTpServ = map[string]string{OsServicePeople: tpServOsPeople, OsServiceValues: tpServOsValues, OsServiceBaggage: tpServOsBaggage}

// osData é o que o builder do CT-e OS usa além do buildParams.
type osData struct {
	serviceType string
	taker       actor
	description string
	quantity    string
	routeUFs    []string
	road        map[string]any
	docRefs     []map[string]any
	gtve        []map[string]any
	federal     map[string]any
}

// CteOsEmit valida, monta e grava o CT-e OS (pending) com o outbox na mesma transação da numeração.
func (s *CteService) CteOsEmit(ctx context.Context, orgPK string, req CteOsEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	cfg, environment, err := s.config(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	emit, emitDoc, crt, err := s.issuer(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if emit.CNPJ == "" {
		return nil, unprocessable(CodeOsEmitterNotCNPJ, "o CT-e OS só pode ser emitido por pessoa jurídica (o XSD do emitente só aceita CNPJ)")
	}
	taker, err := s.personActorByID(ctx, orgPK, req.Taker.PersonID)
	if err != nil {
		return nil, err
	}
	p, err := osParams(req, emit, taker, crt, time.Now())
	if err != nil {
		return nil, err
	}
	ch, err := resolveCharge(req.CteChargeBody, crt, []string{emit.doc(), taker.doc()})
	if err != nil {
		return nil, err
	}
	if req.ICMS != nil && req.ICMS.CST == cst60 {
		return nil, unprocessable(CodeOsICMS60NotAllowed, "o CT-e OS não tem o grupo ICMS60; escolha outro CST")
	}
	if err := checkGTVeTotal(req, ch.vTPrest); err != nil {
		return nil, err
	}
	ch.apply(&p, req.CteChargeBody)
	return s.persist(ctx, orgPK, cfg, environment, emitDoc, p, ch.cst, userID, userName)
}

// osParams valida o corpo contra as regras do MOC provadas só com ele (H015 a H028, H045, H057) e monta os
// parâmetros. Origem, destino e tomador já chegam resolvidos.
func osParams(req CteOsEmitBody, emit, taker actor, crt int, now time.Time) (buildParams, error) {
	p := buildParams{emit: emit, crt: crt, os: &osData{serviceType: req.ServiceType, taker: taker}}
	o := p.os
	o.description = strings.TrimSpace(req.Service.Description)
	if req.Service.Quantity != nil {
		o.quantity = nfexml.Dec(*req.Service.Quantity).StringFixed(4)
	}
	p.serviceType = osTpServ[req.ServiceType]
	for _, pl := range []struct {
		in  *CteOsPlace
		dst *Place
	}{{req.Origin, &p.origin}, {req.Destination, &p.destination}} {
		if pl.in == nil {
			continue
		}
		if services.UFCode[pl.in.UF] != pl.in.CityIBGE[:2] {
			return p, unprocessable(CodeOsPlaceUFMismatch, "o município "+pl.in.CityIBGE+" não pertence à UF "+pl.in.UF+" (H117, H119)")
		}
		*pl.dst = Place{UF: pl.in.UF, CityIBGE: pl.in.CityIBGE, City: strings.TrimSpace(pl.in.City)}
	}
	if len(req.RouteUFs) > osMaxRouteUFs {
		return p, unprocessable(CodeOsPlaceRequired, fmt.Sprintf("o percurso tem no máximo %d UFs", osMaxRouteUFs))
	}
	o.routeUFs = req.RouteUFs

	switch req.ServiceType {
	case OsServicePeople:
		if req.Origin == nil || req.Destination == nil {
			return p, unprocessable(CodeOsPlaceRequired, "informe origem e destino da prestação para o transporte de pessoas (H019, H020)")
		}
	case OsServiceValues:
		if req.Destination == nil {
			return p, unprocessable(CodeOsPlaceRequired, "informe o destino da prestação para o transporte de valores (H020a)")
		}
	}
	if err := osRoad(req, p.origin, p.destination, o, now); err != nil {
		return p, err
	}
	if err := osDocs(req, o, now); err != nil {
		return p, err
	}
	if err := osFederal(req, taker, o); err != nil {
		return p, err
	}
	return p, nil
}

// osRoad aplica H015, H018, H023 a H027 e monta o rodoOS.
func osRoad(req CteOsEmitBody, origin, dest Place, o *osData, now time.Time) error {
	if req.ServiceType == OsServiceValues {
		if req.Road != nil {
			return unprocessable(CodeOsRoadNotAllowed, "o transporte de valores não leva dados do modal rodoviário (H018)")
		}
		return nil
	}
	if req.Road == nil {
		return unprocessable(CodeOsRoadRequired, "informe os dados do modal rodoviário para pessoas e excesso de bagagem (H015)")
	}
	r := req.Road
	road := map[string]any{}
	switch {
	case r.TAF != nil && r.StateRegistration != nil:
		return unprocessable(CodeOsTAFNeeded, "informe o TAF ou o registro estadual, não os dois")
	case r.TAF != nil:
		road["TAF"] = *r.TAF
	case r.StateRegistration != nil:
		road["NroRegEstadual"] = *r.StateRegistration
	default:
		return unprocessable(CodeOsTAFNeeded, "informe o TAF ou o registro estadual do modal rodoviário")
	}
	if req.ServiceType == OsServicePeople {
		if err := osPeopleRegistration(r, origin, dest); err != nil {
			return err
		}
		ch, err := osCharter(r.Charter, now)
		if err != nil {
			return err
		}
		road["infFretamento"] = ch
	} else if r.Charter != nil {
		return unprocessable(CodeOsCharterRequired, "o fretamento só vale para o transporte de pessoas")
	}
	if v := r.Vehicle; v != nil {
		veic := map[string]any{"placa": strings.ToUpper(v.Plate), "RENAVAM": valueOr(v.Renavam), "UF": valueOr(v.UF)}
		road["veic"] = veic
	}
	o.road = road
	return nil
}

// osPeopleRegistration: operação interna exige NroRegEstadual (H023); interestadual exige TAF (H024). Com UF EX
// (exterior) nenhuma das duas se aplica.
func osPeopleRegistration(r *CteOsRoad, origin, dest Place) error {
	if origin.UF == "EX" || dest.UF == "EX" {
		return nil
	}
	if origin.UF == dest.UF {
		if r.StateRegistration == nil {
			return unprocessable(CodeOsRegistrationNeeded, "operação interna de pessoas exige o número do registro estadual (H023)")
		}
		return nil
	}
	if r.TAF == nil {
		return unprocessable(CodeOsTAFNeeded, "operação interestadual de pessoas exige o Termo de Autorização de Fretamento (H024)")
	}
	return nil
}

func osCharter(c *CteOsCharter, now time.Time) (map[string]any, error) {
	if c == nil {
		return nil, unprocessable(CodeOsCharterRequired, "informe o fretamento para o transporte de pessoas (H025)")
	}
	out := map[string]any{"tpFretamento": fmt.Sprintf("%d", c.Type)}
	if c.Type == fretamentoEventual && c.TripAt == nil {
		return nil, unprocessable(CodeOsTripAtRequired, "informe data e hora da viagem no fretamento eventual (H026)")
	}
	if c.TripAt != nil {
		t, err := time.Parse(time.RFC3339, *c.TripAt)
		if err != nil {
			return nil, unprocessable(CodeOsTripAtRequired, "data e hora da viagem inválidas")
		}
		if !t.After(now) {
			return nil, unprocessable(CodeOsTripAtPast, "a data e hora da viagem devem ser posteriores à emissão (H027)")
		}
		out["dhViagem"] = t.Format(layoutDateTimeTZ)
	}
	return out, nil
}

// osDocs aplica H028 (excesso de bagagem exige documentos), H045 e H045a (GTV-e só em valores e obrigatória nele),
// H046/H046a/H051/H052 e a chave do BP-e (H029, H029a).
func osDocs(req CteOsEmitBody, o *osData, now time.Time) error {
	if req.ServiceType == OsServiceBaggage {
		if len(req.DocRefs) == 0 {
			return unprocessable(CodeOsDocRefsRequired, "informe os documentos referenciados do excesso de bagagem (H028)")
		}
		for _, d := range req.DocRefs {
			n, err := osDocRef(d, now)
			if err != nil {
				return err
			}
			o.docRefs = append(o.docRefs, n)
		}
	} else if len(req.DocRefs) > 0 {
		return unprocessable(CodeOsDocRefsNotAllowed, "documentos referenciados só valem para excesso de bagagem")
	}
	if req.ServiceType != OsServiceValues {
		if len(req.GTVe) > 0 {
			return unprocessable(CodeOsGTVeNotAllowed, "a GTV-e só pode ser informada no transporte de valores (H045)")
		}
		return nil
	}
	if len(req.GTVe) == 0 {
		return unprocessable(CodeOsGTVeRequired, "informe as GTV-e do transporte de valores (H045a)")
	}
	var seen []string
	for _, g := range req.GTVe {
		if slices.Contains(seen, g.Key) {
			return unprocessable(CodeOsGTVeInvalid, "GTV-e em duplicidade: "+g.Key+" (H051)")
		}
		seen = append(seen, g.Key)
		if err := checkRecentKey(g.Key, osBPeAgeMonths, now); err != nil {
			return unprocessable(CodeOsGTVeInvalid, "a GTV-e "+g.Key+" tem mais de 6 meses (H046a)")
		}
		comps := make([]map[string]any, 0, len(g.Components))
		for _, c := range g.Components {
			if c.Type == gtveCompOthers && strings.TrimSpace(valueOr(c.Name)) == "" {
				return unprocessable(CodeOsGTVeInvalid, "informe o nome do componente do tipo outros (H052)")
			}
			comps = append(comps, map[string]any{"tpComp": fmt.Sprintf("%d", c.Type), "vComp": q2(nfexml.Dec(c.Value)), "xComp": strings.TrimSpace(valueOr(c.Name))})
		}
		o.gtve = append(o.gtve, map[string]any{"chCTe": g.Key, "Comp": comps})
	}
	return nil
}

func osDocRef(d CteOsDocRef, now time.Time) (map[string]any, error) {
	if d.BPeKey != nil {
		k := *d.BPeKey
		if len(k) != accessKeyLen || onlyDigits(k) != k || k[20:22] != "63" {
			return nil, unprocessable(CodeOsBPeKeyInvalid, "chave de BP-e inválida (44 dígitos, modelo 63): "+k)
		}
		if err := checkRecentKey(k, osBPeAgeMonths, now); err != nil {
			return nil, unprocessable(CodeOsBPeKeyInvalid, "o BP-e "+k+" tem mais de 6 meses (H029a)")
		}
		return map[string]any{"chBPe": k}, nil
	}
	if d.Number == nil || d.IssuedOn == nil {
		return nil, unprocessable(CodeOsDocRefsRequired, "cada documento referenciado leva a chave do BP-e ou número e data de emissão")
	}
	n := map[string]any{"nDoc": *d.Number, "serie": valueOr(d.Serie), "subserie": valueOr(d.Subserie), "dEmi": *d.IssuedOn}
	if d.DocAmount != nil {
		n["vDoc"] = q2(nfexml.Dec(*d.DocAmount))
	}
	return n, nil
}

// checkRecentKey recusa chave com AAMM anterior a maxMonths da data atual.
func checkRecentKey(key string, maxMonths int, now time.Time) error {
	if len(key) != accessKeyLen {
		return fmt.Errorf("chave inválida")
	}
	keyMonths := (2000+atoi(key[2:4]))*12 + atoi(key[4:6])
	if now.Year()*12+int(now.Month())-keyMonths > maxMonths {
		return fmt.Errorf("chave antiga")
	}
	return nil
}

// osFederal monta o infTribFed e aplica H057: pessoas com tomador pessoa jurídica exigem vINSS.
func osFederal(req CteOsEmitBody, taker actor, o *osData) error {
	if req.ServiceType == OsServicePeople && taker.CNPJ != "" && (req.FederalTaxes == nil || req.FederalTaxes.INSS == nil) {
		return unprocessable(CodeOsINSSRequired, "informe o INSS retido (federal_taxes.inss): o tomador é pessoa jurídica (H057)")
	}
	f := req.FederalTaxes
	if f == nil {
		return nil
	}
	n := map[string]any{}
	for k, v := range map[string]*string{"vPIS": f.PIS, "vCOFINS": f.COFINS, "vIR": f.IR, "vINSS": f.INSS, "vCSLL": f.CSLL} {
		if v != nil {
			n[k] = q2(nfexml.Dec(*v))
		}
	}
	o.federal = n
	return nil
}

// checkGTVeTotal aplica H053: a soma dos componentes das GTV-e fecha com o valor da prestação (tolerância 0,10).
func checkGTVeTotal(req CteOsEmitBody, vTPrest decimal.Decimal) error {
	if req.ServiceType != OsServiceValues {
		return nil
	}
	sum := decimal.Zero
	for _, g := range req.GTVe {
		for _, c := range g.Components {
			sum = sum.Add(nfexml.Dec(c.Value))
		}
	}
	if sum.Sub(vTPrest).Abs().GreaterThan(nfexml.Dec(osTotalTolerance)) {
		return unprocessable(CodeOsGTVeTotalMismatch, "a soma dos componentes das GTV-e ("+q2(sum)+") deve fechar com o valor do frete ("+q2(vTPrest)+") (H053)")
	}
	return nil
}

// BuildCTeOS monta o dicionário do CTeOS (autorização síncrona do CT-e OS 4.00; o <CTeOS> é a raiz).
func BuildCTeOS(p buildParams) map[string]any {
	o := p.os
	cUF, cCT, cDV := p.accessKey[0:2], p.accessKey[35:43], string(p.accessKey[43])
	inf := map[string]any{
		"@versao": cteVersao,
		"@Id":     "CTe" + p.accessKey,
		"ide":     p.buildOSIde(cUF, cCT, cDV),
		"emit":    p.buildEmit(),
		"toma":    osToma(o.taker),
		"vPrest":  map[string]any{"vTPrest": q2(p.vTPrest), "vRec": q2(p.vTPrest), "Comp": p.comps},
		"imp":     p.buildOSImp(),
	}
	if p.addInfo != "" {
		inf["compl"] = map[string]any{"xObs": p.addInfo}
	}
	servico := map[string]any{"xDescServ": o.description}
	if o.quantity != "" {
		servico["infQ"] = map[string]any{"qCarga": o.quantity}
	}
	norm := map[string]any{"infServico": servico}
	if len(o.docRefs) > 0 {
		norm["infDocRef"] = o.docRefs
	}
	if o.road != nil {
		norm["infModal"] = map[string]any{"@versaoModal": cteVersao, "rodoOS": o.road}
	}
	if len(o.gtve) > 0 {
		norm["infGTVe"] = o.gtve
	}
	inf["infCTeNorm"] = norm
	if len(p.autXML) > 0 {
		inf["autXML"] = p.autXML
	}
	if p.tech.CNPJ != "" {
		inf["infRespTec"] = services.BuildRespTec(p.tech.CNPJ, p.tech.Name, p.tech.Email, onlyDigits(p.tech.Phone), p.csrtID, p.csrt, p.accessKey)
	}
	return services.PruneEmpty(map[string]any{
		"CTeOS": map[string]any{
			"@xmlns":     cteXMLNS,
			"@versao":    cteVersao, // atributo obrigatório da raiz do cteOS_v4.00.xsd
			"infCte":     inf,
			"infCTeSupl": map[string]any{"qrCodCTe": qrCodeURL(p.emit.Addr.UF, p.accessKey, p.environment, p.qrSign)},
		},
	}).(map[string]any)
}

func (p buildParams) buildOSIde(cUF, cCT, cDV string) map[string]any {
	ide := map[string]any{
		"cUF": cUF, "cCT": cCT, "CFOP": p.cfop, "natOp": natOpTransporte, "mod": ModelCTeOS,
		"serie": fmt.Sprintf("%d", p.serie), "nCT": fmt.Sprintf("%d", p.number), "dhEmi": p.now.Format(layoutDateTimeTZ),
		"tpImp": tpImpRetrato, "tpEmis": p.tpEmis, "cDV": cDV, "tpAmb": fmt.Sprintf("%d", p.environment),
		"tpCTe": tpCTeNormal, "procEmi": procEmiOwn, "verProc": verProc(p.tech),
		"cMunEnv": p.emit.Addr.CityIBGE, "xMunEnv": p.emit.Addr.City, "UFEnv": p.emit.Addr.UF,
		"modal": modalRodoviario, "tpServ": p.serviceType, "indIEToma": indIEToma(p.os.taker.IE),
	}
	if p.origin.CityIBGE != "" {
		ide["cMunIni"], ide["xMunIni"], ide["UFIni"] = p.origin.CityIBGE, p.origin.City, p.origin.UF
	}
	if p.destination.CityIBGE != "" {
		ide["cMunFim"], ide["xMunFim"], ide["UFFim"] = p.destination.CityIBGE, p.destination.City, p.destination.UF
	}
	if len(p.os.routeUFs) > 0 {
		per := make([]map[string]any, 0, len(p.os.routeUFs))
		for _, uf := range p.os.routeUFs {
			per = append(per, map[string]any{"UFPer": uf})
		}
		ide["infPercurso"] = per
	}
	if p.tpEmis == tpEmisOffline {
		ide["dhCont"] = p.contingencyAt.In(p.now.Location()).Format(layoutDateTimeTZ)
		ide["xJust"] = p.justification
	}
	return ide
}

// osToma monta o grupo toma do CT-e OS (choice CNPJ|CPF, IE, xNome, fone, enderToma, email).
func osToma(a actor) map[string]any {
	t := map[string]any{"IE": ieForXML(a.IE), "xNome": a.Name, "fone": onlyDigits(a.Phone), "enderToma": enderNode(a.Addr, true), "email": a.Email}
	setDoc(t, a)
	return t
}

// buildOSImp é o imp do CT-e OS: ICMS, infTribFed e, se informado, IBSCBS com vTotDFe.
func (p buildParams) buildOSImp() map[string]any {
	imp := map[string]any{"ICMS": p.icms}
	if len(p.os.federal) > 0 {
		imp["infTribFed"] = p.os.federal
	}
	if p.ibscbs != nil {
		imp["IBSCBS"] = p.ibscbs.node
		imp["vTotDFe"] = p.ibscbs.totalDFe(p.vTPrest, p.now.Year())
	}
	return imp
}

// osRecord é a linha da tabela ctes de um CT-e OS: mesmas chaves e GSIs do CT-e 57; o tomador ocupa dest_* e taker.
func osRecord(pk string, p buildParams, cst, userID, userName string) map[string]any {
	taker := p.os.taker
	place := func(pl Place) map[string]any {
		return map[string]any{"uf": pl.UF, "city_ibge": pl.CityIBGE, "city": pl.City}
	}
	return map[string]any{
		"pk": pk, "sk": p.accessKey, "incoming": 0,
		"year": p.now.Year(), "month": int(p.now.Month()), "day": p.now.Day(),
		"status": StatusPending, "sefaz_status": nil, "sefaz_motive": nil,
		"emit_cpf_cnpj": p.emit.doc(), "emit_name": p.emit.Name,
		"dest_cpf_cnpj": taker.doc(), "dest_name": taker.Name,
		"number": p.number, "serie": p.serie, "cfop": p.cfop, "dh_emi": p.now.Format(layoutDateTimeTZ),
		"total": q2(p.vTPrest), "freight_total": q2(p.vTPrest),
		"taker":  map[string]any{"role": "tomador", "doc": taker.doc(), "name": taker.Name},
		"origin": place(p.origin), "destination": place(p.destination),
		"icms_cst": cst, "model": ModelCTeOS, "os_service": p.os.serviceType, "nfe_keys": []string{},
		"xml_s3_key": nil, "created_at": p.now.UTC().Format(time.RFC3339), "user_id": userID, "user_name": userName,
	}
}
