package nfcoms

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
	nfcomVersao = "1.00"
	nfcomXMLNS  = "http://www.portalfiscal.inf.br/nfcom"
	// nfcomQRCodeBase é o endereço do Portal Nacional (SVRS) do QR Code da NFCom (portal SVRS, serviços da NFCom).
	nfcomQRCodeBase = "https://dfe-portal.svrs.rs.gov.br/nfcom/qrCode"

	tpEmisNormal  = services.TpEmisNormal // forma de emissão: normal
	tpEmisOffline = "2"                   // contingência offline
	siteNormal    = "0"                   // nSiteAutoriz do ambiente normal de autorização

	finNFComNormal = "0" // NFCom normal; substituição (3) e ajuste (4) ficam fora da v1
	tpFatNormal    = "0" // faturamento normal; centralizado (1) e cofaturamento (2) ficam fora da v1

	layoutDateTimeTZ = "2006-01-02T15:04:05-07:00"

	icmsCSTSimples = "90" // ICMSSN: CST 90 com indSN 1
	icmsSN         = "sn"
	icmsSemCST     = "sem_cst"
	indSN          = "1"
	indSemCST      = "1"
	indPrePago     = "1"

	moneyScale = 2
)

// TechData carries the technical-responsible (infRespTec) information.
type TechData struct {
	CNPJ    string
	Name    string
	Email   string
	Phone   string
	Version string
}

// buildParams carries everything BuildNFCom needs.
type buildParams struct {
	org         map[string]types.AttributeValue
	orgPK       string
	accessKey   string
	serie       int
	number      int
	environment int
	now         time.Time
	req         NfcomEmitBody
	tpEmis      string
	// qrSign é a assinatura da chave (parâmetro sign do QR Code) na contingência offline.
	qrSign            string
	contingencyReason string
	tech              TechData
	csrtID, csrt      string
}

// GenerateAccessKey monta a chave de 44 posições da NFCom: cUF(2) AAMM(4) CNPJ(14) mod(2) série(3) nNF(9) tpEmis(1)
// nSiteAutoriz(1) cNFCom(7) DV(1). Devolve "" para UF desconhecida. O site é sempre 0 (ambiente normal).
func GenerateAccessKey(uf, cnpj string, serie, number int, now time.Time, tpEmis string) string {
	cUF, ok := services.UFCode[uf]
	if !ok {
		return ""
	}
	cnpj = strings.ToUpper(services.StripPKPrefix(cnpj))
	if len(cnpj) > 14 {
		cnpj = cnpj[:14]
	}
	for len(cnpj) < 14 { // CPF entra com zeros à esquerda
		cnpj = "0" + cnpj
	}
	cNF := 1_000_000 + rand.IntN(9_000_000) // 7 dígitos, sem zero à esquerda (o cNF não pode ser todo zero)
	key43 := fmt.Sprintf("%s%s%s%s%03d%09d%s%s%07d", cUF, now.Format("0601"), cnpj, services.ModelNFCom, serie, number, tpEmis, siteNormal, cNF)
	return key43 + services.CalcMod11DV(key43)
}

// BuildNFCom monta o corpo da autorização (raiz NFCom) que o go-dfe assina e envia.
func BuildNFCom(p buildParams) map[string]any {
	orgPerson := personMap(p.org)
	items, totals := p.buildItems()
	inf := map[string]any{
		"@versao":   nfcomVersao,
		"@Id":       "NFCom" + p.accessKey,
		"ide":       p.buildIde(orgPerson),
		"emit":      buildEmit(p.org, orgPerson, p.orgPK),
		"dest":      p.buildDest(),
		"assinante": p.buildSubscriber(),
		"det":       items,
		"total":     totals.node(),
	}
	if p.req.Invoice != nil {
		inf["gFat"] = map[string]any{
			"CompetFat": p.req.Invoice.Competence,
			"dVencFat":  p.req.Invoice.DueDate,
			"codBarras": p.req.Invoice.BarCode,
		}
	}
	if aut := services.AutXMLNodes(p.req.AuthorizedXML); aut != nil {
		inf["autXML"] = aut
	}
	if p.req.AdditionalInfo != nil && *p.req.AdditionalInfo != "" {
		inf["infAdic"] = map[string]any{"infCpl": *p.req.AdditionalInfo}
	}
	if rt := p.respTec(); rt != nil {
		inf["gRespTec"] = rt
	}
	return services.PruneEmpty(map[string]any{
		"NFCom": map[string]any{
			"@xmlns":       nfcomXMLNS,
			"infNFCom":     inf,
			"infNFComSupl": map[string]any{"qrCodNFCom": p.qrCode()},
		},
	}).(map[string]any)
}

func (p buildParams) respTec() map[string]any {
	if p.tech.CNPJ == "" {
		return nil
	}
	return services.BuildRespTec(p.tech.CNPJ, p.tech.Name, p.tech.Email, onlyDigits(p.tech.Phone), p.csrtID, p.csrt, p.accessKey)
}

