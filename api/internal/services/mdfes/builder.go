package mdfes

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	mdfeVersao = "3.00"
	mdfeXMLNS  = "http://www.portalfiscal.inf.br/mdfe"

	procEmiOwn   = "0"                   // emissão com aplicativo do contribuinte
	tpEmisNormal = services.TpEmisNormal // forma de emissão: normal
	// TpEmisContingencia / TpEmisRegEspNFF completam a enumeração de ide/tpEmis
	// do MDF-e (mdfeTiposBasico_v3.00). O MDF-e em contingência é autorizado
	// depois — ver o plano de contingência, fase C6.
	TpEmisContingencia = "2"
	TpEmisRegEspNFF    = "3"

	// tpEmit (TEmit). MVP issues on behalf of an NF-e emitter hauling its own
	// cargo, so tpEmit is "2" (Carga Própria). 1=Prestador de serviço, 3=CT-e
	// Globalizado are not yet supported.
	tpEmitCargaPropria = "2"

	// tpTransp (TTransp) — só é informado quando há grupo prop (proprietário de
	// terceiro). 1=ETC, 2=TAC, 3=CTC.
	tpTranspETC = "1"
	tpTranspTAC = "2"
	tpTranspCTC = "3"

	// tpProp (veicTracao/prop): 0=TAC Agregado, 1=TAC Independente, 2=Outros.
	tpPropOutros = "2"
)

// buildParams carries everything BuildMDFe needs.
type buildParams struct {
	org         map[string]types.AttributeValue
	orgPK       string
	accessKey   string
	serie       int
	number      int
	environment int
	now         time.Time
	modal       string // ModalRodoviario | ModalAereo | ModalAquaviario | ModalFerroviario
	cargo       *resolvedCargo
	vehicle     resolvedVehicle
	trailers    []resolvedVehicle // veicReboque — up to 3
	owner       *resolvedOwner    // third-party traction-vehicle owner (veicTracao/prop)
	drivers     []MdfeDriver
	route       []string
	bulkCargo   *MdfeBulkCargo
	tripStart   *string
	rntrc       *string
	ciot        *string
	addInfo     *string
	air         *MdfeAirModal   // modal aéreo data (API-supplied)
	water       *MdfeWaterModal // modal aquaviário data (API-supplied)
	rail        *MdfeRailModal  // modal ferroviário data (API-supplied)
	// tpEmis é a forma de emissão (1 normal, 2 contingência, 3 Regime Especial
	// NFF). Ao contrário da NF-e, o layout do MDF-e não tem dhCont/xJust.
	tpEmis string
	// tolls são os vales-pedágio da viagem, já cruzados com o cadastro.
	tolls []resolvedToll
	tech  TechData
	// csrtID/csrt são o Código de Segurança do Responsável Técnico. Só o hash
	// derivado entra no XML.
	csrtID string
	csrt   string
}

// BuildMDFe constructs the MDFe dict sent to the py-dfe Lambda. SEFAZ no longer
// accepts the <enviMDFe> batch wrapper for synchronous authorization
// (MDFeRecepcaoSinc): the <MDFe> document is the root node. Element ordering is
// handled downstream by py-dfe's XSD_ORDER table.
func BuildMDFe(p buildParams) map[string]any {
	orgPerson := personMap(p.org)
	cUF := p.accessKey[0:2]
	cMDF := p.accessKey[35:43]
	cDV := string(p.accessKey[43])

	infMDFe := map[string]any{
		"@versao":  mdfeVersao,
		"@Id":      "MDFe" + p.accessKey,
		"ide":      p.buildIde(cUF, cMDF, cDV),
		"emit":     buildEmit(p.org, orgPerson, p.orgPK),
		"infModal": p.buildInfModal(),
		"infDoc":   p.buildInfDoc(),
		"prodPred": p.buildProdPred(),
		"tot":      p.buildTot(),
	}
	if p.addInfo != nil && *p.addInfo != "" {
		infMDFe["infAdic"] = map[string]any{"infCpl": *p.addInfo}
	}
	if rt := buildRespTec(p.tech, p.csrtID, p.csrt, p.accessKey); rt != nil {
		infMDFe["infRespTec"] = rt
	}

	infMDFeSupl := map[string]any{
		"qrCodMDFe": fmt.Sprintf("https://dfe-portal.svrs.rs.gov.br/mdfe/qrCode?chMDFe=%s&tpAmb=%d", p.accessKey, p.environment),
	}

	return map[string]any{
		"MDFe": map[string]any{
			"@xmlns":      mdfeXMLNS,
			"infMDFe":     infMDFe,
			"infMDFeSupl": infMDFeSupl,
		},
	}
}

