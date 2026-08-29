package nfes

// builders_ibscbs.go — todo o bloco da reforma tributária no item da NF-e:
// IBS estadual, IBS municipal, CBS, monofasia, transferência e estorno de
// crédito, créditos presumidos e a tributação de referência.
//
// Fonte de verdade das tags e da ordem: PL_010e_v1.02 (DFeTiposBasicos_v1.00.xsd,
// tipos TTribNFe, TCIBS_NFe, TMonofasia, TTransfCred, TAjusteCompet,
// TEstornoCred, TCredPresOper, TCredPresIBSZFM, TALCZFMCBS_NFe, TTribRegular,
// TTribCompraGov). A ordem dos filhos vive em xsdorder, nas duas tabelas.
//
// Regra que atravessa o arquivo: **nada de valor é digitado duas vezes**. Toda
// tag `v*` sai de `p*` × base, e toda `pAliqEfet` sai de alíquota × (1 − redução).
// Extraído de builders_tax.go, que já não cabia num contexto só.

import (
	"github.com/shopspring/decimal"
)

// competApurLayout é o formato de competApur no leiaute (AAAA-MM).
const competApurLayout = "2006-01"

// ibsCBSExempt são os CSTs em que o item não tem valor de IBS/CBS a declarar:
// o grupo sai com CST e cClassTrib e nada mais.
var ibsCBSExempt = map[string]bool{
	"400": true, "410": true, "800": true, "810": true, "811": true, "820": true,
}

// cstIBSCBSMono é o CST monofásico do IBS/CBS.
const cstIBSCBSMono = "620"

// indDoacaoSim é o único valor que o XSD enumera para indDoacao (TIndDoacao).
// "S"/"N" era o domínio de uma NT anterior e hoje é rejeição.
const indDoacaoSim = "1"

// credPresCondSusSim marca o crédito presumido com condição suspensiva. É um
// flag, não um segundo valor: a conta é a mesma, só o destino muda de
// vCredPres para vCredPresCondSus.
const credPresCondSusSim = "1"

// Casas decimais do bloco: alíquotas e percentuais em 4 (TDec_0302_04RTC),
// valores em 2 (TDec1302RTC) e quantidade monofásica em 4 (TDec1104RTC).
const rtcRatePlaces = 4

// Campos do bloco da reforma na configuração tributária (perfil ou cfop_config).
const (
	// Tributação de referência (gTribRegular): o que o item pagaria fora do
	// regime/benefício. CST e cClassTrib próprios, mais as três alíquotas.
	cfgIBSRegCST       = "ibs_reg_cst"
	cfgIBSRegClassTrib = "ibs_reg_class_trib"
	cfgIBSRegUFAliq    = "ibs_reg_uf_aliq"
	cfgIBSRegMunAliq   = "ibs_reg_mun_aliq"
	cfgCBSRegAliq      = "cbs_reg_aliq"

	// Tributação de compra governamental (gTribCompraGov): o que o item
	// pagaria se o comprador não fosse ente público.
	cfgIBSGovUFAliq  = "ibs_gov_uf_aliq"
	cfgIBSGovMunAliq = "ibs_gov_mun_aliq"
	cfgCBSGovAliq    = "cbs_gov_aliq"

	// Monofasia (gIBSCBSMono): alíquota específica por unidade em cada
	// sub-grupo. A padrão já existia; retenção, retido e diferimento são novos.
	cfgIBSAdRem       = "ibs_ad_rem"
	cfgCBSAdRem       = "cbs_ad_rem"
	cfgIBSAdRemReten  = "ibs_ad_rem_reten"
	cfgCBSAdRemReten  = "cbs_ad_rem_reten"
	cfgIBSAdRemRet    = "ibs_ad_rem_ret"
	cfgCBSAdRemRet    = "cbs_ad_rem_ret"
	cfgIBSPDifMono    = "ibs_p_dif_mono"
	cfgCBSPDifMono    = "cbs_p_dif_mono"
	cfgIBSCBSPDevTrib = "ibs_cbs_p_dev_trib"
	cfgIBSIndDoacao   = "ibs_ind_doacao"

	// Crédito presumido da operação (gCredPresOper).
	cfgIBSCBSCCredPres  = "ibs_cbs_c_cred_pres"
	cfgIBSPCredPres     = "ibs_p_cred_pres"
	cfgCBSPCredPres     = "cbs_p_cred_pres"
	cfgCredPresCondSus  = "ibs_cbs_cred_pres_cond_sus"
	cfgIBSZFMPCredPres  = "ibs_zfm_p_cred_pres"
	cfgALCZFMTp         = "alc_zfm_tp_cbs"
	cfgALCZFMNProcSufra = "alc_zfm_n_proc_suframa"
)

