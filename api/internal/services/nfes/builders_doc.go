package nfes

// builders_doc.go ports app/services/nfes/_builders_doc.py.
// Produces the enviNFe dict structure that py-dfe Lambda signs and sends to SEFAZ.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	nfeXMLNS  = "http://www.portalfiscal.inf.br/nfe"
	nfModel55 = "55"
	nfModel65 = "65"

	// ide/tpEmis — forma de emissão (leiauteNFe_v4.00). Só `1` é normal; todo
	// o resto é contingência e exige o grupo dhCont + xJust.
	tpEmisNormal    = "1"
	TpEmisFS        = "2" // Formulário de Segurança
	TpEmisRegEspNFF = "3" // Regime Especial NFF
	TpEmisEPEC      = "4" // EPEC — evento prévio, transmissão posterior
	TpEmisFSDA      = "5" // Formulário de Segurança para Impressão de DANFE
	TpEmisSVCAN     = "6" // Sefaz Virtual de Contingência — Ambiente Nacional
	TpEmisSVCRS     = "7" // Sefaz Virtual de Contingência — RS
	TpEmisOffline   = "9" // NFC-e offline

	// ide/tpImp — layout do DANFE. Contingência muda o formato impresso.
	tpImpDANFERetrato  = "1"
	TpImpDANFEPaisagem = "2"
	TpImpDANFESimpl    = "3"
	tpImpDANFENFCe     = "4"
	TpImpDANFENFCeMsg  = "5" // NFC-e em mensagem eletrônica
	TpImpDANFESimplT2  = "6" // DANFE NFC-e Simplificado (contingência offline)

	// contJustificationMin is the xJust minimum the XSD imposes on contingency.
	contJustificationMin = 15

	procEmiApp          = "0"
	indTotCompoe        = "1"
	indIEDestNaoContrib = "9"
	indIEDestContrib    = "1"
	modFreteSemFrete    = "9"
	// Own-transport modFrete codes: the carrier (transporta) IS the issuer or the
	// recipient, so its data is copied from emit/dest instead of supplied separately.
	modFreteProprioRemetente    = "3" // Transporte próprio por conta do remetente (emitente)
	modFreteProprioDestinatario = "4" // Transporte próprio por conta do destinatário
	qVolPadrao                  = "1"
	cPaisBrasil                 = "1058"
	// ufExterior é a UF convencional do destinatário no exterior (leiauteNFe).
	ufExterior      = "EX"
	xPaisBrasil     = "Brasil"
	indSinc         = "1"
	natOpVenda      = "Venda de Mercadoria"
	homNameReceiver = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
	homProduct      = "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
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