func (p buildParams) buildIde(cUF, cMDF, cDV string) map[string]any {
	ide := map[string]any{
		"cUF":     cUF,
		"tpAmb":   fmt.Sprintf("%d", p.environment),
		"tpEmit":  tpEmitCargaPropria,
		"mod":     services.ModelMDFe,
		"serie":   fmt.Sprintf("%d", p.serie),
		"nMDF":    fmt.Sprintf("%d", p.number),
		"cMDF":    cMDF,
		"cDV":     cDV,
		"modal":   modalCode(p.modal),
		"dhEmi":   p.now.Format(layoutDateTimeTZ),
		"tpEmis":  p.tpEmis,
		"procEmi": procEmiOwn,
		"verProc": verProc(p.tech),
		"UFIni":   p.cargo.ufIni,
		"UFFim":   p.cargo.ufFim,
	}

	// SEFAZ rule F25 (cStat 745): tpTransp may only be present when the traction
	// vehicle has a third-party owner (grupo prop). For carga própria (own
	// vehicle, no prop) the field MUST be omitted.
	if tp := tpTranspFor(p.owner); tp != "" {
		ide["tpTransp"] = tp
	}

	munCarrega := make([]map[string]any, 0, len(p.cargo.carrega))
	for _, m := range p.cargo.carrega {
		munCarrega = append(munCarrega, map[string]any{
			"cMunCarrega": m.IBGECode,
			"xMunCarrega": m.City,
		})
	}
	ide["infMunCarrega"] = munCarrega

	if len(p.route) > 0 {
		perc := make([]map[string]any, 0, len(p.route))
		for _, uf := range p.route {
			perc = append(perc, map[string]any{"UFPer": uf})
		}
		ide["infPercurso"] = perc
	}
	if p.tripStart != nil && *p.tripStart != "" {
		ide["dhIniViagem"] = *p.tripStart
	}
	return ide
}

// buildInfModal dispatches to the modal-specific builder. Rodoviário is fully
// implemented from internal vehicle/owner data; the remaining modals are built
// straight from the API-supplied payload following their XSDs.
func (p buildParams) buildInfModal() map[string]any {
	modal := map[string]any{"@versaoModal": mdfeVersao}
	switch p.modal {
	case ModalAereo:
		modal["aereo"] = buildAereo(p.air)
	case ModalAquaviario:
		modal["aquav"] = buildAquav(p.water)
	case ModalFerroviario:
		modal["ferrov"] = buildFerrov(p.rail)
	default:
		modal["rodo"] = p.buildRodo()
	}
	return modal
}

// buildRodo builds the rodoviário modal node (infANTT + veicTracao + veicReboque).
func (p buildParams) buildRodo() map[string]any {
	veic := map[string]any{
		"placa": p.vehicle.Placa,
		"tara":  p.vehicle.Tara,
		"tpRod": p.vehicle.TpRod,
		"tpCar": p.vehicle.TpCar,
		"UF":    p.vehicle.UF,
	}
	if p.vehicle.CInt != "" {
		veic["cInt"] = p.vehicle.CInt
	}
	if p.vehicle.RENAVAM != "" {
		veic["RENAVAM"] = p.vehicle.RENAVAM
	}
	if p.vehicle.CapKG != "" {
		veic["capKG"] = p.vehicle.CapKG
	}
	if p.vehicle.CapM3 != "" {
		veic["capM3"] = p.vehicle.CapM3
	}
	if prop := buildProp(p.owner); prop != nil {
		veic["prop"] = prop
	}

	condutores := make([]map[string]any, 0, len(p.drivers))
	for _, c := range p.drivers {
		condutores = append(condutores, map[string]any{"xNome": c.Name, "CPF": onlyDigits(c.CPF)})
	}
	veic["condutor"] = condutores

	rodo := map[string]any{"veicTracao": veic}
	if len(p.trailers) > 0 {
		reboques := make([]map[string]any, 0, len(p.trailers))
		for _, t := range p.trailers {
			reboque := map[string]any{"placa": t.Placa, "tara": t.Tara, "tpCar": t.TpCar}
			if t.CInt != "" {
				reboque["cInt"] = t.CInt
			}
			if t.RENAVAM != "" {
				reboque["RENAVAM"] = t.RENAVAM
			}
			if t.CapKG != "" {
				reboque["capKG"] = t.CapKG
			}
			if t.CapM3 != "" {
				reboque["capM3"] = t.CapM3
			}
			if t.UF != "" {
				reboque["UF"] = t.UF
			}
			reboques = append(reboques, reboque)
		}
		rodo["veicReboque"] = reboques
	}
	if infANTT := p.buildInfANTT(); len(infANTT) > 0 {
		rodo["infANTT"] = infANTT
	}
	return rodo
}

// resolveRNTRC picks the request override, then the registered-vehicle owner RNTRC.
func (p buildParams) resolveRNTRC() string {
	if p.rntrc != nil && *p.rntrc != "" {
		return *p.rntrc
	}
	return p.vehicle.RNTRC
}

// buildProp builds the veicTracao/prop group for a third-party owner, or nil when
// the vehicle belongs to the MDF-e emitter (carga própria — no prop group).
// Element order: choice{CPF|CNPJ}, RNTRC, xNome, optional{IE,UF}, tpProp.
func buildProp(o *resolvedOwner) map[string]any {
	if o == nil {
		return nil
	}
	prop := map[string]any{}
	if o.CPF != "" {
		prop["CPF"] = onlyDigits(o.CPF)
	} else {
		prop["CNPJ"] = onlyDigits(o.CNPJ)
	}
	prop["RNTRC"] = o.RNTRC
	prop["xNome"] = o.Name
	if o.IE != "" && o.UF != "" {
		prop["IE"] = o.IE
		prop["UF"] = o.UF
	}
	prop["tpProp"] = strOrDefault(o.TpProp, tpPropOutros)
	return prop
}

