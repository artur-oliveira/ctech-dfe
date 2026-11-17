package bpes

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	bpeVersao = "1.00"
	bpeXMLNS  = "http://www.portalfiscal.inf.br/bpe"
	// bpeQRCodeBase é o endereço do Portal Nacional (SVRS) do QR Code do BP-e (portal SVRS, serviços do BP-e).
	bpeQRCodeBase = "https://dfe-portal.svrs.rs.gov.br/bpe/qrCode"

	tpEmisNormal  = services.TpEmisNormal // forma de emissão: normal
	tpEmisOffline = "2"                   // contingência offline

	tpBPeNormal = "0" // BP-e normal; substituição (3) fica fora desta versão

	layoutDateTimeTZ = "2006-01-02T15:04:05-07:00"

	icmsCSTSimples = "90" // ICMSSN: CST 90 com indSN 1
	icmsSN         = "sn"
	indSN          = "1"

	moneyScale = 2
	cBPDigits  = 8
)

// TechData carries the technical-responsible (infRespTec) information.
type TechData struct {
	CNPJ    string
	Name    string
	Email   string
	Phone   string
	Version string
}

// modalCodes maps the API modal to ide/modal (1 rodoviário, 3 aquaviário, 4 ferroviário).
var modalCodes = map[string]string{ModalRodoviario: "1", ModalAquaviario: "3", ModalFerroviario: "4"}

// buildParams carries everything BuildBPe needs.
type buildParams struct {
	org         map[string]types.AttributeValue
	orgPK       string
	accessKey   string
	serie       int
	number      int
	environment int
	now         time.Time
	req         BpeEmitBody
	tm          BpeTMEmitBody // corpo do TM, usado quando variant é tm
	variant     string
	tpEmis      string
	// qrSign é a assinatura da chave (parâmetro sign do QR Code) na contingência offline.
	qrSign            string
	contingencyReason string
	tech              TechData
	csrtID, csrt      string
}

// GenerateAccessKey monta a chave de 44 posições do BP-e: cUF(2) AAMM(4) CNPJ(14) mod(2) série(3) nBP(9) tpEmis(1) cBP(8)
// DV(1). Devolve "" para UF desconhecida.
func GenerateAccessKey(uf, cnpj string, serie, number int, now time.Time, tpEmis string) string {
	cUF, ok := services.UFCode[uf]
	if !ok {
		return ""
	}
	cnpj = strings.ToUpper(services.StripPKPrefix(cnpj))
	if len(cnpj) > 14 {
		cnpj = cnpj[:14]
	}
	for len(cnpj) < 14 {
		cnpj = "0" + cnpj
	}
	cBP := 10_000_000 + rand.IntN(90_000_000) // 8 dígitos, sem zero à esquerda
	key43 := fmt.Sprintf("%s%s%s%s%03d%09d%s%0*d", cUF, now.Format("0601"), cnpj, services.ModelBPe, serie, number, tpEmis, cBPDigits, cBP)
	return key43 + services.CalcMod11DV(key43)
}

// totals são os valores que o servidor calcula a partir do corpo.
type totals struct{ vBP, discount, vPgto, vTroco decimal.Decimal }

func computeTotals(req BpeEmitBody) totals {
	var t totals
	for _, c := range req.Components {
		t.vBP = t.vBP.Add(parseDec(c.Value))
	}
	t.discount = parseOpt(req.Discount)
	t.vPgto = t.vBP.Sub(t.discount)
	paid := decimal.Zero
	for _, p := range req.Payments {
		paid = paid.Add(parseDec(p.Value))
	}
	if over := paid.Sub(t.vPgto); over.Sign() > 0 {
		t.vTroco = over
	}
	return t
}