// buildPag builds the pag XML node.
func buildPag(payments []map[string]any, vTroco *string, terminals map[string]map[string]any) map[string]any {
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
		if v := anyStr(p, "x_pag", ""); v != "" {
			item["xPag"] = v
		}
		term := terminals[anyStr(p, "terminal_id", "")]
		if v := anyStr(term, "cnpj_pag", ""); v != "" {
			item["CNPJPag"] = v
			// UFPag só é válido acompanhado de CNPJPag.
			if uf := anyStr(term, "uf_pag", ""); uf != "" {
				item["UFPag"] = uf
			}
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
			if v := anyStr(term, "cnpj_receb", ""); v != "" {
				cardNode["CNPJReceb"] = v
			}
			if v := anyStr(term, "id_term_pag", ""); v != "" {
				cardNode["idTermPag"] = v
			}
			if _, ok := cardNode["tBand"]; !ok {
				if v := anyStr(term, "t_band", ""); v != "" {
					cardNode["tBand"] = v
				}
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
	mode EmissionMode,
	extra docExtras,
) map[string]any {
	isNFCe := model == nfModel65
	// Período de apuração da reforma (AAAA-MM): é o mês da emissão, nunca um
	// campo — gAjusteCompet e gCredPresIBSZFM leem daqui.
	competApur := now.Format(competApurLayout)
	orgPerson := getPersonMap(org)
	orgAddress := services.FirstAddress(orgPerson)
	orgCRT := getAnyInt(orgPerson, "crt", 1)
	emitUF := anyStr(orgAddress, "state_federation", "")
	cUF := services.UFCode[emitUF]
	if cUF == "" {
		cUF = "35"
	}

	emitDoc := services.StripPKPrefix(orgPK)
	isEmitPJ := strings.HasPrefix(orgPK, cnpjPrefix)

	receiverSK := anyStr(receiver, "sk", "")
	destDoc := services.StripPKPrefix(receiverSK)
	isDestPJ := strings.HasPrefix(receiverSK, cnpjPrefix)
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
	t := newTotals(totalProducts, totalDiscount)
	var (
		totalPesoL = decimal.Zero
		totalPesoB = decimal.Zero
		hasPesoL   = false
		hasPesoB   = false
	)

	det := make([]map[string]any, 0, len(productItems))

	for i, item := range productItems {
		cfopEntries := getCFOPConfig(item)
		cfopEntry := findCFOPEntry(cfopEntries, anyStr(item, "cfop", ""))
		origin := anyStr(item, "origin", "0")

		originPtr := &origin
		itemNCM := anyStr(item, "ncm", "")
		pICMSResolved := resolveICMSAliq(emitUF, destUF, itemNCM, anyStrPtr(item, "icms_aliq_override"))
		pFCPResolved := resolveFCPAliq(destUF, itemNCM, anyStrPtr(item, "fcp_aliq_override"))

		qty := d(anyStr(item, "quantity", "0"))
		unitVal := d(anyStr(item, "unit_value", "0"))
		disc := d(anyStr(item, "discount", "0"))
		vFretItem := d(anyStr(item, "v_frete", "0"))
		vSegItem := d(anyStr(item, "v_seg", "0"))
		vOutroItem := d(anyStr(item, "v_outro", "0"))
		vProdDec := qty.Mul(unitVal).RoundBank(2)
		vBCIBSCBS := vProdDec.Sub(disc).RoundBank(2)
		vProd := q2(vProdDec)

		t.VFrete = t.VFrete.Add(vFretItem)
		t.VSeg = t.VSeg.Add(vSegItem)
		t.VOutro = t.VOutro.Add(vOutroItem)

		isISSQNItem := cfgStr(cfopEntry, "issqn_ind_iss", "") != ""

		var issqnNode map[string]any
		var icmsForImposto map[string]any
		var icmsUFDestNode map[string]any

		if isISSQNItem {
			issqnNode = buildISSQN(vBCIBSCBS, cfopEntry)
			if issqnNode != nil {
				t.HasISSQN = true
				issqnInner, _ := issqnNode["ISSQN"].(map[string]any)
				t.VServ = t.VServ.Add(vProdDec)
				t.VBCISSQN = t.VBCISSQN.Add(vBCIBSCBS)
				t.VISSQN = t.VISSQN.Add(d(anyStr(issqnInner, "vISSQN", "0")))
				// ISSQNtot repete, somados, os mesmos valores do item.
				t.VDeducaoISSQN = t.VDeducaoISSQN.Add(d(anyStr(issqnInner, "vDeducao", "0")))
				t.VOutroISSQN = t.VOutroISSQN.Add(d(anyStr(issqnInner, "vOutro", "0")))
				t.VDescIncondISSQN = t.VDescIncondISSQN.Add(d(anyStr(issqnInner, "vDescIncond", "0")))
				t.VDescCondISSQN = t.VDescCondISSQN.Add(d(anyStr(issqnInner, "vDescCond", "0")))
				t.VISSRet = t.VISSRet.Add(d(anyStr(issqnInner, "vISSRet", "0")))
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

			t.VBC = t.VBC.Add(dv(icmsInner, "vBC"))
			t.VICMS = t.VICMS.Add(dv(icmsInner, "vICMS"))
			t.VICMSDeson = t.VICMSDeson.Add(dv(icmsInner, "vICMSDeson"))
			t.VFCP = t.VFCP.Add(dv(icmsInner, "vFCP"))
			t.VBCST = t.VBCST.Add(dv(icmsInner, "vBCST"))
			t.VICMSST = t.VICMSST.Add(dv(icmsInner, "vICMSST"))
			t.VFCPST = t.VFCPST.Add(dv(icmsInner, "vFCPST"))

			icmsUFDestNode = map[string]any{}
			if isDifalEligible && icmsCSTDifalEligible[cstForItem] {
				pICMSUFDest := resolveICMSIntraAliq(destUF)
				pICMSInter := resolveICMSInterAliq(emitUF, destUF, originPtr)
				pFCPUFDest := resolveFCPAliq(destUF, "", nil)
				icmsUFDestNode = buildICMSUFDest(vBCIBSCBS, pICMSUFDest, pICMSInter, pFCPUFDest)
				t.VICMSUFDest = t.VICMSUFDest.Add(d(anyStr(icmsUFDestNode, "vICMSUFDest", "0")))
				t.VFCPUFDest = t.VFCPUFDest.Add(d(anyStr(icmsUFDestNode, "vFCPUFDest", "0")))
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
								var err error
								factor, err = decimal.NewFromString(v.String())
								if err != nil {
									slog.Warn("NF-e conversion factor parse failed", "err", err)
									continue
								}

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

		gIBSCBS := buildIBSCBS(ibsCBSParams{
			CST: ibsCBSCST, ClassTrib: ibsCBSClassTrib, VBC: vBCIBSCBS,
			IBSUFAliq: ibsUFAliq, IBSMunAliq: ibsMunAliq, CBSAliq: cbsAliq,
			Cfg: cfopEntry, Quantity: qty, CompetApur: competApur,
			TpCredPresIBSZFM: anyStr(item, "tp_cred_pres_ibs_zfm", ""),
			NProcSuframa:     anyStr(item, "alc_zfm_n_proc_suframa", ""),
			TransfCred:       itemIBSCBSPair(item, "transf_cred"),
			AjusteCompet:     itemIBSCBSPair(item, "ajuste_compet"),
			EstornoCred:      itemIBSCBSPair(item, "estorno_cred"),
		})

		accumulateIBSCBS(&t, gIBSCBS, vBCIBSCBS)

		var prodDescription string
		if isNFCe && environment == 2 && i == 0 {
			prodDescription = homProduct
		} else {
			prodDescription = anyStr(item, "description", "")
		}

		// prod node
		prod := buildProd(item, prodParams{
			Description: prodDescription,
			Unit:        unit,
			TaxableUnit: taxableUnit,
			QTrib:       qTrib,
			VUnTrib:     vUnTrib,
			VProd:       vProd,
			Disc:        disc,
			VFrete:      vFretItem,
			VSeg:        vSegItem,
			VOutro:      vOutroItem,
		})

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
				t.VPIS = t.VPIS.Add(pisV)
				if isISSQNItem {
					t.VPISISSQN = t.VPISISSQN.Add(pisV)
				}
				break
			}
		}
		for _, ck := range []string{"COFINSAliq", "COFINSOutr", "COFINSQtde"} {
			if inner, ok := cofinsBuilt[ck].(map[string]any); ok {
				cofinsV := d(anyStr(inner, "vCOFINS", "0"))
				t.VCOFINS = t.VCOFINS.Add(cofinsV)
				if isISSQNItem {
					t.VCOFINSISSQN = t.VCOFINSISSQN.Add(cofinsV)
				}
				break
			}
		}

		// PISST/COFINSST são grupos irmãos de PIS/COFINS, não parte deles:
		// combustíveis e farmacêutico recolhem a ST separada.
		pisSTNode := buildPISST(cfopEntry, vBCIBSCBS)
		cofinsSTNode := buildCOFINSST(cfopEntry, vBCIBSCBS)

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
			ipiNode := buildIPI(ipiCST, vBCIBSCBS, qty, cfopEntry, item)
			if ipiNode != nil {
				if ipiData, ok := ipiNode["IPI"].(map[string]any); ok {
					if ipiTrib, ok := ipiData["IPITrib"].(map[string]any); ok {
						t.VIPI = t.VIPI.Add(d(anyStr(ipiTrib, "vIPI", "0")))
					}
					imposto["IPI"] = ipiData
				}
			}

			if len(icmsUFDestNode) > 0 {
				imposto["ICMSUFDest"] = icmsUFDestNode
			}
		}

		if iiNode := buildII(item, vProdDec); iiNode != nil {
			imposto["II"] = iiNode
			t.VII = t.VII.Add(d(anyStr(iiNode, "vII", "0")))
		}
		if pisSTNode != nil {
			imposto["PISST"] = pisSTNode
		}
		if cofinsSTNode != nil {
			imposto["COFINSST"] = cofinsSTNode
		}

		isCST := cfgStr(cfopEntry, "is_cst", "")
		isAliq := anyStrPtr(cfopEntry, "is_aliq")
		isNode := buildIS(isCST, vBCIBSCBS, isAliq, cfopEntry)
		if isNode != nil {
			imposto["IS"] = isNode["IS"]
			// O ISTot é a soma do vIS dos itens — lida do nó emitido, não
			// recalculada, para que total e itens fechem centavo a centavo.
			if inner, ok := isNode["IS"].(map[string]any); ok {
				t.VIS = t.VIS.Add(d(anyStr(inner, "vIS", "0")))
			}
		}

		detItem := map[string]any{
			"@nItem":  i + 1,
			"prod":    prod,
			"imposto": imposto,
		}
		if infAd := anyStr(item, "inf_ad_prod", ""); infAd != "" {
			detItem["infAdProd"] = infAd
		}
		if obs := buildObsItem(cfopEntry, item); obs != nil {
			detItem["obsItem"] = obs
		}
		// impostoDevol só existe na devolução (finNFe=4); a emissão recusa
		// p_devol fora desse caso antes de chegar aqui.
		if extra.FinNFe4 {
			if pDevol := anyStr(item, "p_devol", ""); pDevol != "" {
				vIPIItem := decimal.Zero
				if ipiData, ok := imposto["IPI"].(map[string]any); ok {
					if ipiTrib, ok := ipiData["IPITrib"].(map[string]any); ok {
						vIPIItem = d(anyStr(ipiTrib, "vIPI", "0"))
					}
				}
				devol := buildImpostoDevol(pDevol, vIPIItem)
				detItem["impostoDevol"] = devol
				t.VIPIDevol = t.VIPIDevol.Add(d(anyStr(devol["IPI"].(map[string]any), "vIPIDevol", "0")))
			}
		}
		det = append(det, detItem)
	}

	// ── emit / dest structs ───────────────────────────────────────────────────
	emitStruct := buildEmit(org, orgPerson, orgPK, emitUF, destUF, orgCRT)

	// dest é opcional só na NFC-e (consumidor não identificado).
	destStruct := buildDest(receiver, destPerson, receiverSK, destUF, isNFCe, environment, destIE)

	// ── totals ────────────────────────────────────────────────────────────────
	// A base das retenções é o valor dos produtos líquido de desconto.
	totalNode := buildTotal(t, now, buildRetTrib(extra.RetTrib, t.Products.Sub(t.Discount)))

	// ── infNFe ────────────────────────────────────────────────────────────────
	natOpStr := natOpVenda
	if natOp != nil && *natOp != "" {
		natOpStr = truncateNatOp(*natOp)
	}
	ide := buildIde(ideParams{
		CUF: cUF, CNF: cNF, NatOp: natOpStr, Model: model, AccessKey: accessKey,
		Serie: serie, Number: number, Environment: environment,
		DhEmi: dhEmi, TpNF: tpNF, IdDest: idDest,
		CMunFG:   strOrDefault(anyStr(orgAddress, "city_ibge_code", ""), "0000000"),
		FinNFe:   finNFe,
		IndFinal: indFinal,
		IndPres:  indPres,
		Mode:     mode,
		VerProc:  tech.Version,
		NFref:    extra.NFRefs,
		DhSaiEnt: extra.DhSaiEnt, DPrevEntrega: extra.DPrevEntrega,
		IndIntermed: extra.IndIntermed,
		CIndOp:      extra.CIndOp, CMunFGIBS: extra.CMunFGIBS,
		TpNFDebito: extra.TpNFDebito, TpNFCredito: extra.TpNFCredito,
		CompraGov: extra.CompraGov, PagAntecipado: extra.PagAntecipado,
	})
	infNFe := map[string]any{
		"@versao": "4.00",
		"@Id":     fmt.Sprintf("NFe%s", accessKey),
		"ide":     ide,
		"emit":    emitStruct,
		"det":     det,
		"total":   totalNode,
		"transp": buildTransp(hasPesoL, hasPesoB, totalPesoL, totalPesoB, transport,
			buildPartyTransporta(emitDoc, isEmitPJ, anyStr(org, "name", ""), getIEForUF(orgPerson, emitUF), orgAddress),
			buildPartyTransporta(destDoc, isDestPJ, anyStr(receiver, "name", ""), destIE, destAddress),
			extra.Vols, extra.Reboques),
		"pag": buildPag(payments, vTroco, extra.PaymentTerminals),
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
	if autXML := buildAutXML(org); autXML != nil {
		infNFe["autXML"] = autXML
	}
	if cobrFat != nil || len(cobrDuplicatas) > 0 {
		infNFe["cobr"] = buildCobr(cobrFat, cobrDuplicatas)
	}
	if len(extra.Exporta) > 0 {
		infNFe["exporta"] = extra.Exporta
	}
	if infAdic := buildInfAdic(extra.InfAdFisco, ptrStr(additionalInfo),
		extra.ObsCont, extra.ObsFisco, extra.ProcRef); infAdic != nil {
		infNFe["infAdic"] = infAdic
	}
	if len(extra.InfIntermed) > 0 {
		infNFe["infIntermed"] = extra.InfIntermed
	}
	if len(extra.Compra) > 0 {
		infNFe["compra"] = extra.Compra
	}
	if len(extra.Cana) > 0 {
		infNFe["cana"] = extra.Cana
	}
	if len(extra.Agropecuario) > 0 {
		infNFe["agropecuario"] = extra.Agropecuario
	}
	infNFe["infRespTec"] = services.BuildRespTec(
		tech.CNPJ, tech.Name, tech.Email, tech.Phone,
		extra.CsrtID, extra.Csrt, accessKey)

	nfe := map[string]any{"infNFe": infNFe}
	if supl != nil {
		nfe["infNFeSupl"] = supl
	}

	return map[string]any{
		"enviNFe": map[string]any{
			"@versao": "4.00",
			"@xmlns":  nfeXMLNS,
			"idLote":  fmt.Sprintf("%015d", number),
			"indSinc": indSinc,
			"NFe":     nfe,
		},
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

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

// anyInt lê um inteiro opcional de um mapa vindo do DynamoDB (onde números
// chegam como float64). O segundo retorno distingue "ausente" de "zero" — um
// offset de 0 dia significa saída no mesmo dia, não campo em branco.
func anyInt(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	switch n := m[key].(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
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