// ibsCBSParams carrega o que o bloco da reforma precisa do laço de itens. É
// struct e não uma lista de parâmetros porque o bloco cresce por NT.
type ibsCBSParams struct {
	CST, ClassTrib        string
	VBC                   decimal.Decimal
	IBSUFAliq, IBSMunAliq string
	CBSAliq               string
	Cfg                   map[string]any
	// Quantity é a quantidade tributável do item: base da monofasia (qBCMono),
	// que é alíquota por unidade, não percentual sobre valor.
	Quantity decimal.Decimal
	// CompetApur é o período de apuração (AAAA-MM) da emissão, usado por
	// gAjusteCompet e gCredPresIBSZFM.
	CompetApur string
	// TpCredPresIBSZFM é a classificação da subapuração do IBS na ZFM, do
	// cadastro do produto.
	TpCredPresIBSZFM string
	// NProcSuframa é o processo Suframa do item (gALCZFMCBS), do produto.
	NProcSuframa string
	// Os quatro grupos que são alternativos a gIBSCBS no choice do XSD, e o
	// estorno, que convive com qualquer um deles. Vêm do request: são valores
	// que só existem naquela nota.
	TransfCred   *ibsCBSPair
	AjusteCompet *ibsCBSPair
	EstornoCred  *ibsCBSPair
}

// ibsCBSPair é o par de valores IBS/CBS que vários grupos da reforma repetem.
type ibsCBSPair struct {
	VIBS, VCBS string
}

// calcTaxValue devolve p% de vBC com 2 casas — a conta que toda tag `v*` da
// reforma faz.
func calcTaxValue(vBC decimal.Decimal, pAliq string) string {
	return q2(vBC.Mul(d(pAliq)).Div(decimal.NewFromInt(100)).RoundBank(2))
}

// calcAdRemValue devolve quantidade × alíquota específica com 2 casas — a conta
// da monofasia, que tributa por unidade e não por valor.
func calcAdRemValue(qty decimal.Decimal, adRem string) string {
	return q2(qty.Mul(d(adRem)).RoundBank(2))
}