// BuildBPe monta o corpo da autorização (raiz BPe) que o go-dfe assina e envia.
func BuildBPe(p buildParams) map[string]any {
	orgPerson := personMap(p.org)
	t := computeTotals(p.req)
	inf := map[string]any{
		"@versao":     bpeVersao,
		"@Id":         "BPe" + p.accessKey,
		"ide":         p.buildIde(),
		"emit":        buildEmit(p.org, orgPerson, p.orgPK),
		"infPassagem": p.buildPassage(),
		"infViagem":   p.buildTrips(),
		"infValorBPe": p.buildValue(t),
		"imp":         p.buildTax(t),
		"pag":         p.buildPayments(),
	}
	if p.req.Buyer != nil {
		inf["comp"] = p.buildBuyer(*p.req.Buyer)
	}
	if aut := services.AutXMLNodes(p.req.AuthorizedXML); aut != nil {
		inf["autXML"] = aut
	}
	if p.req.AdditionalInfo != nil && *p.req.AdditionalInfo != "" {
		inf["infAdic"] = map[string]any{"infCpl": *p.req.AdditionalInfo}
	}
	if rt := p.respTec(); rt != nil {
		inf["infRespTec"] = rt
	}
	return services.PruneEmpty(map[string]any{
		"BPe": map[string]any{
			"@xmlns":     bpeXMLNS,
			"infBPe":     inf,
			"infBPeSupl": map[string]any{"qrCodBPe": p.qrCode()},
		},
	}).(map[string]any)
}

func (p buildParams) respTec() map[string]any {
	if p.tech.CNPJ == "" {
		return nil
	}
	return services.BuildRespTec(p.tech.CNPJ, p.tech.Name, p.tech.Email, onlyDigits(p.tech.Phone), p.csrtID, p.csrt, p.accessKey)
}

// qrCode é URL?chBPe=...&tpAmb=... mais &sign=... na contingência offline.
func (p buildParams) qrCode() string {
	u := fmt.Sprintf("%s?chBPe=%s&tpAmb=%d", bpeQRCodeBase, p.accessKey, p.environment)
	if p.qrSign != "" {
		u += "&sign=" + p.qrSign
	}
	return u
}

func (p buildParams) buildIde() map[string]any {
	r := p.req
	ide := map[string]any{
		"cUF": p.accessKey[0:2], "tpAmb": strconv.Itoa(p.environment), "mod": services.ModelBPe,
		"serie": strconv.Itoa(p.serie), "nBP": strconv.Itoa(p.number), "cBP": p.accessKey[35:43], "cDV": p.accessKey[43:],
		"modal": modalCodes[r.Modal], "dhEmi": p.now.Format(layoutDateTimeTZ), "tpEmis": p.tpEmis,
		"verProc": strOrDefault(p.tech.Version, "ctech-dfe"), "tpBPe": tpBPeNormal, "indPres": r.Presence,
		"UFIni": r.Origin.UF, "cMunIni": r.Origin.IBGECode, "UFFim": r.Destination.UF, "cMunFim": r.Destination.IBGECode,
	}
	if p.tpEmis == tpEmisOffline {
		ide["dhCont"] = p.now.Format(layoutDateTimeTZ)
		ide["xJust"] = p.contingencyReason
	}
	return ide
}