// qrCode é URL?chNFCom=...&tpAmb=... mais &sign=... na contingência offline.
func (p buildParams) qrCode() string {
	u := fmt.Sprintf("%s?chNFCom=%s&tpAmb=%d", nfcomQRCodeBase, p.accessKey, p.environment)
	if p.qrSign != "" {
		u += "&sign=" + p.qrSign
	}
	return u
}

func (p buildParams) buildIde(orgPerson map[string]any) map[string]any {
	addr := services.FirstAddress(orgPerson)
	ide := map[string]any{
		"cUF":          p.accessKey[0:2],
		"tpAmb":        strconv.Itoa(p.environment),
		"mod":          services.ModelNFCom,
		"serie":        strconv.Itoa(p.serie),
		"nNF":          strconv.Itoa(p.number),
		"cNF":          p.accessKey[36:43],
		"cDV":          p.accessKey[43:],
		"dhEmi":        p.now.Format(layoutDateTimeTZ),
		"tpEmis":       p.tpEmis,
		"nSiteAutoriz": siteNormal,
		"cMunFG":       strOrDefault(anyStr(addr, "city_ibge_code"), "0000000"),
		"finNFCom":     finNFComNormal,
		"tpFat":        tpFatNormal,
		"verProc":      strOrDefault(p.tech.Version, "ctech-dfe"),
	}
	if p.req.Prepaid {
		ide["indPrePago"] = indPrePago
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
		"xLgr":    anyStr(addr, "street"),
		"nro":     strOrDefault(anyStr(addr, "number"), "S/N"),
		"xBairro": anyStr(addr, "neighborhood"),
		"cMun":    strOrDefault(anyStr(addr, "city_ibge_code"), "0000000"),
		"xMun":    anyStr(addr, "city"),
		"CEP":     onlyDigits(anyStr(addr, "postal_code")),
		"UF":      anyStr(addr, "state_federation"),
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
	emit := map[string]any{
		"xNome":     strAttr(org, "name"),
		"enderEmit": ender,
		"CRT":       strconv.Itoa(crt),
	}
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

func (p buildParams) buildDest() map[string]any {
	c := p.req.Customer
	a := c.Address
	ender := map[string]any{
		"xLgr": a.Street, "nro": a.Number, "xBairro": a.Neighborhood, "cMun": a.IBGECode, "xMun": a.City,
		"CEP": a.PostalCode, "UF": a.UF,
	}
	setIfPtr(ender, "xCpl", a.Complement)
	setIfPtr(ender, "fone", a.Phone)
	setIfPtr(ender, "email", a.Email)
	dest := map[string]any{"xNome": c.Name, "indIEDest": c.IEIndicator, "enderDest": ender}
	switch {
	case c.CNPJ != nil && *c.CNPJ != "":
		dest["CNPJ"] = docAlnum(*c.CNPJ)
	case c.CPF != nil && *c.CPF != "":
		dest["CPF"] = onlyDigits(*c.CPF)
	case c.OtherID != nil:
		dest["idOutros"] = *c.OtherID
	}
	if c.IEIndicator != "9" && c.IE != nil {
		// A IE do isento é o literal ISENTO (a validação aceita qualquer caixa).
		dest["IE"] = strings.ToUpper(strings.TrimSpace(*c.IE))
	}
	setIfPtr(dest, "IM", c.MunicipalRegistration)
	return dest
}

func (p buildParams) buildSubscriber() map[string]any {
	s := p.req.Subscriber
	a := map[string]any{"iCodAssinante": s.Code, "tpAssinante": s.Type, "tpServUtil": s.ServiceUsed}
	setIfPtr(a, "nContrato", s.ContractNumber)
	setIfPtr(a, "dContratoIni", s.ContractStart)
	setIfPtr(a, "dContratoFim", s.ContractEnd)
	setIfPtr(a, "NroTermPrinc", s.PrimaryTerminal)
	if s.PrimaryUF != nil {
		if code, ok := services.UFCode[*s.PrimaryUF]; ok {
			a["cUFPrinc"] = code
		}
	}
	setIfPtr(a, "cMunPrinc", s.PrimaryIBGE)
	return a
}

// totals acumula o que o grupo total precisa: somas dos itens.
type totals struct {
	vProd, vDesc, vOutro, vBC, vICMS, vPIS, vCOFINS decimal.Decimal
}

func (t totals) node() map[string]any {
	zero := decimal.Zero
	return map[string]any{
		"vProd":   money(t.vProd),
		"ICMSTot": map[string]any{"vBC": money(t.vBC), "vICMS": money(t.vICMS), "vICMSDeson": money(zero), "vFCP": money(zero)},
		"vCOFINS": money(t.vCOFINS), "vPIS": money(t.vPIS), "vFUNTTEL": money(zero), "vFUST": money(zero),
		"vRetTribTot": map[string]any{"vRetPIS": money(zero), "vRetCofins": money(zero), "vRetCSLL": money(zero), "vIRRF": money(zero)},
		"vDesc":       money(t.vDesc), "vOutro": money(t.vOutro),
		// vNF = vProd - retenções (zero na v1); o vProd do item já considera desconto e outras despesas (MOC G137).
		"vNF": money(t.vProd),
	}
}

func (p buildParams) buildItems() ([]any, totals) {
	var t totals
	out := make([]any, 0, len(p.req.Items))
	for i, it := range p.req.Items {
		// vProd usa os valores como saem no XML (qFaturada com até 4 casas, vItem com até 8), senão a conta do item e a
		// que a SEFAZ refaz sobre o documento divergem em 1 centavo.
		qty, unit := parseDec(it.Quantity).Round(4), parseDec(it.UnitValue).Round(8)
		desc, other := parseOpt(it.Discount), parseOpt(it.OtherCharges)
		vProd := qty.Mul(unit).Sub(desc).Add(other).Round(moneyScale)
		prod := map[string]any{
			"cProd": it.Code, "xProd": it.Description, "cClass": it.ClassCode, "uMed": it.Unit,
			"qFaturada": decStr(qty, 2, 4), "vItem": decStr(unit, 2, 8), "vProd": money(vProd),
		}
		if it.ICMS.CST != icmsSemCST {
			setIfPtr(prod, "CFOP", it.CFOP)
		}
		if desc.Sign() != 0 {
			prod["vDesc"] = money(desc)
		}
		if other.Sign() != 0 {
			prod["vOutro"] = money(other)
		}
		imp, icms := p.buildTax(it, vProd)
		out = append(out, map[string]any{"@nItem": strconv.Itoa(i + 1), "prod": prod, "imposto": imp})
		t.vProd = t.vProd.Add(vProd)
		t.vDesc = t.vDesc.Add(desc)
		t.vOutro = t.vOutro.Add(other)
		t.vBC = t.vBC.Add(icms.base)
		t.vICMS = t.vICMS.Add(icms.value)
		t.vPIS = t.vPIS.Add(icms.pis)
		t.vCOFINS = t.vCOFINS.Add(icms.cofins)
	}
	return out, t
}

type itemTax struct{ base, value, pis, cofins decimal.Decimal }

// buildTax monta o grupo imposto do item e devolve o que entra nos totais. Base de ICMS, PIS e COFINS = valor do item.
func (p buildParams) buildTax(it NfcomItem, vProd decimal.Decimal) (map[string]any, itemTax) {
	var tax itemTax
	imp := map[string]any{}
	switch it.ICMS.CST {
	case "00", "90":
		rate := parseOpt(it.ICMS.Rate)
		tax.base = vProd
		tax.value = vProd.Mul(rate).Div(decimal.NewFromInt(100)).Round(moneyScale)
		imp["ICMS"+it.ICMS.CST] = map[string]any{
			"CST": it.ICMS.CST, "vBC": money(tax.base), "pICMS": rateStr(rate), "vICMS": money(tax.value),
		}
	case "40", "41":
		imp["ICMS40"] = map[string]any{"CST": it.ICMS.CST}
	case "51":
		imp["ICMS51"] = map[string]any{"CST": it.ICMS.CST}
	case icmsSN:
		imp["ICMSSN"] = map[string]any{"CST": icmsCSTSimples, "indSN": indSN}
	case icmsSemCST:
		imp["indSemCST"] = indSemCST
	}
	if it.PIS != nil {
		rate := parseDec(it.PIS.Rate)
		tax.pis = vProd.Mul(rate).Div(decimal.NewFromInt(100)).Round(moneyScale)
		imp["PIS"] = map[string]any{"CST": it.PIS.CST, "vBC": money(vProd), "pPIS": rateStr(rate), "vPIS": money(tax.pis)}
	}
	if it.COFINS != nil {
		rate := parseDec(it.COFINS.Rate)
		tax.cofins = vProd.Mul(rate).Div(decimal.NewFromInt(100)).Round(moneyScale)
		imp["COFINS"] = map[string]any{"CST": it.COFINS.CST, "vBC": money(vProd), "pCOFINS": rateStr(rate), "vCOFINS": money(tax.cofins)}
	}
	return imp, tax
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

// decStr formata d com no mínimo minPlaces e no máximo maxPlaces casas (o leiaute pede de 2 a 4, ou de 2 a 8).
func decStr(d decimal.Decimal, minPlaces, maxPlaces int32) string {
	d = d.Round(maxPlaces)
	places := -d.Exponent()
	if places < minPlaces {
		places = minPlaces
	}
	return d.StringFixed(places)
}

// rateStr formata uma alíquota em %: inteira ("18") ou com 2 a 4 casas ("0.65", "1.6500" vira "1.65").
func rateStr(d decimal.Decimal) string {
	d = d.Round(4)
	if d.IsInteger() {
		return d.StringFixed(0)
	}
	return decStr(d, 2, 4)
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

// docAlnum normaliza CNPJ sem perder a letra do CNPJ alfanumérico (IN RFB 2229/2024): caixa alta, só letras e dígitos.
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