// buildIBSCBS monta det/imposto/IBSCBS (type TTribNFe). Ordem XSD: CST,
// cClassTrib, indDoacao, o choice (gIBSCBS | gIBSCBSMono | gTransfCred |
// gAjusteCompet), gEstornoCred e o choice de crédito presumido
// (gCredPresOper | gCredPresIBSZFM).
func buildIBSCBS(p ibsCBSParams) map[string]any {
	outer := map[string]any{"CST": p.CST}
	if p.ClassTrib != "" {
		outer["cClassTrib"] = p.ClassTrib
	}
	// TIndDoacao enumera "1" e nada mais.
	if cfgStr(p.Cfg, cfgIBSIndDoacao, "") == indDoacaoSim {
		outer["indDoacao"] = indDoacaoSim
	}

	// gEstornoCred convive com qualquer ramo do choice.
	if node := buildEstornoCred(p.EstornoCred); node != nil {
		outer["gEstornoCred"] = node
	}
	// O crédito presumido é um choice à parte: operação ou ZFM, nunca os dois.
	if node := buildCredPresOper(p.Cfg, p.VBC); node != nil {
		outer["gCredPresOper"] = node
	} else if node := buildCredPresIBSZFM(p.Cfg, p.VBC, p.CompetApur, p.TpCredPresIBSZFM); node != nil {
		outer["gCredPresIBSZFM"] = node
	}

	// O choice principal, na ordem de precedência do leiaute: transferência e
	// ajuste de competência substituem a apuração normal do item.
	switch {
	case p.TransfCred != nil:
		outer["gTransfCred"] = map[string]any{
			"vIBS": q2(d(p.TransfCred.VIBS).RoundBank(2)),
			"vCBS": q2(d(p.TransfCred.VCBS).RoundBank(2)),
		}
		return outer
	case p.AjusteCompet != nil:
		outer["gAjusteCompet"] = map[string]any{
			"competApur": p.CompetApur,
			"vIBS":       q2(d(p.AjusteCompet.VIBS).RoundBank(2)),
			"vCBS":       q2(d(p.AjusteCompet.VCBS).RoundBank(2)),
		}
		return outer
	}

	if ibsCBSExempt[p.CST] {
		return outer
	}
	if p.CST == cstIBSCBSMono {
		outer["gIBSCBSMono"] = buildIBSCBSMono(p.Cfg, p.Quantity)
		return outer
	}
	outer["gIBSCBS"] = buildGIBSCBSValues(p)
	return outer
}

// buildGIBSCBSValues monta o nó gIBSCBS (type TCIBS_NFe). Ordem XSD: vBC,
// gIBSUF, gIBSMun, vIBS, gCBS, gTribRegular, gTribCompraGov.
func buildGIBSCBSValues(p ibsCBSParams) map[string]any {
	vIBSUF := d(calcTaxValue(p.VBC, p.IBSUFAliq))
	vIBSMun := d(calcTaxValue(p.VBC, p.IBSMunAliq))

	gIBSUF := map[string]any{"pIBSUF": p.IBSUFAliq}
	addRateSubgroups(gIBSUF, p, p.IBSUFAliq, "ibs_uf_p_dif", "ibs_uf_p_red")
	gIBSUF["vIBSUF"] = q2(vIBSUF.RoundBank(2))

	gIBSMun := map[string]any{"pIBSMun": p.IBSMunAliq}
	addRateSubgroups(gIBSMun, p, p.IBSMunAliq, "ibs_mun_p_dif", "ibs_mun_p_red")
	gIBSMun["vIBSMun"] = q2(vIBSMun.RoundBank(2))

	gCBS := map[string]any{"pCBS": p.CBSAliq}
	addRateSubgroups(gCBS, p, p.CBSAliq, "cbs_p_dif", "cbs_p_red")
	// gALCZFMCBS mora dentro de gCBS, antes do vCBS.
	if node := buildALCZFMCBS(p.Cfg, p.VBC, p.NProcSuframa); node != nil {
		gCBS["gALCZFMCBS"] = node
	}
	gCBS["vCBS"] = calcTaxValue(p.VBC, p.CBSAliq)

	node := map[string]any{
		"vBC":     q2(p.VBC.RoundBank(2)),
		"gIBSUF":  gIBSUF,
		"gIBSMun": gIBSMun,
		"vIBS":    q2(vIBSUF.Add(vIBSMun).RoundBank(2)),
		"gCBS":    gCBS,
	}
	if reg := buildTribRegular(p.Cfg, p.VBC); reg != nil {
		node["gTribRegular"] = reg
	}
	if gov := buildTribCompraGov(p.Cfg, p.VBC); gov != nil {
		node["gTribCompraGov"] = gov
	}
	return node
}