func buildEmit(org map[string]types.AttributeValue, orgPerson map[string]any, orgPK string) map[string]any {
	addr := services.FirstAddress(orgPerson)
	ender := map[string]any{
		"xLgr": anyStr(addr, "street"), "nro": strOrDefault(anyStr(addr, "number"), "S/N"),
		"xBairro": anyStr(addr, "neighborhood"), "cMun": strOrDefault(anyStr(addr, "city_ibge_code"), "0000000"),
		"xMun": anyStr(addr, "city"), "CEP": onlyDigits(anyStr(addr, "postal_code")), "UF": anyStr(addr, "state_federation"),
	}
	if cpl := anyStr(addr, "complement"); cpl != "" {
		ender["xCpl"] = cpl
	}
	if email := services.FirstEmail(orgPerson); email != "" {
		ender["email"] = email
	}
	if fone := onlyDigits(services.FirstPhone(orgPerson)); fone != "" {
		ender["fone"] = fone
	}
	crt := 3
	if v, ok := orgPerson["crt"].(float64); ok {
		crt = int(v)
	}
	emit := map[string]any{"xNome": strAttr(org, "name"), "enderEmit": ender, "CRT": strconv.Itoa(crt)}
	if fant := anyStr(orgPerson, "fantasy_name"); fant != "" {
		emit["xFant"] = fant
	}
	// Off the record: a company id carries no document, and reading the key would put the UUID in the emit node.
	if doc, _ := services.IssuerDocAV(org, orgPK); doc != "" {
		emit["CNPJ"] = doc
	}
	if ie := services.IEForUF(orgPerson, anyStr(addr, "state_federation")); ie != "" {
		emit["IE"] = ie
	}
	return emit
}

func (p buildParams) buildBuyer(b BpeBuyer) map[string]any {
	a := b.Address
	ender := map[string]any{
		"xLgr": a.Street, "nro": a.Number, "xBairro": a.Neighborhood, "cMun": a.IBGECode, "xMun": a.City, "UF": a.UF,
	}
	setIfPtr(ender, "xCpl", a.Complement)
	setIfPtr(ender, "CEP", a.PostalCode)
	setIfPtr(ender, "fone", a.Phone)
	setIfPtr(ender, "email", a.Email)
	comp := map[string]any{"xNome": b.Name, "enderComp": ender}
	switch {
	case b.CNPJ != nil && *b.CNPJ != "":
		comp["CNPJ"] = docAlnum(*b.CNPJ)
	case b.CPF != nil && *b.CPF != "":
		comp["CPF"] = onlyDigits(*b.CPF)
	case b.ForeignID != nil:
		comp["idEstrangeiro"] = *b.ForeignID
	}
	setIfPtr(comp, "IE", b.IE)
	return comp
}

func (p buildParams) buildPassage() map[string]any {
	ps := p.req.Passage
	out := map[string]any{
		"cLocOrig": ps.FromCode, "xLocOrig": ps.FromName, "cLocDest": ps.ToCode, "xLocDest": ps.ToName,
		"dhEmb": ps.Boarding, "dhValidade": ps.Validity,
	}
	if px := ps.Passenger; px != nil {
		pass := map[string]any{"xNome": px.Name, "tpDoc": px.DocType, "nDoc": px.DocNumber}
		if px.CPF != nil && *px.CPF != "" {
			pass["CPF"] = onlyDigits(*px.CPF)
		}
		setIfPtr(pass, "xDoc", px.DocDesc)
		setIfPtr(pass, "dNasc", px.Birth)
		setIfPtr(pass, "fone", px.Phone)
		setIfPtr(pass, "email", px.Email)
		out["infPassageiro"] = pass
	}
	return out
}

func (p buildParams) buildTrips() []any {
	out := make([]any, 0, len(p.req.Trips))
	for _, tr := range p.req.Trips {
		v := map[string]any{
			"cPercurso": tr.RouteCode, "xPercurso": tr.RouteName, "tpViagem": tr.TripType, "tpServ": tr.ServiceType,
			"tpAcomodacao": tr.Accommodation, "tpTrecho": tr.LegType, "dhViagem": tr.Departure,
		}
		setIfPtr(v, "dhConexao", tr.Connection)
		setIfPtr(v, "prefixo", tr.Prefix)
		setIfPtr(v, "poltrona", tr.Seat)
		setIfPtr(v, "plataforma", tr.Platform)
		if c := tr.Crossing; c != nil {
			v["infTravessia"] = map[string]any{"tpVeiculo": c.VehicleType, "sitVeiculo": c.VehicleStatus}
		}
		out = append(out, v)
	}
	return out
}

