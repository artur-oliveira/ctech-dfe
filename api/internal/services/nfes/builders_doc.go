package nfes

// builders_doc.go ports app/services/nfes/_builders_doc.py.
// Produces the enviNFe dict structure that py-dfe Lambda signs and sends to SEFAZ.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/services"
	"github.com/shopspring/decimal"
)

const (
	nfModel55    = "55"
	nfModel65    = "65"
	tpEmisNormal = "1"

	tpImpDANFERetrato = "1"
	tpImpDANFENFCe    = "4"

	procEmiApp          = "0"
	indTotCompoe        = "1"
	indIEDestNaoContrib = "9"
	indIEDestContrib    = "1"
	modFreteSemFrete    = "9"
	// Own-transport modFrete codes: the carrier (transporta) IS the issuer or the
	// recipient, so its data is copied from emit/dest instead of supplied separately.
	modFreteProprioRemetente    = "3" // Transporte próprio por conta do remetente (emitente)
	modFreteProprioDestinatario = "4" // Transporte próprio por conta do destinatário
	qVolPadrao          = "1"
	cPaisBrasil         = "1058"
	xPaisBrasil         = "Brasil"
	indSinc             = "1"
	natOpVenda          = "Venda de Mercadoria"
	natOpMaxLen         = 60 // SEFAZ ide.natOp limit (xNatOp: 1-60 chars)
	homNameReceiver     = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
	homProduct          = "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
)

// TechData holds the technical issuer info included in infRespTec.
type TechData struct {
	CNPJ    string
	Name    string
	Email   string
	Phone   string
	Version string
}

// First-address / first-phone / first-email lookups are shared across every DFe
// builder and live in the services package (services.FirstAddress, etc.).

// getIEForUF returns the IE for the given UF from state_registrations list.
func getIEForUF(person map[string]any, uf string) string {
	if regs, ok := person["state_registrations"].([]any); ok && len(regs) > 0 {
		for _, r := range regs {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if rm["uf"] == uf {
				if v, ok := rm["state_registration"].(string); ok && v != "" {
					return v
				}
				if v, ok := rm["ie"].(string); ok && v != "" {
					return v
				}
			}
		}
		// fallback: first entry
		first := regs[0].(map[string]any)
		if v, ok := first["state_registration"].(string); ok && v != "" {
			return v
		}
		if v, ok := first["ie"].(string); ok && v != "" {
			return v
		}
	}
	if v, ok := person["state_registration"].(string); ok {
		return v
	}
	return ""
}