// addRateSubgroups acrescenta gDif, gDevTrib e gRed a um dos três nós de
// alíquota. Os três são idênticos em forma nos três nós — daí um helper só, em
// vez de três closures repetidas.
func addRateSubgroups(node map[string]any, p ibsCBSParams, aliq, difField, redField string) {
	if pDif := cfgStrPtr(p.Cfg, difField); pDif != nil {
		// vDif é o tributo que deixou de ser recolhido: percentual diferido
		// sobre o próprio valor da alíquota, não sobre a base.
		vTrib := d(calcTaxValue(p.VBC, aliq))
		node["gDif"] = map[string]any{
			"pDif": *pDif,
			"vDif": calcTaxValue(vTrib, *pDif),
		}
	}
	if node2 := buildDevTrib(p.Cfg, p.VBC, aliq); node2 != nil {
		node["gDevTrib"] = node2
	}
	if pRed := cfgStrPtr(p.Cfg, redField); pRed != nil {
		node["gRed"] = map[string]any{
			"pRedAliq":  *pRed,
			"pAliqEfet": effectiveRate(aliq, *pRed),
		}
	}
}

// effectiveRate é alíquota × (1 − redução), com as 4 casas do leiaute.
func effectiveRate(aliq, pRed string) string {
	one := decimal.NewFromInt(1)
	hundred := decimal.NewFromInt(100)
	return d(aliq).Mul(one.Sub(d(pRed).Div(hundred))).RoundBank(rtcRatePlaces).StringFixed(rtcRatePlaces)
}

// buildDevTrib monta gDevTrib (devolução de tributo ao adquirente). pDevTrib é
// opcional no XSD, mas vDevTrib não é: sem percentual não há o que devolver, e
// um vDevTrib de zero declarado seria ruído.
func buildDevTrib(cfg map[string]any, vBC decimal.Decimal, aliq string) map[string]any {
	pDev := cfgStrPtr(cfg, cfgIBSCBSPDevTrib)
	if pDev == nil {
		return nil
	}
	// A devolução é percentual sobre o tributo do item, não sobre a base.
	vTrib := d(calcTaxValue(vBC, aliq))
	return map[string]any{
		"pDevTrib": *pDev,
		"vDevTrib": calcTaxValue(vTrib, *pDev),
	}
}

// ── Monofasia (gIBSCBSMono, type TMonofasia) ─────────────────────────────────

// buildIBSCBSMono monta gIBSCBSMono. Ordem XSD: gMonoPadrao, gMonoReten,
// gMonoRet, gMonoDif, vTotIBSMonoItem, vTotCBSMonoItem.
//
// Espelha, em IBS/CBS, o que ICMS02/15/53/61 já fazem em ICMS: a base é a
// **quantidade**, não o valor, e cada valor é quantidade × alíquota específica.
// Os dois totais do item são a soma dos sub-grupos — nunca digitados.
func buildIBSCBSMono(cfg map[string]any, qty decimal.Decimal) map[string]any {
	node := map[string]any{}
	totIBS, totCBS := decimal.Zero, decimal.Zero
	qtyStr := q4(qty.RoundBank(rtcRatePlaces))

	adRemIBS := cfgStr(cfg, cfgIBSAdRem, "0.0000")
	adRemCBS := cfgStr(cfg, cfgCBSAdRem, "0.0000")
	vIBSMono := calcAdRemValue(qty, adRemIBS)
	vCBSMono := calcAdRemValue(qty, adRemCBS)
	node["gMonoPadrao"] = map[string]any{
		"qBCMono":  qtyStr,
		"adRemIBS": adRemIBS,
		"adRemCBS": adRemCBS,
		"vIBSMono": vIBSMono,
		"vCBSMono": vCBSMono,
	}
	totIBS = totIBS.Add(d(vIBSMono))
	totCBS = totCBS.Add(d(vCBSMono))

	// gMonoReten: tributação monofásica com retenção (o substituto recolhe).
	if adRemIBSReten, adRemCBSReten, ok := adRemPair(cfg, cfgIBSAdRemReten, cfgCBSAdRemReten); ok {
		vIBS := calcAdRemValue(qty, adRemIBSReten)
		vCBS := calcAdRemValue(qty, adRemCBSReten)
		node["gMonoReten"] = map[string]any{
			"qBCMonoReten":  qtyStr,
			"adRemIBSReten": adRemIBSReten,
			"vIBSMonoReten": vIBS,
			"adRemCBSReten": adRemCBSReten,
			"vCBSMonoReten": vCBS,
		}
		totIBS = totIBS.Add(d(vIBS))
		totCBS = totCBS.Add(d(vCBS))
	}

	// gMonoRet: tributo monofásico já retido anteriormente. Não soma no total
	// do item — já foi recolhido por outro.
	if adRemIBSRet, adRemCBSRet, ok := adRemPair(cfg, cfgIBSAdRemRet, cfgCBSAdRemRet); ok {
		node["gMonoRet"] = map[string]any{
			"qBCMonoRet":  qtyStr,
			"adRemIBSRet": adRemIBSRet,
			"vIBSMonoRet": calcAdRemValue(qty, adRemIBSRet),
			"adRemCBSRet": adRemCBSRet,
			"vCBSMonoRet": calcAdRemValue(qty, adRemCBSRet),
		}
	}

	// gMonoDif: parcela diferida do monofásico, percentual sobre o padrão.
	pDifIBS := cfgStrPtr(cfg, cfgIBSPDifMono)
	pDifCBS := cfgStrPtr(cfg, cfgCBSPDifMono)
	if pDifIBS != nil || pDifCBS != nil {
		pIBS := strOrDefault(ptrStr(pDifIBS), "0.0000")
		pCBS := strOrDefault(ptrStr(pDifCBS), "0.0000")
		node["gMonoDif"] = map[string]any{
			"pDifIBS":     pIBS,
			"vIBSMonoDif": calcTaxValue(d(vIBSMono), pIBS),
			"pDifCBS":     pCBS,
			"vCBSMonoDif": calcTaxValue(d(vCBSMono), pCBS),
		}
	}

	node["vTotIBSMonoItem"] = q2(totIBS.RoundBank(2))
	node["vTotCBSMonoItem"] = q2(totCBS.RoundBank(2))
	return node
}