func (p buildParams) buildValue(t totals) map[string]any {
	comps := make([]any, 0, len(p.req.Components))
	for _, c := range p.req.Components {
		comps = append(comps, map[string]any{"tpComp": c.Type, "vComp": money(parseDec(c.Value))})
	}
	v := map[string]any{
		"vBP": money(t.vBP), "vDesconto": money(t.discount), "vPgto": money(t.vPgto), "vTroco": money(t.vTroco), "Comp": comps,
	}
	setIfPtr(v, "tpDesconto", p.req.DiscountType)
	setIfPtr(v, "xDesconto", p.req.DiscountDesc)
	setIfPtr(v, "cDesconto", p.req.DiscountCode)
	return v
}

// buildTax monta o grupo imp. Base do ICMS = valor pago (valor do bilhete menos o desconto).
func (p buildParams) buildTax(t totals) map[string]any {
	icms := map[string]any{}
	switch p.req.ICMS.CST {
	case "00", "90":
		rate := parseOpt(p.req.ICMS.Rate)
		value := t.vPgto.Mul(rate).Div(decimal.NewFromInt(100)).Round(moneyScale)
		icms["ICMS"+p.req.ICMS.CST] = map[string]any{"CST": p.req.ICMS.CST, "vBC": money(t.vPgto), "pICMS": rateStr(rate), "vICMS": money(value)}
	case "40", "41", "51":
		icms["ICMS45"] = map[string]any{"CST": p.req.ICMS.CST}
	case icmsSN:
		icms["ICMSSN"] = map[string]any{"CST": icmsCSTSimples, "indSN": indSN}
	}
	return map[string]any{"ICMS": icms}
}

func (p buildParams) buildPayments() []any {
	out := make([]any, 0, len(p.req.Payments))
	for _, pay := range p.req.Payments {
		m := map[string]any{"tPag": pay.Type, "vPag": money(parseDec(pay.Value))}
		setIfPtr(m, "xPag", pay.Desc)
		setIfPtr(m, "nDocPag", pay.Doc)
		if c := pay.Card; c != nil {
			card := map[string]any{"tpIntegra": c.Integration}
			if c.CNPJ != nil && *c.CNPJ != "" {
				card["CNPJ"] = docAlnum(*c.CNPJ)
			}
			setIfPtr(card, "tBand", c.Brand)
			setIfPtr(card, "cAut", c.Authorization)
			m["card"] = card
		}
		out = append(out, m)
	}
	return out
}

// --- número e texto ---

func parseDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return decimal.Zero
	}
	return d
}

func parseOpt(s *string) decimal.Decimal {
	if s == nil {
		return decimal.Zero
	}
	return parseDec(*s)
}

func money(d decimal.Decimal) string { return d.StringFixed(moneyScale) }

// rateStr formata uma alíquota em %: inteira ("12") ou com 2 a 4 casas ("0.65").
func rateStr(d decimal.Decimal) string {
	d = d.Round(4)
	if d.IsInteger() {
		return d.StringFixed(0)
	}
	places := -d.Exponent()
	if places < 2 {
		places = 2
	}
	return d.StringFixed(places)
}

func personMap(org map[string]types.AttributeValue) map[string]any {
	var out map[string]any
	if err := attributevalue.UnmarshalMap(org, &out); err != nil {
		return map[string]any{}
	}
	if person, ok := out["person"].(map[string]any); ok {
		return person
	}
	return map[string]any{}
}

func anyStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func strOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func setIfPtr(m map[string]any, key string, val *string) {
	if val != nil && *val != "" {
		m[key] = *val
	}
}

// onlyDigits mantém só os dígitos (CPF, CEP, telefone).
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// docAlnum normaliza CNPJ sem perder a letra do CNPJ alfanumérico (IN RFB 2229/2024).
func docAlnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func strAttr(item map[string]types.AttributeValue, key string) string {
	v, ok := item[key].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}