func anyStr(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func anyStrPtr(m map[string]any, key string) *string {
	if v, ok := m[key].(string); ok && v != "" {
		return &v
	}
	return nil
}

// buildTransp builds the transp XML node.
//
// For own-transport freight modes the transportador is the issuer or the
// recipient itself, so its transporta data is taken from emitTransporta /
// destTransporta instead of the request-supplied transporta_* fields:
//   - modFrete "3" (próprio por conta do remetente)    → emitTransporta
//   - modFrete "4" (próprio por conta do destinatário)  → destTransporta
func buildTransp(hasPesoL, hasPesoB bool, totalPesoL, totalPesoB decimal.Decimal, transport, emitTransporta, destTransporta map[string]any) map[string]any {
	modFrete := modFreteSemFrete
	if transport != nil {
		if v := anyStr(transport, "mod_frete", ""); v != "" {
			modFrete = v
		}
	}
	transp := map[string]any{"modFrete": modFrete}

	var transporta map[string]any
	switch modFrete {
	case modFreteProprioRemetente:
		transporta = emitTransporta
	case modFreteProprioDestinatario:
		transporta = destTransporta
	default:
		transporta = transportaFromRequest(transport)
	}
	if len(transporta) > 0 {
		transp["transporta"] = transporta
	}

	if transport != nil {
		veiculo := map[string]any{}
		if v := anyStr(transport, "veiculo_placa", ""); v != "" {
			veiculo["placa"] = v
		}
		if v := anyStr(transport, "veiculo_uf", ""); v != "" {
			veiculo["UF"] = v
		}
		if v := anyStr(transport, "veiculo_rntrc", ""); v != "" {
			veiculo["RNTRC"] = v
		}
		if len(veiculo) > 0 {
			transp["veicTransp"] = veiculo
		}
	}

	if hasPesoL || hasPesoB {
		vol := map[string]any{"qVol": qVolPadrao}
		if hasPesoL {
			vol["pesoL"] = totalPesoL.StringFixed(3)
		}
		if hasPesoB {
			vol["pesoB"] = totalPesoB.StringFixed(3)
		}
		transp["vol"] = vol
	}
	return transp
}

// transportaFromRequest builds the transporta node from the request-supplied
// transporta_* fields (used for non-own-transport freight modes).
func transportaFromRequest(transport map[string]any) map[string]any {
	if transport == nil {
		return nil
	}
	transporta := map[string]any{}
	if v := anyStr(transport, "transporta_cnpj", ""); v != "" {
		transporta["CNPJ"] = v
	} else if v := anyStr(transport, "transporta_cpf", ""); v != "" {
		transporta["CPF"] = v
	}
	if v := anyStr(transport, "transporta_nome", ""); v != "" {
		transporta["xNome"] = v
	}
	if v := anyStr(transport, "transporta_ie", ""); v != "" {
		transporta["IE"] = v
	}
	if v := anyStr(transport, "transporta_ender", ""); v != "" {
		transporta["xEnder"] = v
	}
	if v := anyStr(transport, "transporta_mun", ""); v != "" {
		transporta["xMun"] = v
	}
	if v := anyStr(transport, "transporta_uf", ""); v != "" {
		transporta["UF"] = v
	}
	return transporta
}

// buildPartyTransporta builds a transporta node from a party (emitente or
// destinatário) for own-transport freight modes (modFrete 3/4). Address fields
// are sourced from the party's first address.
func buildPartyTransporta(doc string, isPJ bool, name, ie string, address map[string]any) map[string]any {
	if doc == "" {
		return nil
	}
	transporta := map[string]any{}
	if isPJ {
		transporta["CNPJ"] = doc
	} else {
		transporta["CPF"] = doc
	}
	if name != "" {
		transporta["xNome"] = name
	}
	if ie != "" {
		transporta["IE"] = ie
	}
	if v := anyStr(address, "street", ""); v != "" {
		transporta["xEnder"] = v
	}
	if v := anyStr(address, "city", ""); v != "" {
		transporta["xMun"] = v
	}
	if v := anyStr(address, "state_federation", ""); v != "" {
		transporta["UF"] = v
	}
	return transporta
}

// buildPag builds the pag XML node.
func buildPag(payments []map[string]any, vTroco *string) map[string]any {
	detPag := make([]map[string]any, 0, len(payments))
	for _, p := range payments {
		item := map[string]any{}
		if v := anyStr(p, "ind_pag", ""); v != "" {
			item["indPag"] = v
		}
		item["tPag"] = anyStr(p, "payment_type", "")
		item["vPag"] = q2(d(anyStr(p, "value", "0")).RoundBank(2))
		if v := anyStr(p, "d_pag", ""); v != "" {
			item["dPag"] = v
		}
		if cardRaw, ok := p["card"].(map[string]any); ok && cardRaw != nil {
			cardNode := map[string]any{"tpIntegra": anyStr(cardRaw, "tp_integra", "")}
			if v := anyStr(cardRaw, "cnpj", ""); v != "" {
				cardNode["CNPJ"] = v
			}
			if v := anyStr(cardRaw, "t_band", ""); v != "" {
				cardNode["tBand"] = v
			}
			if v := anyStr(cardRaw, "c_aut", ""); v != "" {
				cardNode["cAut"] = v
			}
			item["card"] = cardNode
		}
		detPag = append(detPag, item)
	}
	pag := map[string]any{"detPag": detPag}
	if vTroco != nil && *vTroco != "" {
		vTd := d(*vTroco)
		if vTd.GreaterThan(decimal.Zero) {
			pag["vTroco"] = q2(vTd.RoundBank(2))
		}
	}
	return pag
}

// buildCobr builds the cobr XML node.
func buildCobr(fat map[string]any, duplicatas []map[string]any) map[string]any {
	cobr := map[string]any{}
	if fat != nil {
		fatNode := map[string]any{}
		if v := anyStr(fat, "n_fat", ""); v != "" {
			fatNode["nFat"] = v
		}
		if v := anyStr(fat, "v_orig", ""); v != "" {
			fatNode["vOrig"] = v
		}
		if v := anyStr(fat, "v_desc", ""); v != "" {
			fatNode["vDesc"] = v
		}
		if v := anyStr(fat, "v_liq", ""); v != "" {
			fatNode["vLiq"] = v
		}
		cobr["fat"] = fatNode
	}
	if len(duplicatas) > 0 {
		dups := make([]map[string]any, 0, len(duplicatas))
		for _, dup := range duplicatas {
			item := map[string]any{}
			if v := anyStr(dup, "n_dup", ""); v != "" {
				item["nDup"] = v
			}
			if v := anyStr(dup, "d_venc", ""); v != "" {
				item["dVenc"] = v
			}
			item["vDup"] = anyStr(dup, "v_dup", "0.00")
			dups = append(dups, item)
		}
		cobr["dup"] = dups
	}
	return cobr
}

func buildEnder(person map[string]any) map[string]string {
	address := services.FirstAddress(person)
	uf := anyStr(address, "state_federation", "")
	ender := map[string]string{
		"xLgr":    anyStr(address, "street", ""),
		"nro":     strOrDefault(anyStr(address, "number", ""), "S/N"),
		"xBairro": anyStr(address, "neighborhood", ""),
		"cMun":    strOrDefault(anyStr(address, "city_ibge_code", ""), "0000000"),
		"xMun":    anyStr(address, "city", ""),
		"UF":      uf,
		"CEP":     strings.ReplaceAll(anyStr(address, "postal_code", ""), "-", ""),
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
	}
	if phone := services.FirstPhone(person); phone != "" {
		ender["fone"] = phone
	}
	return ender
}

// buildLocal builds a TLocal-shaped map (local de retirada/entrega) — same
// field set for both, per xsd_order.py's "retirada"/"entrega" ordering.
// Unlike buildEnder (TEndereco), TLocal has no CEP.
func buildLocal(l *NfeLocalBody) map[string]any {
	if l == nil {
		return nil
	}
	m := map[string]any{
		"xLgr":    l.XLgr,
		"nro":     l.Nro,
		"xBairro": l.XBairro,
		"cMun":    l.CMun,
		"xMun":    l.XMun,
		"UF":      l.UF,
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
	}
	if l.CNPJ != nil && *l.CNPJ != "" {
		m["CNPJ"] = *l.CNPJ
	}
	if l.CPF != nil && *l.CPF != "" {
		m["CPF"] = *l.CPF
	}
	if l.XNome != nil && *l.XNome != "" {
		m["xNome"] = *l.XNome
	}
	if l.XCpl != nil && *l.XCpl != "" {
		m["xCpl"] = *l.XCpl
	}
	if l.Fone != nil && *l.Fone != "" {
		m["fone"] = *l.Fone
	}
	if l.Email != nil && *l.Email != "" {
		m["email"] = *l.Email
	}
	return m
}

// BuildEnviNFe constructs the enviNFe dict structure for py-dfe Lambda.
// Mirrors Python _build_envi_nfe exactly.
func BuildEnviNFe(
	org, receiver map[string]any,
	orgPK string,
	productItems []map[string]any,
	payments []map[string]any,
	number, serie, environment int,
	accessKey string,
	totalProducts, totalDiscount decimal.Decimal,
	additionalInfo *string,
	now time.Time,
	natOp *string,
	finNFe, indFinal, indPres, tpNF string,
	transport map[string]any,
	cobrFat map[string]any,
	cobrDuplicatas []map[string]any,
	vTroco *string,
	tech TechData,
	model string,
	supl map[string]any,
	retirada, entrega *NfeLocalBody,
) map[string]any {
	isNFCe := model == nfModel65
	orgPerson := getPersonMap(org)
	orgAddress := services.FirstAddress(orgPerson)
	orgCRT := getAnyInt(orgPerson, "crt", 1)
	emitUF := anyStr(orgAddress, "state_federation", "")
	cUF := services.UFCode[emitUF]
	if cUF == "" {
		cUF = "35"
	}

	emitDoc := services.StripPKPrefix(orgPK)
	isEmitPJ := strings.HasPrefix(orgPK, "CNPJ_")

	receiverSK := anyStr(receiver, "sk", "")
	destDoc := services.StripPKPrefix(receiverSK)
	isDestPJ := strings.HasPrefix(receiverSK, "CNPJ_")
	destPerson := getPersonMap(receiver)
	destAddress := services.FirstAddress(destPerson)
	destUF := anyStr(destAddress, "state_federation", "")

	idDest := "1"
	if emitUF != destUF {
		idDest = "2"
	}
	if isNFCe {
		// NFC-e is restricted to internal operations (operação interna).
		idDest = "1"
	}

	dhEmi := fmtDhEmi(now)
	cNF := ""
	if len(accessKey) >= 44 {
		cNF = accessKey[35:43]
	}

	destIE := getIEForUF(destPerson, destUF)
	isDifalEligible := idDest == "2" && destIE == "" && orgCRT == 3

	// ── Per-item tax totals ───────────────────────────────────────────────────
	var (
		totalIBSBC        = decimal.Zero
		totalIBSUF        = decimal.Zero
		totalIBSMun       = decimal.Zero
		totalIBS          = decimal.Zero
		totalCBSBC        = decimal.Zero
		totalCBS          = decimal.Zero
		totalPesoL        = decimal.Zero
		totalPesoB        = decimal.Zero
		hasPesoL          = false
		hasPesoB          = false
		totalVBC          = decimal.Zero
		totalVICMS        = decimal.Zero
		totalVICMSDeson   = decimal.Zero
		totalVFCP         = decimal.Zero
		totalVBCST        = decimal.Zero
		totalVICMSST      = decimal.Zero
		totalVFCPST       = decimal.Zero
		totalVPIS         = decimal.Zero
		totalVCOFINS      = decimal.Zero
		totalVIPI         = decimal.Zero
		totalVFrete       = decimal.Zero
		totalVSeg         = decimal.Zero
		totalVOutro       = decimal.Zero
		totalVICMSUFDest  = decimal.Zero
		totalVFCPUFDest   = decimal.Zero
		totalVServ        = decimal.Zero
		totalVBCISSQN     = decimal.Zero
		totalVISSQN       = decimal.Zero
		totalVPISISSQN    = decimal.Zero
		totalVCOFINSISSQN = decimal.Zero
		hasISSQN          = false
	)

	det := make([]map[string]any, 0, len(productItems))

	for i, item := range productItems {
		cfopEntries := getCFOPConfig(item)
		cfopEntry := findCFOPEntry(cfopEntries, anyStr(item, "cfop", ""))
		origin := anyStr(item, "origin", "0")

		originPtr := &origin
		pICMSResolved := resolveICMSAliq(emitUF, destUF, anyStrPtr(item, "icms_aliq_override"))
		pFCPResolved := resolveFCPAliq(destUF, anyStrPtr(item, "fcp_aliq_override"))

		qty := d(anyStr(item, "quantity", "0"))
		unitVal := d(anyStr(item, "unit_value", "0"))
		disc := d(anyStr(item, "discount", "0"))
		vFretItem := d(anyStr(item, "v_frete", "0"))
		vSegItem := d(anyStr(item, "v_seg", "0"))
		vOutroItem := d(anyStr(item, "v_outro", "0"))
		vProdDec := qty.Mul(unitVal).RoundBank(2)
		vBCIBSCBS := vProdDec.Sub(disc).RoundBank(2)
		vProd := q2(vProdDec)

		totalVFrete = totalVFrete.Add(vFretItem)
		totalVSeg = totalVSeg.Add(vSegItem)
		totalVOutro = totalVOutro.Add(vOutroItem)

		isISSQNItem := cfgStr(cfopEntry, "issqn_ind_iss", "") != ""

		var issqnNode map[string]any
		var icmsForImposto map[string]any
		var icmsUFDestNode map[string]any

		if isISSQNItem {
			issqnNode = buildISSQN(vBCIBSCBS, cfopEntry)
			if issqnNode != nil {
				hasISSQN = true
				issqnInner, _ := issqnNode["ISSQN"].(map[string]any)
				totalVServ = totalVServ.Add(vProdDec)
				totalVBCISSQN = totalVBCISSQN.Add(vBCIBSCBS)
				totalVISSQN = totalVISSQN.Add(d(anyStr(issqnInner, "vISSQN", "0")))
			}
		} else {
			cstForItem := cfgStr(cfopEntry, "icms", "40")
			var icms map[string]any
			if orgCRT == 1 || orgCRT == 2 || orgCRT == 4 {
				csosn := cfgStr(cfopEntry, "csosn", "102")
				icms = buildICMSSN(origin, csosn, vBCIBSCBS, cfopEntry)
			} else {
				cfopEntry = applyUFRules(emitUF, cstForItem, cfopEntry, pICMSResolved)
				icms = buildICMSNormal(origin, cstForItem, vBCIBSCBS, cfopEntry, pICMSResolved, pFCPResolved, qty)
			}
			icmsForImposto = icms
			icmsInner := firstValue(icms)

			totalVBC = totalVBC.Add(dv(icmsInner, "vBC"))
			totalVICMS = totalVICMS.Add(dv(icmsInner, "vICMS"))
			totalVICMSDeson = totalVICMSDeson.Add(dv(icmsInner, "vICMSDeson"))
			totalVFCP = totalVFCP.Add(dv(icmsInner, "vFCP"))
			totalVBCST = totalVBCST.Add(dv(icmsInner, "vBCST"))
			totalVICMSST = totalVICMSST.Add(dv(icmsInner, "vICMSST"))
			totalVFCPST = totalVFCPST.Add(dv(icmsInner, "vFCPST"))

			icmsUFDestNode = map[string]any{}
			if isDifalEligible && icmsCSTDifalEligible[cstForItem] {
				pICMSUFDest := resolveICMSIntraAliq(destUF)
				pICMSInter := resolveICMSInterAliq(emitUF, destUF, originPtr)
				pFCPUFDest := resolveFCPAliq(destUF, nil)
				icmsUFDestNode = buildICMSUFDest(vBCIBSCBS, pICMSUFDest, pICMSInter, pFCPUFDest)
				totalVICMSUFDest = totalVICMSUFDest.Add(d(anyStr(icmsUFDestNode, "vICMSUFDest", "0")))
				totalVFCPUFDest = totalVFCPUFDest.Add(d(anyStr(icmsUFDestNode, "vFCPUFDest", "0")))
			}
		}

		// Taxable unit / conversion
		unit := anyStr(item, "unit", "UN")
		taxableUnit := anyStr(item, "taxable_unit", unit)
		if taxableUnit == "" {
			taxableUnit = unit
		}
		qTrib := anyStr(item, "quantity", "0")
		vUnTrib := anyStr(item, "unit_value", "0")
		if taxableUnit != unit {
			if factors, ok := item["conversion_factors"].([]any); ok {
				for _, f := range factors {
					if fm, ok := f.(map[string]any); ok {
						fmUnit := fm["origin_unit"].(string)
						tmTarget := fm["target_unit"].(string)

						if fmUnit == unit && tmTarget == taxableUnit {
							var factor decimal.Decimal
							switch v := fm["factor"].(type) {
							case string:
								factor = decimal.RequireFromString(v)

							case float64:
								factor = decimal.NewFromFloat(v)

							case float32:
								factor = decimal.NewFromFloat32(v)

							case int:
								factor = decimal.NewFromInt(int64(v))

							case int64:
								factor = decimal.NewFromInt(v)

							case json.Number:
								factor, _ = decimal.NewFromString(v.String())

							default:
								// tipo não suportado
								continue
							}
							if factor.IsPositive() {
								qTrib = q4(qty.Mul(factor).RoundBank(4))
								vUnTrib = q4(unitVal.Div(factor).RoundBank(4))
							}
						}
					}
				}
			}
		}

		// Weight totals
		if nw := anyStr(item, "net_weight", ""); nw != "" {
			if v, err := decimal.NewFromString(nw); err == nil {
				totalPesoL = totalPesoL.Add(v.Mul(qty))
				hasPesoL = true
			}
		}
		if gw := anyStr(item, "gross_weight", ""); gw != "" {
			if v, err := decimal.NewFromString(gw); err == nil {
				totalPesoB = totalPesoB.Add(v.Mul(qty))
				hasPesoB = true
			}
		}

		// IBS/CBS
		ibsCBSCST := cfgStr(cfopEntry, "ibs_cbs_cst", "400")
		ibsCBSClassTrib := cfgStr(cfopEntry, "ibs_cbs_class_trib", "")
		ibsUFAliq := cfgStr(cfopEntry, "ibs_uf_aliq", "0.0000")
		ibsMunAliq := cfgStr(cfopEntry, "ibs_mun_aliq", "0.0000")
		cbsAliq := cfgStr(cfopEntry, "cbs_aliq", "0.0000")

		gIBSCBS := buildGIBSCBS(ibsCBSCST, ibsCBSClassTrib, vBCIBSCBS, ibsUFAliq, ibsMunAliq, cbsAliq, cfopEntry)

		if !ibsCBSExempt[ibsCBSCST] {
			if inner, ok := gIBSCBS["gIBSCBS"].(map[string]any); ok {
				totalIBSBC = totalIBSBC.Add(vBCIBSCBS)
				gIBSUF, _ := inner["gIBSUF"].(map[string]any)
				gIBSMunMap, _ := inner["gIBSMun"].(map[string]any)
				itemIBSUF := d(anyStr(gIBSUF, "vIBSUF", "0"))
				itemIBSMun := d(anyStr(gIBSMunMap, "vIBSMun", "0"))
				totalIBSUF = totalIBSUF.Add(itemIBSUF)
				totalIBSMun = totalIBSMun.Add(itemIBSMun)
				totalIBS = totalIBS.Add(itemIBSUF).Add(itemIBSMun)
				totalCBSBC = totalCBSBC.Add(vBCIBSCBS)
				gCBSMap, _ := inner["gCBS"].(map[string]any)
				totalCBS = totalCBS.Add(d(anyStr(gCBSMap, "vCBS", "0")))
			}
		}

		var prodDescription string
		if isNFCe && environment == 2 && i == 0 {
			prodDescription = homProduct
		} else {
			prodDescription = anyStr(item, "description", "")
		}

		// prod node
		prod := map[string]any{
			"cProd":    anyStr(item, "product_code", ""),
			"cEAN":     strOrDefault(anyStr(item, "cean", ""), "SEM GTIN"),
			"xProd":    prodDescription,
			"NCM":      anyStr(item, "ncm", ""),
			"CFOP":     anyStr(item, "cfop", ""),
			"uCom":     unit,
			"qCom":     anyStr(item, "quantity", "0"),
			"vUnCom":   anyStr(item, "unit_value", "0"),
			"vProd":    vProd,
			"cEANTrib": strOrDefault(anyStr(item, "cean", ""), "SEM GTIN"),
			"uTrib":    taxableUnit,
			"qTrib":    qTrib,
			"vUnTrib":  vUnTrib,
			"indTot":   strOrDefault(anyStr(item, "ind_tot", ""), indTotCompoe),
		}
		if d(anyStr(item, "discount", "0")).GreaterThan(decimal.Zero) {
			prod["vDesc"] = q2(disc.RoundBank(2))
		}
		if vFretItem.GreaterThan(decimal.Zero) {
			prod["vFrete"] = q2(vFretItem.RoundBank(2))
		}
		if vSegItem.GreaterThan(decimal.Zero) {
			prod["vSeg"] = q2(vSegItem.RoundBank(2))
		}
		if vOutroItem.GreaterThan(decimal.Zero) {
			prod["vOutro"] = q2(vOutroItem.RoundBank(2))
		}
		if cest := anyStr(item, "cest", ""); cest != "" {
			prod["CEST"] = cest
			if v := anyStr(item, "ind_escala", ""); v != "" {
				prod["indEscala"] = v
			}
			if v := anyStr(item, "cnpj_fab", ""); v != "" {
				prod["CNPJFab"] = v
			}
		}
		if v := anyStr(item, "c_benef", ""); v != "" {
			prod["cBenef"] = v
		}
		if v := anyStr(item, "ext_ipi", ""); v != "" {
			prod["EXTIPI"] = v
		}

		if combProd := anyStr(item, "comb_c_prod_anp", ""); combProd != "" {
			combNode := map[string]any{
				"cProdANP": combProd,
				"descANP":  strOrDefault(anyStr(item, "comb_desc_anp", ""), ""),
				"UFCons":   strOrDefault(anyStr(item, "comb_uf_cons", ""), ""),
			}
			for field, xml := range map[string]string{
				"comb_p_glp": "pGLP", "comb_p_gnn": "pGNn", "comb_p_gni": "pGNi",
				"comb_v_part": "vPart", "comb_codif": "CODIF", "comb_p_bio": "pBio",
			} {
				if v := anyStr(item, field, ""); v != "" {
					combNode[xml] = v
				}
			}
			prod["comb"] = combNode
		}

		if medProd := anyStr(item, "med_c_prod_anvisa", ""); medProd != "" {
			medNode := map[string]any{
				"cProdANVISA": medProd,
				"vPMC":        strOrDefault(anyStr(item, "med_v_pmc", ""), "0.00"),
			}
			if v := anyStr(item, "med_x_motivo_isencao", ""); v != "" {
				medNode["xMotivoIsencao"] = v
			}
			prod["med"] = medNode
		}

		if veicChassi := anyStr(item, "veic_chassi", ""); veicChassi != "" {
			veicNode := map[string]any{
				"tpOp":         strOrDefault(anyStr(item, "veic_tp_op", ""), "0"),
				"chassi":       veicChassi,
				"cCor":         strOrDefault(firstNonEmpty(anyStr(item, "veic_c_cor_override", ""), anyStr(item, "veic_c_cor", "")), ""),
				"xCor":         strOrDefault(firstNonEmpty(anyStr(item, "veic_x_cor_override", ""), anyStr(item, "veic_x_cor", "")), ""),
				"pot":          strOrDefault(anyStr(item, "veic_pot", ""), "0"),
				"cilin":        strOrDefault(anyStr(item, "veic_cilin", ""), "0"),
				"pesoL":        strOrDefault(anyStr(item, "net_weight", ""), "0"),
				"pesoB":        strOrDefault(anyStr(item, "gross_weight", ""), "0"),
				"nSerie":       strOrDefault(anyStr(item, "veic_n_serie", ""), ""),
				"tpComb":       strOrDefault(anyStr(item, "veic_tp_comb", ""), "02"),
				"nMotor":       strOrDefault(anyStr(item, "veic_n_motor", ""), ""),
				"CMT":          strOrDefault(anyStr(item, "veic_cmt", ""), "0"),
				"dist":         strOrDefault(anyStr(item, "veic_dist", ""), "0"),
				"anoMod":       strOrDefault(anyStr(item, "veic_ano_mod", ""), ""),
				"anoFab":       strOrDefault(anyStr(item, "veic_ano_fab", ""), ""),
				"tpPint":       strOrDefault(anyStr(item, "veic_tp_pint", ""), "S"),
				"tpVeic":       strOrDefault(anyStr(item, "veic_tp_veic", ""), "06"),
				"espVeic":      strOrDefault(anyStr(item, "veic_esp_veic", ""), "1"),
				"VIN":          strOrDefault(anyStr(item, "veic_vin", ""), "N"),
				"condVeic":     strOrDefault(anyStr(item, "veic_cond_veic", ""), "1"),
				"cMod":         strOrDefault(anyStr(item, "veic_c_mod", ""), "000001"),
				"cCorDENATRAN": strOrDefault(anyStr(item, "veic_c_cor_denatran", ""), "01"),
				"lota":         strOrDefault(anyStr(item, "veic_lota", ""), "5"),
				"tpRest":       strOrDefault(anyStr(item, "veic_tp_rest", ""), "0"),
			}
			prod["veicProd"] = veicNode
		}

		if armas, ok := item["armas"].([]any); ok && len(armas) > 0 {
			armaList := make([]map[string]any, 0, len(armas))
			for _, a := range armas {
				if am, ok := a.(map[string]any); ok {
					armaItem := map[string]any{
						"tpArma": strOrDefault(anyStr(item, "arma_tp_arma", ""), "0"),
						"nSerie": anyStr(am, "n_serie", ""),
						"nCano":  anyStr(am, "n_cano", ""),
						"descr":  firstNonEmpty(anyStr(am, "descr", ""), anyStr(item, "arma_descr", "")),
					}
					armaList = append(armaList, armaItem)
				}
			}
			if len(armaList) > 0 {
				prod["arma"] = armaList
			}
		}

		pisCST := cfgStr(cfopEntry, "pis", "49")
		pisAliq := anyStrPtr(cfopEntry, "pis_aliq")
		pisAliqUnid := anyStrPtr(cfopEntry, "pis_aliq_unid")
		pisBuilt := buildPIS(pisCST, vBCIBSCBS, pisAliq, pisAliqUnid)

		cofinsCST := cfgStr(cfopEntry, "cofins", "49")
		cofinsAliq := anyStrPtr(cfopEntry, "cofins_aliq")
		cofinsAliqUnid := anyStrPtr(cfopEntry, "cofins_aliq_unid")
		cofinsBuilt := buildCOFINS(cofinsCST, vBCIBSCBS, cofinsAliq, cofinsAliqUnid)

		for _, pk := range []string{"PISAliq", "PISOutr", "PISQtde"} {
			if inner, ok := pisBuilt[pk].(map[string]any); ok {
				pisV := d(anyStr(inner, "vPIS", "0"))
				totalVPIS = totalVPIS.Add(pisV)
				if isISSQNItem {
					totalVPISISSQN = totalVPISISSQN.Add(pisV)
				}
				break
			}
		}
		for _, ck := range []string{"COFINSAliq", "COFINSOutr", "COFINSQtde"} {
			if inner, ok := cofinsBuilt[ck].(map[string]any); ok {
				cofinsV := d(anyStr(inner, "vCOFINS", "0"))
				totalVCOFINS = totalVCOFINS.Add(cofinsV)
				if isISSQNItem {
					totalVCOFINSISSQN = totalVCOFINSISSQN.Add(cofinsV)
				}
				break
			}
		}

		var imposto map[string]any
		if isISSQNItem && issqnNode != nil {
			imposto = map[string]any{
				"ISSQN":  issqnNode["ISSQN"],
				"PIS":    pisBuilt,
				"COFINS": cofinsBuilt,
				"IBSCBS": gIBSCBS,
			}
		} else {
			imposto = map[string]any{
				"ICMS":   icmsForImposto,
				"PIS":    pisBuilt,
				"COFINS": cofinsBuilt,
				"IBSCBS": gIBSCBS,
			}

			ipiCST := cfgStr(cfopEntry, "ipi_cst", "")
			ipiAliq := anyStrPtr(cfopEntry, "ipi_aliq")
			ipiNode := buildIPI(ipiCST, vBCIBSCBS, ipiAliq)
			if ipiNode != nil {
				if ipiData, ok := ipiNode["IPI"].(map[string]any); ok {
					if ipiTrib, ok := ipiData["IPITrib"].(map[string]any); ok {
						totalVIPI = totalVIPI.Add(d(anyStr(ipiTrib, "vIPI", "0")))
					}
					imposto["IPI"] = ipiData
				}
			}

			if len(icmsUFDestNode) > 0 {
				imposto["ICMSUFDest"] = icmsUFDestNode
			}
		}

		isCST := cfgStr(cfopEntry, "is_cst", "")
		isAliq := anyStrPtr(cfopEntry, "is_aliq")
		isNode := buildIS(isCST, vBCIBSCBS, isAliq, cfopEntry)
		if isNode != nil {
			imposto["IS"] = isNode["IS"]
		}

		detItem := map[string]any{
			"@nItem":  i + 1,
			"prod":    prod,
			"imposto": imposto,
		}
		if infAd := anyStr(item, "inf_ad_prod", ""); infAd != "" {
			detItem["infAdProd"] = infAd
		}
		det = append(det, detItem)
	}

	// ── emit / dest structs ───────────────────────────────────────────────────
	emitKey := "CPF"
	if isEmitPJ {
		emitKey = "CNPJ"
	}
	emitStruct := map[string]any{
		emitKey:     emitDoc,
		"xNome":     anyStr(org, "name", ""),
		"xFant":     anyStr(orgPerson, "fantasy_name", ""),
		"enderEmit": buildEnder(orgPerson),
		"IE":        getIEForUF(orgPerson, emitUF),
		"CRT":       fmt.Sprintf("%d", orgCRT),
	}

	// dest is optional. For NFC-e the consumer (pessoa física) is identified by
	// CPF only — no address node — and may be omitted entirely (receiver == nil).
	// For NF-e dest is always present and carries the full address.
	var destStruct map[string]any
	hasDest := len(receiver) > 0
	destKey := "CPF"
	if isDestPJ {
		destKey = "CNPJ"
	}
	switch {
	case isNFCe:
		if hasDest {
			destStruct = map[string]any{
				destKey:     destDoc,
				"indIEDest": indIEDestNaoContrib,
			}
		}
	default:
		receiverName := anyStr(receiver, "name", "")
		if environment != 1 {
			receiverName = homNameReceiver
		}
		destStruct = map[string]any{
			destKey:     destDoc,
			"xNome":     receiverName,
			"enderDest": buildEnder(destPerson),
			"indIEDest": func() string {
				if destIE == "" {
					return indIEDestNaoContrib
				}
				return indIEDestContrib
			}(),
		}
		if destIE != "" {
			destStruct["IE"] = destIE
		}
		if email := services.FirstEmail(destPerson); email != "" {
			destStruct["email"] = email
		}
	}

	// ── totals ────────────────────────────────────────────────────────────────
	vNF := totalProducts.Sub(totalDiscount).
		Add(totalVFrete).Add(totalVSeg).Add(totalVOutro).
		Add(totalVIPI).Add(totalVICMSST).
		RoundBank(2)

	icmsTot := map[string]any{
		"vBC":        q2(totalVBC.RoundBank(2)),
		"vICMS":      q2(totalVICMS.RoundBank(2)),
		"vICMSDeson": q2(totalVICMSDeson.RoundBank(2)),
		"vFCP":       q2(totalVFCP.RoundBank(2)),
		"vBCST":      q2(totalVBCST.RoundBank(2)),
		"vST":        q2(totalVICMSST.RoundBank(2)),
		"vFCPST":     q2(totalVFCPST.RoundBank(2)),
		"vFCPSTRet":  "0.00",
		"vProd":      q2(totalProducts.RoundBank(2)),
		"vFrete":     q2(totalVFrete.RoundBank(2)),
		"vSeg":       q2(totalVSeg.RoundBank(2)),
		"vDesc":      q2(totalDiscount.RoundBank(2)),
		"vII":        "0.00",
		"vIPI":       q2(totalVIPI.RoundBank(2)),
		"vIPIDevol":  "0.00",
		"vPIS":       q2(totalVPIS.RoundBank(2)),
		"vCOFINS":    q2(totalVCOFINS.RoundBank(2)),
		"vOutro":     q2(totalVOutro.RoundBank(2)),
		"vNF":        q2(vNF),
		"vTotTrib":   "0.00",
	}
	if totalVICMSUFDest.GreaterThan(decimal.Zero) || totalVFCPUFDest.GreaterThan(decimal.Zero) {
		icmsTot["vFCPUFDest"] = q2(totalVFCPUFDest.RoundBank(2))
		icmsTot["vICMSUFDest"] = q2(totalVICMSUFDest.RoundBank(2))
		icmsTot["vICMSUFRemet"] = "0.00"
	}

	totalNode := map[string]any{
		"ICMSTot": icmsTot,
		"IBSCBSTot": map[string]any{
			"vBCIBSCBS": q2(totalIBSBC.RoundBank(2)),
			"gIBS": map[string]any{
				"gIBSUF": map[string]any{
					"vDif": "0.00", "vDevTrib": "0.00",
					"vIBSUF": q2(totalIBSUF.RoundBank(2)),
				},
				"gIBSMun": map[string]any{
					"vDif": "0.00", "vDevTrib": "0.00",
					"vIBSMun": q2(totalIBSMun.RoundBank(2)),
				},
				"vIBS":             q2(totalIBS.RoundBank(2)),
				"vCredPres":        "0.00",
				"vCredPresCondSus": "0.00",
			},
			"gCBS": map[string]any{
				"vDif": "0.00", "vDevTrib": "0.00",
				"vCBS":             q2(totalCBS.RoundBank(2)),
				"vCredPres":        "0.00",
				"vCredPresCondSus": "0.00",
			},
		},
	}
	if hasISSQN {
		totalNode["ISSQNtot"] = map[string]any{
			"vServ":   q2(totalVServ.RoundBank(2)),
			"vBC":     q2(totalVBCISSQN.RoundBank(2)),
			"vISS":    q2(totalVISSQN.RoundBank(2)),
			"vPIS":    q2(totalVPISISSQN.RoundBank(2)),
			"vCOFINS": q2(totalVCOFINSISSQN.RoundBank(2)),
			"dCompet": now.Format("2006-01-02"),
		}
	}

	// ── infNFe ────────────────────────────────────────────────────────────────
	natOpStr := natOpVenda
	if natOp != nil && *natOp != "" {
		natOpStr = truncateNatOp(*natOp)
	}
	var tpImp string
	if isNFCe {
		tpImp = tpImpDANFENFCe
	} else {
		tpImp = tpImpDANFERetrato
	}
	infNFe := map[string]any{
		"@versao": "4.00",
		"@Id":     fmt.Sprintf("NFe%s", accessKey),
		"ide": map[string]any{
			"cUF":      cUF,
			"cNF":      cNF,
			"natOp":    natOpStr,
			"mod":      model,
			"serie":    fmt.Sprintf("%d", serie),
			"nNF":      fmt.Sprintf("%d", number),
			"dhEmi":    dhEmi,
			"tpNF":     tpNF,
			"idDest":   idDest,
			"cMunFG":   strOrDefault(anyStr(orgAddress, "city_ibge_code", ""), "0000000"),
			"tpImp":    tpImp,
			"tpEmis":   tpEmisNormal,
			"cDV":      string(accessKey[len(accessKey)-1]),
			"tpAmb":    fmt.Sprintf("%d", environment),
			"finNFe":   finNFe,
			"indFinal": indFinal,
			"indPres":  indPres,
			"procEmi":  procEmiApp,
			"verProc":  tech.Version,
		},
		"emit":  emitStruct,
		"det":   det,
		"total": totalNode,
		"transp": buildTransp(hasPesoL, hasPesoB, totalPesoL, totalPesoB, transport,
			buildPartyTransporta(emitDoc, isEmitPJ, anyStr(org, "name", ""), getIEForUF(orgPerson, emitUF), orgAddress),
			buildPartyTransporta(destDoc, isDestPJ, anyStr(receiver, "name", ""), destIE, destAddress)),
		"pag":    buildPag(payments, vTroco),
	}
	if destStruct != nil {
		infNFe["dest"] = destStruct
	}
	if retiradaMap := buildLocal(retirada); retiradaMap != nil {
		infNFe["retirada"] = retiradaMap
	}
	if entregaMap := buildLocal(entrega); entregaMap != nil {
		infNFe["entrega"] = entregaMap
	}
	if cobrFat != nil || len(cobrDuplicatas) > 0 {
		infNFe["cobr"] = buildCobr(cobrFat, cobrDuplicatas)
	}
	if additionalInfo != nil && *additionalInfo != "" {
		infNFe["infAdic"] = map[string]any{"infCpl": *additionalInfo}
	}
	infNFe["infRespTec"] = map[string]any{
		"CNPJ":     tech.CNPJ,
		"xContato": tech.Name,
		"email":    tech.Email,
		"fone":     tech.Phone,
	}

	nfe := map[string]any{"infNFe": infNFe}
	if supl != nil {
		nfe["infNFeSupl"] = supl
	}

	return map[string]any{
		"enviNFe": map[string]any{
			"@versao": "4.00",
			"@xmlns":  "http://www.portalfiscal.inf.br/nfe",
			"idLote":  fmt.Sprintf("%015d", number),
			"indSinc": indSinc,
			"NFe":     nfe,
		},
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// truncateNatOp enforces the SEFAZ ide.natOp 60-char limit. The frontend sends a
// summarized CFOP description; this is a rune-safe safety net that truncates with
// an ellipsis suffix when the value exceeds natOpMaxLen.
func truncateNatOp(s string) string {
	r := []rune(s)
	if len(r) <= natOpMaxLen {
		return s
	}
	return string(r[:natOpMaxLen-3]) + "..."
}

func getPersonMap(entity map[string]any) map[string]any {
	if p, ok := entity["person"].(map[string]any); ok {
		return p
	}
	return map[string]any{}
}

func getAnyInt(m map[string]any, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return def
}

func getCFOPConfig(item map[string]any) []any {
	if v, ok := item["cfop_config"].([]any); ok {
		return v
	}
	return nil
}

func findCFOPEntry(entries []any, cfop string) map[string]any {
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			if m["cfop"] == cfop {
				return m
			}
		}
	}
	return map[string]any{}
}

func firstValue(m map[string]any) map[string]any {
	for _, v := range m {
		if inner, ok := v.(map[string]any); ok {
			return inner
		}
	}
	return map[string]any{}
}

func dv(m map[string]any, key string) decimal.Decimal {
	if m == nil {
		return decimal.Zero
	}
	if v, ok := m[key]; ok {
		switch s := v.(type) {
		case string:
			if s != "" {
				return d(s)
			}
		}
	}
	return decimal.Zero
}

func strOrDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