// adRemPair lê o par de alíquotas específicas de um sub-grupo monofásico. O
// grupo é tudo-ou-nada: uma alíquota sozinha não descreve a operação.
func adRemPair(cfg map[string]any, ibsField, cbsField string) (string, string, bool) {
	ibs := cfgStrPtr(cfg, ibsField)
	cbs := cfgStrPtr(cfg, cbsField)
	if ibs == nil && cbs == nil {
		return "", "", false
	}
	return strOrDefault(ptrStr(ibs), "0.0000"), strOrDefault(ptrStr(cbs), "0.0000"), true
}

// ── Transferência, ajuste e estorno de crédito ──────────────────────────────

// buildEstornoCred monta gEstornoCred. Convive com qualquer ramo do choice
// principal, por isso fica fora dele.
func buildEstornoCred(pair *ibsCBSPair) map[string]any {
	if pair == nil {
		return nil
	}
	return map[string]any{
		"vIBSEstCred": q2(d(pair.VIBS).RoundBank(2)),
		"vCBSEstCred": q2(d(pair.VCBS).RoundBank(2)),
	}
}

// ── Créditos presumidos ─────────────────────────────────────────────────────

// buildCredPresOper monta gCredPresOper. Ordem XSD: vBCCredPres, cCredPres,
// gIBSCredPres, gCBSCredPres. Dentro de cada um, pCredPres e então o choice
// vCredPres | vCredPresCondSus — condição suspensiva é a mesma conta com
// destino diferente, então é um flag, não um segundo campo de valor.
func buildCredPresOper(cfg map[string]any, vBC decimal.Decimal) map[string]any {
	code := cfgStrPtr(cfg, cfgIBSCBSCCredPres)
	if code == nil {
		return nil
	}
	pIBS := cfgStrPtr(cfg, cfgIBSPCredPres)
	pCBS := cfgStrPtr(cfg, cfgCBSPCredPres)
	if pIBS == nil && pCBS == nil {
		return nil
	}
	condSus := cfgStr(cfg, cfgCredPresCondSus, "") == credPresCondSusSim
	node := map[string]any{
		"vBCCredPres": q2(vBC.RoundBank(2)),
		"cCredPres":   *code,
	}
	if pIBS != nil {
		node["gIBSCredPres"] = credPresValue(*pIBS, vBC, condSus)
	}
	if pCBS != nil {
		node["gCBSCredPres"] = credPresValue(*pCBS, vBC, condSus)
	}
	return node
}