// tpTranspFor derives ide/tpTransp from the third-party owner, per SEFAZ rules
// F18 (cStat 743) and F19 (cStat 744): a CPF owner ⇒ TAC; a CNPJ owner ⇒ ETC,
// or CTC when the caller explicitly requests it. Returns "" when there is no
// owner (rule F25: tpTransp must then be absent).
func tpTranspFor(o *resolvedOwner) string {
	if o == nil {
		return ""
	}
	if o.CPF != "" {
		return tpTranspTAC
	}
	if o.TpTransp == tpTranspCTC {
		return tpTranspCTC
	}
	return tpTranspETC
}

func (p buildParams) buildProdPred() map[string]any {
	pred := map[string]any{
		"tpCarga": p.cargo.prodPred.TpCarga,
		"xProd":   p.cargo.prodPred.XProd,
	}
	if p.cargo.prodPred.NCM != "" {
		pred["NCM"] = p.cargo.prodPred.NCM
	}
	if p.bulkCargo != nil {
		carr := map[string]any{"CEP": onlyDigits(p.bulkCargo.CEPLoading)}
		setIfPtr(carr, "latitude", p.bulkCargo.LatLoading)
		setIfPtr(carr, "longitude", p.bulkCargo.LonLoading)
		desc := map[string]any{"CEP": onlyDigits(p.bulkCargo.CEPUnloading)}
		setIfPtr(desc, "latitude", p.bulkCargo.LatUnloading)
		setIfPtr(desc, "longitude", p.bulkCargo.LonUnloading)
		pred["infLotacao"] = map[string]any{
			"infLocalCarrega":    carr,
			"infLocalDescarrega": desc,
		}
	}
	return pred
}

func (p buildParams) buildTot() map[string]any {
	tot := map[string]any{
		"vCarga": p.cargo.totalValue.StringFixed(2),
		"cUnid":  cUnidKG,
		"qCarga": p.cargo.totalWeight.StringFixed(4),
	}
	n := fmt.Sprintf("%d", len(p.cargo.docs))
	if len(p.cargo.docs) > 0 && p.cargo.docs[0].docType == docTypeCTe {
		tot["qCTe"] = n
	} else {
		tot["qNFe"] = n
	}
	return tot
}

func (p buildParams) firstCondutorCPF() string {
	if len(p.drivers) > 0 {
		return p.drivers[0].CPF
	}
	return ""
}

// modalCode maps a modal name to its ide/modal code (1..4), defaulting to
// rodoviário for an unknown/empty value.
func modalCode(modal string) string {
	if code, ok := modalCodes[modal]; ok {
		return code
	}
	return modalCodeRodoviario
}

// --- emit / shared helpers ---

func buildEmit(org map[string]types.AttributeValue, orgPerson map[string]any, orgPK string) map[string]any {
	emit := map[string]any{
		"xNome":     strAttr(org, "name"),
		"enderEmit": buildEnderMDFe(orgPerson),
	}
	if fant := anyStr(orgPerson, "fantasy_name"); fant != "" {
		emit["xFant"] = fant
	}
	doc := services.StripPKPrefix(orgPK)
	if strings.HasPrefix(orgPK, "CNPJ_") {
		emit["CNPJ"] = doc
	} else {
		emit["CPF"] = doc
	}
	if ie := firstIE(orgPerson); ie != "" {
		emit["IE"] = ie
	}
	return emit
}

func buildEnderMDFe(person map[string]any) map[string]any {
	addr := services.FirstAddress(person)
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
	if email := services.FirstEmail(person); email != "" {
		ender["email"] = email
	}
	if fone := services.FirstPhone(person); fone != "" {
		ender["fone"] = fone
	}
	return ender
}

// buildRespTec delega ao nó compartilhado: infRespTec é literalmente o mesmo
// grupo na NF-e, no CT-e e no MDF-e (ver a tabela de ordem XSD).
func buildRespTec(t TechData, csrtID, csrt, accessKey string) map[string]any {
	if t.CNPJ == "" {
		return nil
	}
	return services.BuildRespTec(t.CNPJ, t.Name, t.Email, onlyDigits(t.Phone), csrtID, csrt, accessKey)
}

func verProc(t TechData) string {
	if t.Version != "" {
		return t.Version
	}
	return "ctech-dfe"
}

func sefazBatchID() string {
	return fmt.Sprintf("%d", rand.Int63n(999_999_999_999_999)+1)
}

// --- DynamoDB attribute helpers ---

// personMap unmarshals an org/person DynamoDB item and returns its "person" map.
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

func firstIE(person map[string]any) string {
	if regs, ok := person["state_registrations"].([]any); ok && len(regs) > 0 {
		if rm, ok := regs[0].(map[string]any); ok {
			if v, ok := rm["state_registration"].(string); ok {
				return v
			}
		}
	}
	return ""
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

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