// credPresValue monta um lado do crédito presumido (IBS ou CBS).
func credPresValue(pct string, vBC decimal.Decimal, condSus bool) map[string]any {
	node := map[string]any{"pCredPres": pct}
	if condSus {
		node["vCredPresCondSus"] = calcTaxValue(vBC, pct)
	} else {
		node["vCredPres"] = calcTaxValue(vBC, pct)
	}
	return node
}

// buildCredPresIBSZFM monta gCredPresIBSZFM. Ordem XSD: competApur,
// tpCredPresIBSZFM, vCredPresIBSZFM. A classificação vem do produto; o valor é
// o percentual cadastrado sobre a base.
func buildCredPresIBSZFM(cfg map[string]any, vBC decimal.Decimal, competApur, tp string) map[string]any {
	if tp == "" {
		return nil
	}
	pct := cfgStrPtr(cfg, cfgIBSZFMPCredPres)
	if pct == nil {
		return nil
	}
	return map[string]any{
		"competApur":       competApur,
		"tpCredPresIBSZFM": tp,
		"vCredPresIBSZFM":  calcTaxValue(vBC, *pct),
	}
}

// buildALCZFMCBS monta gCBS/gALCZFMCBS — a alíquota zero da CBS em área de
// livre comércio / Zona Franca de Manaus. Ordem XSD: tpALCZFMCBS,
// nProcSuframa, pAliqEfetRegCBS, vTribRegCBS.
//
// pAliqEfetRegCBS é a alíquota **de referência** (a que valeria fora da área) e
// vTribRegCBS é o que a CBS seria com ela — os dois medem o benefício, então
// nenhum dos dois é digitado junto do outro.
func buildALCZFMCBS(cfg map[string]any, vBC decimal.Decimal, nProcSuframa string) map[string]any {
	tp := cfgStrPtr(cfg, cfgALCZFMTp)
	if tp == nil {
		return nil
	}
	regAliq := cfgStr(cfg, cfgCBSRegAliq, "0.0000")
	node := map[string]any{
		"tpALCZFMCBS":     *tp,
		"pAliqEfetRegCBS": regAliq,
		"vTribRegCBS":     calcTaxValue(vBC, regAliq),
	}
	// O processo Suframa é do item; o cadastro do produto pode trazer o padrão.
	if proc := firstNonEmpty(nProcSuframa, cfgStr(cfg, cfgALCZFMNProcSufra, "")); proc != "" {
		node["nProcSuframa"] = proc
	}
	return node
}

// ── Tributação de referência ────────────────────────────────────────────────

// tribPairTags nomeia as seis tags de um bloco de tributação de referência.
// gTribRegular e gTribCompraGov são o mesmo bloco com nomes diferentes, então
// a montagem é uma só, parametrizada.
type tribPairTags struct {
	PIBSUF, VIBSUF   string
	PIBSMun, VIBSMun string
	PCBS, VCBS       string
}

// buildTribPairs monta os três pares (alíquota de referência, valor) sobre a
// mesma base.
func buildTribPairs(tags tribPairTags, vBC decimal.Decimal, ibsUF, ibsMun, cbs string) map[string]any {
	return map[string]any{
		tags.PIBSUF:  ibsUF,
		tags.VIBSUF:  calcTaxValue(vBC, ibsUF),
		tags.PIBSMun: ibsMun,
		tags.VIBSMun: calcTaxValue(vBC, ibsMun),
		tags.PCBS:    cbs,
		tags.VCBS:    calcTaxValue(vBC, cbs),
	}
}

// buildTribRegular monta gIBSCBS/gTribRegular — quanto o item pagaria fora do
// regime ou benefício. Ordem XSD: CSTReg, cClassTribReg e os três pares.
func buildTribRegular(cfg map[string]any, vBC decimal.Decimal) map[string]any {
	cst := cfgStrPtr(cfg, cfgIBSRegCST)
	if cst == nil {
		return nil
	}
	node := buildTribPairs(tribPairTags{
		PIBSUF: "pAliqEfetRegIBSUF", VIBSUF: "vTribRegIBSUF",
		PIBSMun: "pAliqEfetRegIBSMun", VIBSMun: "vTribRegIBSMun",
		PCBS: "pAliqEfetRegCBS", VCBS: "vTribRegCBS",
	}, vBC,
		cfgStr(cfg, cfgIBSRegUFAliq, "0.0000"),
		cfgStr(cfg, cfgIBSRegMunAliq, "0.0000"),
		cfgStr(cfg, cfgCBSRegAliq, "0.0000"),
	)
	node["CSTReg"] = *cst
	node["cClassTribReg"] = cfgStr(cfg, cfgIBSRegClassTrib, "")
	return node
}

// buildTribCompraGov monta gIBSCBS/gTribCompraGov — quanto o item pagaria se o
// comprador não fosse ente público. Não tem CST próprio no XSD.
func buildTribCompraGov(cfg map[string]any, vBC decimal.Decimal) map[string]any {
	if cfgStrPtr(cfg, cfgIBSGovUFAliq) == nil &&
		cfgStrPtr(cfg, cfgIBSGovMunAliq) == nil &&
		cfgStrPtr(cfg, cfgCBSGovAliq) == nil {
		return nil
	}
	return buildTribPairs(tribPairTags{
		PIBSUF: "pAliqIBSUF", VIBSUF: "vTribIBSUF",
		PIBSMun: "pAliqIBSMun", VIBSMun: "vTribIBSMun",
		PCBS: "pAliqCBS", VCBS: "vTribCBS",
	}, vBC,
		cfgStr(cfg, cfgIBSGovUFAliq, "0.0000"),
		cfgStr(cfg, cfgIBSGovMunAliq, "0.0000"),
		cfgStr(cfg, cfgCBSGovAliq, "0.0000"),
	)
}

// ── Acumulação nos totais e leitura do item ─────────────────────────────────

// accumulateIBSCBS soma o item nos acumuladores do IBSCBSTot. Ler os valores de
// volta do nó (em vez de recalcular) é o que garante que o total seja
// exatamente a soma do que foi emitido — o teste de conservação depende disso.
func accumulateIBSCBS(t *totals, node map[string]any, vBC decimal.Decimal) {
	if inner, ok := node["gIBSCBS"].(map[string]any); ok {
		t.IBSBC = t.IBSBC.Add(vBC)
		gIBSUF, _ := inner["gIBSUF"].(map[string]any)
		gIBSMun, _ := inner["gIBSMun"].(map[string]any)
		gCBS, _ := inner["gCBS"].(map[string]any)
		itemIBSUF := d(anyStr(gIBSUF, "vIBSUF", "0"))
		itemIBSMun := d(anyStr(gIBSMun, "vIBSMun", "0"))
		t.IBSUF = t.IBSUF.Add(itemIBSUF)
		t.IBSMun = t.IBSMun.Add(itemIBSMun)
		t.IBS = t.IBS.Add(itemIBSUF).Add(itemIBSMun)
		t.CBSBC = t.CBSBC.Add(vBC)
		t.CBS = t.CBS.Add(d(anyStr(gCBS, "vCBS", "0")))
		// Diferimento e devolução de tributo entram nos seus próprios
		// acumuladores do total, não no valor do imposto.
		t.IBSUFDif = t.IBSUFDif.Add(subgroupValue(gIBSUF, "gDif", "vDif"))
		t.IBSMunDif = t.IBSMunDif.Add(subgroupValue(gIBSMun, "gDif", "vDif"))
		t.CBSDif = t.CBSDif.Add(subgroupValue(gCBS, "gDif", "vDif"))
		t.IBSUFDevTrib = t.IBSUFDevTrib.Add(subgroupValue(gIBSUF, "gDevTrib", "vDevTrib"))
		t.IBSMunDevTrib = t.IBSMunDevTrib.Add(subgroupValue(gIBSMun, "gDevTrib", "vDevTrib"))
		t.CBSDevTrib = t.CBSDevTrib.Add(subgroupValue(gCBS, "gDevTrib", "vDevTrib"))
	}

	if mono, ok := node["gIBSCBSMono"].(map[string]any); ok {
		t.HasIBSCBSMono = true
		padrao, _ := mono["gMonoPadrao"].(map[string]any)
		t.IBSMono = t.IBSMono.Add(d(anyStr(padrao, "vIBSMono", "0")))
		t.CBSMono = t.CBSMono.Add(d(anyStr(padrao, "vCBSMono", "0")))
		if reten, ok := mono["gMonoReten"].(map[string]any); ok {
			t.IBSMonoReten = t.IBSMonoReten.Add(d(anyStr(reten, "vIBSMonoReten", "0")))
			t.CBSMonoReten = t.CBSMonoReten.Add(d(anyStr(reten, "vCBSMonoReten", "0")))
		}
		if ret, ok := mono["gMonoRet"].(map[string]any); ok {
			t.IBSMonoRet = t.IBSMonoRet.Add(d(anyStr(ret, "vIBSMonoRet", "0")))
			t.CBSMonoRet = t.CBSMonoRet.Add(d(anyStr(ret, "vCBSMonoRet", "0")))
		}
	}

	if est, ok := node["gEstornoCred"].(map[string]any); ok {
		t.HasEstornoCred = true
		t.IBSEstCred = t.IBSEstCred.Add(d(anyStr(est, "vIBSEstCred", "0")))
		t.CBSEstCred = t.CBSEstCred.Add(d(anyStr(est, "vCBSEstCred", "0")))
	}

	// Crédito presumido: o total distingue o crédito do crédito com condição
	// suspensiva, e os dois lados (IBS e CBS) somam em acumuladores separados.
	if oper, ok := node["gCredPresOper"].(map[string]any); ok {
		addCredPres(&t.IBSCredPres, &t.IBSCredPresCondSus, oper, "gIBSCredPres")
		addCredPres(&t.CBSCredPres, &t.CBSCredPresCondSus, oper, "gCBSCredPres")
	}
}

// subgroupValue lê uma tag de valor de um sub-grupo opcional, devolvendo zero
// quando o sub-grupo não existe.
func subgroupValue(parent map[string]any, group, tag string) decimal.Decimal {
	inner, ok := parent[group].(map[string]any)
	if !ok {
		return decimal.Zero
	}
	return d(anyStr(inner, tag, "0"))
}

// addCredPres soma um lado do crédito presumido no acumulador certo — o choice
// do XSD é vCredPres **ou** vCredPresCondSus.
func addCredPres(direct, condSus *decimal.Decimal, oper map[string]any, group string) {
	inner, ok := oper[group].(map[string]any)
	if !ok {
		return
	}
	if v := anyStr(inner, "vCredPresCondSus", ""); v != "" {
		*condSus = condSus.Add(d(v))
		return
	}
	*direct = direct.Add(d(anyStr(inner, "vCredPres", "0")))
}

// itemIBSCBSPair lê do item o par de valores IBS/CBS de um dos grupos que só
// existem por nota (transferência, ajuste de competência, estorno). O par é
// tudo-ou-nada: um lado sozinho não descreve a operação.
func itemIBSCBSPair(item map[string]any, prefix string) *ibsCBSPair {
	vIBS := anyStr(item, prefix+"_v_ibs", "")
	vCBS := anyStr(item, prefix+"_v_cbs", "")
	if vIBS == "" && vCBS == "" {
		return nil
	}
	return &ibsCBSPair{
		VIBS: strOrDefault(vIBS, "0.00"),
		VCBS: strOrDefault(vCBS, "0.00"),
	}
}
