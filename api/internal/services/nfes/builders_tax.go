package nfes

// builders_tax.go ports app/services/nfes/_builders_tax.py.
// All builders return map[string]any matching the exact dict structure py-dfe Lambda expects.

import (
	"github.com/shopspring/decimal"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func cfgStr(c map[string]any, key, def string) string {
	if c == nil {
		return def
	}
	if v, ok := c[key]; ok {
		switch s := v.(type) {
		case string:
			if s != "" {
				return s
			}
		}
	}
	return def
}

func cfgStrPtr(c map[string]any, key string) *string {
	if c == nil {
		return nil
	}
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

// ─── ICMS — Simples Nacional ──────────────────────────────────────────────────

func buildICMSSN(origin, csosn string, vProd decimal.Decimal, cfg map[string]any) map[string]any {
	pCredSN := cfgStr(cfg, "icms_sn_cred_aliq", "0.00")
	modBCST := cfgStr(cfg, "icms_st_mod_bc", "4")
	pMVAST := cfgStr(cfg, "icms_st_mva", "0.00")
	pRedBCST := cfgStr(cfg, "icms_st_red_bc", "0.00")
	pICMSST := cfgStr(cfg, "icms_st_aliq", "0.00")
	pFCPST := cfgStr(cfg, "icms_st_fcp_aliq", "0.00")
	hasFCPST := pFCPST != "" && pFCPST != "0.00"

	pMVASTd := d(pMVAST)
	pRedBCSTd := decimal.Zero
	if pRedBCST != "" && pRedBCST != "0.00" {
		pRedBCSTd = d(pRedBCST)
	}
	vBCSTd := vProd.Mul(decimal.NewFromInt(1).Add(pMVASTd.Div(decimal.NewFromInt(100)))).
		Mul(decimal.NewFromInt(1).Sub(pRedBCSTd.Div(decimal.NewFromInt(100)))).
		RoundBank(2)
	vICMSSTd := vBCSTd.Mul(d(pICMSST)).Div(decimal.NewFromInt(100)).RoundBank(2)
	vFCPSTd := decimal.Zero
	if hasFCPST {
		vFCPSTd = vBCSTd.Mul(d(pFCPST)).Div(decimal.NewFromInt(100)).RoundBank(2)
	}
	vCredSNd := vProd.Mul(d(pCredSN)).Div(decimal.NewFromInt(100)).RoundBank(2)

	vBCST := q2(vBCSTd)
	vICMSST := q2(vICMSSTd)
	vFCPSTStr := q2(vFCPSTd)
	vCredSN := q2(vCredSNd)

	switch csosn {
	case "102", "103", "300", "400":
		return map[string]any{"ICMSSN102": map[string]any{"orig": origin, "CSOSN": csosn}}
	case "500":
		// CSOSN 500 é a revenda de mercadoria com ST já retida: os mesmos
		// valores retidos e o mesmo ICMS efetivo do ICMS60 do regime normal.
		node := map[string]any{"orig": origin, "CSOSN": csosn}
		addSTRetida(node, cfg)
		addICMSEfetivo(node, vProd, cfg)
		return map[string]any{"ICMSSN500": node}
	case "101":
		return map[string]any{"ICMSSN101": map[string]any{
			"orig": origin, "CSOSN": csosn, "pCredSN": pCredSN, "vCredICMSSN": vCredSN,
		}}
	case "201":
		node := map[string]any{
			"orig": origin, "CSOSN": csosn, "modBCST": modBCST,
			"pMVAST": pMVAST, "pRedBCST": pRedBCST, "vBCST": vBCST,
			"pICMSST": pICMSST, "vICMSST": vICMSST,
			"pCredSN": pCredSN, "vCredICMSSN": vCredSN,
		}
		if hasFCPST {
			node["vBCFCPST"] = vBCST
			node["pFCPST"] = pFCPST
			node["vFCPST"] = vFCPSTStr
		}
		return map[string]any{"ICMSSN201": node}
	case "202", "203":
		node := map[string]any{
			"orig": origin, "CSOSN": csosn, "modBCST": modBCST,
			"pMVAST": pMVAST, "pRedBCST": pRedBCST, "vBCST": vBCST,
			"pICMSST": pICMSST, "vICMSST": vICMSST,
		}
		if hasFCPST {
			node["vBCFCPST"] = vBCST
			node["pFCPST"] = pFCPST
			node["vFCPST"] = vFCPSTStr
		}
		return map[string]any{"ICMSSN202": node}
	case "900":
		pICMS900 := cfgStr(cfg, "icms_aliq_override", "0.00")
		vBC900 := q2(vProd.RoundBank(2))
		vICMS900 := q2(vProd.Mul(d(pICMS900)).Div(decimal.NewFromInt(100)).RoundBank(2))
		return map[string]any{"ICMSSN900": map[string]any{
			"orig": origin, "CSOSN": csosn, "modBC": "3",
			"vBC": vBC900, "pRedBC": "0.00", "pICMS": pICMS900, "vICMS": vICMS900,
			"modBCST": modBCST, "pMVAST": pMVAST, "pRedBCST": pRedBCST,
			"vBCST": vBCST, "pICMSST": pICMSST, "vICMSST": vICMSST,
		}}
	}
	return map[string]any{"ICMSSN102": map[string]any{"orig": origin, "CSOSN": "102"}}
}

// ─── ICMS — Regime Normal ─────────────────────────────────────────────────────

func buildICMSNormal(origin, cst string, vProd decimal.Decimal, cfg map[string]any, pICMSResolved, pFCPResolved string, qty decimal.Decimal) map[string]any {
	modBC := cfgStr(cfg, "icms_mod_bc", "3")
	pICMS := cfgStr(cfg, "icms_aliq_override", pICMSResolved)
	if pICMS == "" {
		pICMS = pICMSResolved
	}
	pFCP := cfgStr(cfg, "icms_fcp_override", pFCPResolved)
	if pFCP == "" {
		pFCP = pFCPResolved
	}
	pRedBC := cfgStr(cfg, "icms_p_red_bc", "0.00")
	motDes := cfgStrPtr(cfg, "icms_mot_des")
	pDif := cfgStr(cfg, "icms_p_dif", "0.00")
	indDeduz := cfgStr(cfg, "icms_ind_deduz_deson", "0")
	modBCST := cfgStr(cfg, "icms_st_mod_bc", "4")
	pMVAST := cfgStr(cfg, "icms_st_mva", "0.00")
	pRedBCST := cfgStr(cfg, "icms_st_red_bc", "0.00")
	pICMSST := cfgStr(cfg, "icms_st_aliq", "0.00")
	pFCPST := cfgStr(cfg, "icms_st_fcp_aliq", "0.00")
	hasFCP := pFCP != "" && pFCP != "0.00"
	hasFCPST := pFCPST != "" && pFCPST != "0.00"

	pRedBCd := decimal.Zero
	if pRedBC != "" && pRedBC != "0.00" {
		pRedBCd = d(pRedBC)
	}
	vBCd := vProd.Mul(decimal.NewFromInt(1).Sub(pRedBCd.Div(decimal.NewFromInt(100)))).RoundBank(2)
	vICMSd := vBCd.Mul(d(pICMS)).Div(decimal.NewFromInt(100)).RoundBank(2)

	pRedBCSTd := decimal.Zero
	if pRedBCST != "" && pRedBCST != "0.00" {
		pRedBCSTd = d(pRedBCST)
	}
	vBCSTd := vProd.Mul(decimal.NewFromInt(1).Add(d(pMVAST).Div(decimal.NewFromInt(100)))).
		Mul(decimal.NewFromInt(1).Sub(pRedBCSTd.Div(decimal.NewFromInt(100)))).RoundBank(2)
	vICMSSTd := decimal.Max(
		vBCSTd.Mul(d(pICMSST)).Div(decimal.NewFromInt(100)).Sub(vICMSd).RoundBank(2),
		decimal.Zero,
	)

	vFCPd := decimal.Zero
	if hasFCP {
		vFCPd = vBCd.Mul(d(pFCP)).Div(decimal.NewFromInt(100)).RoundBank(2)
	}
	vFCPSTd := decimal.Zero
	if hasFCPST {
		vFCPSTd = vBCSTd.Mul(d(pFCPST)).Div(decimal.NewFromInt(100)).RoundBank(2)
	}

	vBC := q2(vBCd)
	vICMS := q2(vICMSd)
	vBCST := q2(vBCSTd)
	vICMSST := q2(vICMSSTd)
	vFCP := q2(vFCPd)
	vFCPST := q2(vFCPSTd)

	addDeson := func(nd map[string]any) map[string]any {
		if motDes != nil {
			nd["vICMSDeson"] = vICMS
			nd["motDesICMS"] = *motDes
			nd["indDeduzDeson"] = indDeduz
		}
		return nd
	}
	addFCP := func(nd map[string]any) map[string]any {
		if hasFCP {
			nd["vBCFCP"] = vBC
			nd["pFCP"] = pFCP
			nd["vFCP"] = vFCP
		}
		return nd
	}
	// addSTDeson: ST desonerada. vICMSSTDeson é o ICMS-ST que deixou de ser
	// cobrado — o próprio vICMSST calculado —, e motDesICMSST diz por quê.
	addSTDeson := func(nd map[string]any) map[string]any {
		if mot := cfgStrPtr(cfg, "icms_mot_des_st"); mot != nil && *mot != "" {
			nd["vICMSSTDeson"] = vICMSST
			nd["motDesICMSST"] = *mot
		}
		return nd
	}
	// addFCPDif: FCP diferido. vFCPDif é a parcela diferida e vFCPEfet o que
	// sobra a recolher.
	addFCPDif := func(nd map[string]any) map[string]any {
		pFCPDif := cfgStrPtr(cfg, "icms_p_fcp_dif")
		if pFCPDif == nil || *pFCPDif == "" || !hasFCP {
			return nd
		}
		vDif := vFCPd.Mul(d(*pFCPDif)).Div(decimal.NewFromInt(100)).RoundBank(2)
		nd["pFCPDif"] = *pFCPDif
		nd["vFCPDif"] = q2(vDif)
		nd["vFCPEfet"] = q2(vFCPd.Sub(vDif).RoundBank(2))
		return nd
	}
	addFCPST := func(nd map[string]any) map[string]any {
		if hasFCPST {
			nd["vBCFCPST"] = vBCST
			nd["pFCPST"] = pFCPST
			nd["vFCPST"] = vFCPST
		}
		return nd
	}

	// ICMSPart substitui ICMS10/ICMS90 quando há partilha do ICMS entre a UF de
	// origem e a de destino. O par pBCOp+UFST é o que distingue os dois casos.
	if pBCOp := cfgStrPtr(cfg, "icms_part_p_bc_op"); pBCOp != nil && *pBCOp != "" && icmsPartCSTs[cst] {
		if ufST := cfgStrPtr(cfg, "icms_part_uf_st"); ufST != nil && *ufST != "" {
			node := map[string]any{
				"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC,
				"pRedBC": pRedBC, "pICMS": pICMS, "vICMS": vICMS,
				"modBCST": modBCST, "pMVAST": pMVAST, "pRedBCST": pRedBCST,
				"vBCST": vBCST, "pICMSST": pICMSST, "vICMSST": vICMSST,
				"pBCOp": *pBCOp, "UFST": *ufST,
			}
			addFCPST(node)
			addDeson(node)
			return map[string]any{"ICMSPart": node}
		}
	}

	switch cst {
	case "00":
		node := map[string]any{"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC, "pICMS": pICMS, "vICMS": vICMS}
		// ICMS00 é o único grupo de FCP sem vBCFCP no leiaute: a base é o
		// próprio vBC, então o XSD só traz pFCP e vFCP.
		if hasFCP {
			node["pFCP"] = pFCP
			node["vFCP"] = vFCP
		}
		return map[string]any{"ICMS00": node}

	case "02":
		adRem := cfgStr(cfg, "icms_ad_rem", "0.0000")
		qBCMono := cfgStr(cfg, "icms_q_bc_mono", q4(qty.RoundBank(4)))
		vICMSMono := q2(d(qBCMono).Mul(d(adRem)).RoundBank(2))
		node := map[string]any{"orig": origin, "CST": cst, "adRemICMS": adRem, "vICMSMono": vICMSMono}
		if qBCMono != "" {
			node["qBCMono"] = qBCMono
		}
		return map[string]any{"ICMS02": node}

	case "10":
		node := map[string]any{
			"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC, "pICMS": pICMS, "vICMS": vICMS,
			"modBCST": modBCST, "pMVAST": pMVAST, "pRedBCST": pRedBCST, "vBCST": vBCST,
			"pICMSST": pICMSST, "vICMSST": vICMSST,
		}
		addFCP(node)
		addFCPST(node)
		addSTDeson(node)
		return map[string]any{"ICMS10": node}

	case "15":
		adRem := cfgStr(cfg, "icms_ad_rem", "0.0000")
		adRemReten := cfgStr(cfg, "icms_ad_rem_reten", "0.0000")
		qBCMono := cfgStr(cfg, "icms_q_bc_mono", q4(qty.RoundBank(4)))
		vICMSMono := q2(d(qBCMono).Mul(d(adRem)).RoundBank(2))
		vICMSMonoReten := q2(d(qBCMono).Mul(d(adRemReten)).RoundBank(2))
		node := map[string]any{
			"orig": origin, "CST": cst,
			"adRemICMS": adRem, "vICMSMono": vICMSMono,
			"adRemICMSReten": adRemReten, "vICMSMonoReten": vICMSMonoReten,
		}
		if qBCMono != "" {
			node["qBCMono"] = qBCMono
		}
		if pRed := cfgStrPtr(cfg, "icms_p_red_ad_rem"); pRed != nil {
			if motRed := cfgStrPtr(cfg, "icms_mot_red_ad_rem"); motRed != nil {
				node["pRedAdRem"] = *pRed
				node["motRedAdRem"] = *motRed
			}
		}
		return map[string]any{"ICMS15": node}

	case "20":
		node := map[string]any{
			"orig": origin, "CST": cst, "modBC": modBC, "pRedBC": pRedBC, "vBC": vBC, "pICMS": pICMS, "vICMS": vICMS,
		}
		addFCP(node)
		addDeson(node)
		return map[string]any{"ICMS20": node}

	case "30":
		node := map[string]any{
			"orig": origin, "CST": cst, "modBCST": modBCST, "pMVAST": pMVAST,
			"pRedBCST": pRedBCST, "vBCST": vBCST, "pICMSST": pICMSST, "vICMSST": vICMSST,
		}
		addFCPST(node)
		addDeson(node)
		return map[string]any{"ICMS30": node}

	case "40", "41", "50":
		// ICMSST (CST 41) é o repasse, na operação interestadual, do ICMS-ST já
		// retido antes. Sem os valores retidos, 41 é apenas não tributada.
		if cst == "41" {
			if vBCSTRet := cfgStrPtr(cfg, "icms_v_bc_st_ret"); vBCSTRet != nil && *vBCSTRet != "" {
				node := map[string]any{"orig": origin, "CST": cst}
				addSTRetida(node, cfg)
				if v := cfgStrPtr(cfg, "icms_v_bc_st_dest"); v != nil && *v != "" {
					node["vBCSTDest"] = *v
					node["vICMSSTDest"] = cfgStr(cfg, "icms_v_icms_st_dest", "0.00")
				}
				addICMSEfetivo(node, vProd, cfg)
				return map[string]any{"ICMSST": node}
			}
		}
		return map[string]any{"ICMS40": addDeson(map[string]any{"orig": origin, "CST": cst})}

	case "51":
		pDifd := decimal.Zero
		if pDif != "" && pDif != "0.00" {
			pDifd = d(pDif)
		}
		vICMSOp := vICMSd
		vICMSDif := vICMSOp.Mul(pDifd).Div(decimal.NewFromInt(100)).RoundBank(2)
		vICMSEfetivo := vICMSOp.Sub(vICMSDif).RoundBank(2)
		node := map[string]any{
			"orig": origin, "CST": cst, "modBC": modBC, "pRedBC": pRedBC,
			"vBC": vBC, "pICMS": pICMS, "vICMSOp": q2(vICMSOp), "pDif": pDif,
			"vICMSDif": q2(vICMSDif), "vICMS": q2(vICMSEfetivo),
		}
		addFCP(node)
		addFCPDif(node)
		return map[string]any{"ICMS51": node}

	case "53":
		adRem := cfgStr(cfg, "icms_ad_rem", "0.0000")
		pDifMono := cfgStr(cfg, "icms_p_dif_mono", "0.0000")
		qBCMono := cfgStr(cfg, "icms_q_bc_mono", q4(qty.RoundBank(4)))
		vICMSMonoOp := q2(d(qBCMono).Mul(d(adRem)).RoundBank(2))
		vICMSMonoDif := q2(d(vICMSMonoOp).Mul(d(pDifMono)).Div(decimal.NewFromInt(100)).RoundBank(2))
		vICMSMonoVal := q2(d(vICMSMonoOp).Sub(d(vICMSMonoDif)).RoundBank(2))
		node := map[string]any{"orig": origin, "CST": cst}
		if qBCMono != "" {
			node["qBCMono"] = qBCMono
		}
		node["adRemICMS"] = adRem
		node["vICMSMonoOp"] = vICMSMonoOp
		if pDifMono != "" && pDifMono != "0.0000" {
			node["pDif"] = pDifMono
			node["vICMSMonoDif"] = vICMSMonoDif
		}
		node["vICMSMono"] = vICMSMonoVal
		return map[string]any{"ICMS53": node}

	case "60":
		node := map[string]any{"orig": origin, "CST": cst}
		addSTRetida(node, cfg)
		addICMSEfetivo(node, vProd, cfg)
		return map[string]any{"ICMS60": node}

	case "61":
		adRemRet := cfgStr(cfg, "icms_ad_rem", "0.0000")
		qBCMonoRet := cfgStr(cfg, "icms_q_bc_mono", q4(qty.RoundBank(4)))
		vICMSMonoRet := q2(d(qBCMonoRet).Mul(d(adRemRet)).RoundBank(2))
		node := map[string]any{
			"orig": origin, "CST": cst, "adRemICMSRet": adRemRet, "vICMSMonoRet": vICMSMonoRet,
		}
		if qBCMonoRet != "" {
			node["qBCMonoRet"] = qBCMonoRet
		}
		return map[string]any{"ICMS61": node}

	case "70":
		node := map[string]any{
			"orig": origin, "CST": cst, "modBC": modBC, "pRedBC": pRedBC,
			"vBC": vBC, "pICMS": pICMS, "vICMS": vICMS,
			"modBCST": modBCST, "pMVAST": pMVAST, "pRedBCST": pRedBCST,
			"vBCST": vBCST, "pICMSST": pICMSST, "vICMSST": vICMSST,
		}
		addFCP(node)
		addFCPST(node)
		addDeson(node)
		addSTDeson(node)
		return map[string]any{"ICMS70": node}

	case "90":
		node := map[string]any{"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC}
		if pRedBC != "" && pRedBC != "0.00" {
			node["pRedBC"] = pRedBC
		}
		node["pICMS"] = pICMS
		node["vICMS"] = vICMS
		addFCP(node)
		addFCPDif(node)
		addDeson(node)
		addSTDeson(node)
		return map[string]any{"ICMS90": node}
	}

	return map[string]any{"ICMS40": map[string]any{"orig": origin, "CST": "40"}}
}

// ─── IPI ─────────────────────────────────────────────────────────────────────

var ipiCSTTributado = map[string]bool{"50": true, "51": true}

func buildIPI(ipiCST string, vProd, qty decimal.Decimal, cfg, item map[string]any) map[string]any {
	if ipiCST == "" {
		return nil
	}
	// cEnq é o enquadramento legal do IPI: vem do produto, com o genérico 999
	// como default (é o que a Receita aceita quando não há enquadramento).
	node := map[string]any{}
	if v := anyStrPtr(item, "ipi_cnpj_prod"); v != nil && *v != "" {
		node["CNPJProd"] = *v
	}
	if v := anyStrPtr(item, "ipi_c_selo"); v != nil && *v != "" {
		node["cSelo"] = *v
		if q := anyStrPtr(item, "ipi_q_selo"); q != nil && *q != "" {
			node["qSelo"] = *q
		}
	}
	node["cEnq"] = anyStr(item, "ipi_c_enq", "999")

	if !ipiCSTTributado[ipiCST] {
		node["IPINT"] = map[string]any{"CST": ipiCST}
		return map[string]any{"IPI": node}
	}

	// vBC+pIPI e qUnid+vUnid são choice no XSD: o IPI é ad valorem ou por
	// unidade, nunca os dois. vUnid configurado é o que decide.
	trib := map[string]any{"CST": ipiCST}
	if vUnid := cfgStrPtr(cfg, "ipi_v_unid"); vUnid != nil && *vUnid != "" {
		qUnid := q4(qty.RoundBank(4))
		trib["qUnid"] = qUnid
		trib["vUnid"] = *vUnid
		trib["vIPI"] = q2(d(qUnid).Mul(d(*vUnid)).RoundBank(2))
	} else {
		aliq := cfgStr(cfg, "ipi_aliq", "0.0000")
		trib["vBC"] = q2(vProd.RoundBank(2))
		trib["pIPI"] = aliq
		trib["vIPI"] = q2(vProd.Mul(d(aliq)).Div(decimal.NewFromInt(100)).RoundBank(2))
	}
	node["IPITrib"] = trib
	return map[string]any{"IPI": node}
}

// buildPISCOFINSST monta PISST/COFINSST, que têm estrutura idêntica e só
// diferem nos nomes das tags. Base própria (v_bc) quando informada; senão o
// valor do produto. O modo por quantidade (qBCProd+vAliqProd) fica de fora até
// haver caso concreto — o XSD é choice e adivinhar o ramo é pior que não emitir.
func buildPISCOFINSST(cfg map[string]any, vProd decimal.Decimal, aliqKey, vbcKey, pTag, vTag string) map[string]any {
	aliq := cfgStrPtr(cfg, aliqKey)
	if aliq == nil || *aliq == "" {
		return nil
	}
	vBC := vProd.RoundBank(2)
	if v := cfgStrPtr(cfg, vbcKey); v != nil && *v != "" {
		vBC = d(*v)
	}
	return map[string]any{
		"vBC": q2(vBC), pTag: *aliq,
		vTag: q2(vBC.Mul(d(*aliq)).Div(decimal.NewFromInt(100)).RoundBank(2)),
	}
}

func buildPISST(cfg map[string]any, vProd decimal.Decimal) map[string]any {
	return buildPISCOFINSST(cfg, vProd, "pis_st_aliq", "pis_st_v_bc", "pPIS", "vPIS")
}

func buildCOFINSST(cfg map[string]any, vProd decimal.Decimal) map[string]any {
	return buildPISCOFINSST(cfg, vProd, "cofins_st_aliq", "cofins_st_v_bc", "pCOFINS", "vCOFINS")
}

// ─── IS ──────────────────────────────────────────────────────────────────────

func buildIS(isCST string, vProd decimal.Decimal, isAliq *string, cfg map[string]any) map[string]any {
	if isCST == "" {
		return nil
	}
	classTrib := cfgStr(cfg, "is_class_trib", "")
	aliqEspec := cfgStrPtr(cfg, "is_aliq_espec")
	unidTrib := cfgStrPtr(cfg, "is_unid_trib")

	node := map[string]any{"CSTIS": isCST}
	if classTrib != "" {
		node["cClassTribIS"] = classTrib
	}

	if isCST == "000" {
		aliq := "0.0000"
		if isAliq != nil && *isAliq != "" {
			aliq = *isAliq
		}
		vIS := q2(vProd.Mul(d(aliq)).Div(decimal.NewFromInt(100)).RoundBank(2))
		node["vBCIS"] = q2(vProd.RoundBank(2))
		node["pIS"] = aliq
		if aliqEspec != nil {
			node["adRemIS"] = *aliqEspec
		}
		if unidTrib != nil {
			node["uTrib"] = *unidTrib
			node["qTrib"] = "0.0000"
		}
		node["vIS"] = vIS
	}
	return map[string]any{"IS": node}
}

// ─── PIS ─────────────────────────────────────────────────────────────────────

var pisOutrCSTs = map[string]bool{
	"49": true, "50": true, "51": true, "52": true, "53": true, "54": true,
	"55": true, "56": true, "60": true, "61": true, "62": true, "63": true,
	"64": true, "65": true, "66": true, "67": true, "70": true, "71": true,
	"72": true, "73": true, "74": true, "75": true, "98": true, "99": true,
}

func buildPIS(cst string, vProd decimal.Decimal, aliq, aliqUnid *string) map[string]any {
	switch cst {
	case "04", "05", "06", "07", "08", "09":
		return map[string]any{"PISNT": map[string]any{"CST": cst}}
	case "03":
		aUnid := "0.0000"
		if aliqUnid != nil {
			aUnid = *aliqUnid
		}
		return map[string]any{"PISQtde": map[string]any{
			"CST": cst, "qBCProd": "0.0000", "vAliqProd": aUnid, "vPIS": "0.00",
		}}
	}
	pPIS := "0.00"
	if aliq != nil && *aliq != "" {
		pPIS = *aliq
	}
	vBC := q2(vProd.RoundBank(2))
	vPIS := q2(vProd.Mul(d(pPIS)).Div(decimal.NewFromInt(100)).RoundBank(2))
	if pisOutrCSTs[cst] {
		return map[string]any{"PISOutr": map[string]any{"CST": cst, "vBC": vBC, "pPIS": pPIS, "vPIS": vPIS}}
	}
	return map[string]any{"PISAliq": map[string]any{"CST": cst, "vBC": vBC, "pPIS": pPIS, "vPIS": vPIS}}
}

// ─── COFINS ───────────────────────────────────────────────────────────────────

func buildCOFINS(cst string, vProd decimal.Decimal, aliq, aliqUnid *string) map[string]any {
	switch cst {
	case "04", "05", "06", "07", "08", "09":
		return map[string]any{"COFINSNT": map[string]any{"CST": cst}}
	case "03":
		aUnid := "0.0000"
		if aliqUnid != nil {
			aUnid = *aliqUnid
		}
		return map[string]any{"COFINSQtde": map[string]any{
			"CST": cst, "qBCProd": "0.0000", "vAliqProd": aUnid, "vCOFINS": "0.00",
		}}
	}
	pCOFINS := "0.00"
	if aliq != nil && *aliq != "" {
		pCOFINS = *aliq
	}
	vBC := q2(vProd.RoundBank(2))
	vCOFINS := q2(vProd.Mul(d(pCOFINS)).Div(decimal.NewFromInt(100)).RoundBank(2))
	if pisOutrCSTs[cst] {
		return map[string]any{"COFINSOutr": map[string]any{"CST": cst, "vBC": vBC, "pCOFINS": pCOFINS, "vCOFINS": vCOFINS}}
	}
	return map[string]any{"COFINSAliq": map[string]any{"CST": cst, "vBC": vBC, "pCOFINS": pCOFINS, "vCOFINS": vCOFINS}}
}

// ─── IBS / CBS ────────────────────────────────────────────────────────────────

var ibsCBSExempt = map[string]bool{
	"400": true, "410": true, "800": true, "810": true, "811": true, "820": true,
}

func calcTaxValue(vBC decimal.Decimal, pAliq string) string {
	return q2(vBC.Mul(d(pAliq)).Div(decimal.NewFromInt(100)).RoundBank(2))
}

func buildGIBSCBS(cst, classTrib string, vBC decimal.Decimal, ibsUFAliq, ibsMunAliq, cbsAliq string, cfg map[string]any) map[string]any {
	outer := map[string]any{"CST": cst}
	if classTrib != "" {
		outer["cClassTrib"] = classTrib
	}
	if indDoacao := cfgStrPtr(cfg, "ibs_ind_doacao"); indDoacao != nil && (*indDoacao == "S" || *indDoacao == "N") {
		outer["indDoacao"] = *indDoacao
	}

	if ibsCBSExempt[cst] {
		return outer
	}

	if cst == "620" {
		adRemIBS := cfgStr(cfg, "ibs_ad_rem", "0.0000")
		adRemCBS := cfgStr(cfg, "cbs_ad_rem", "0.0000")
		outer["gIBSCBSMono"] = map[string]any{
			"gMonoPadrao": map[string]any{
				"qBCMono": "0.0000", "adRemIBS": adRemIBS, "adRemCBS": adRemCBS,
				"vIBSMono": "0.00", "vCBSMono": "0.00",
			},
		}
		return outer
	}

	vBCStr := q2(vBC.RoundBank(2))
	vIBSUF := d(calcTaxValue(vBC, ibsUFAliq))
	vIBSMun := d(calcTaxValue(vBC, ibsMunAliq))

	gDif := func(pDif *string) map[string]any {
		if pDif == nil {
			return nil
		}
		return map[string]any{"pDif": *pDif, "vDif": "0.00"}
	}
	gRed := func(aliq, pRed *string) map[string]any {
		if pRed == nil || aliq == nil {
			return nil
		}
		eff := q4(d(*aliq).Mul(decimal.NewFromInt(1).Sub(d(*pRed).Div(decimal.NewFromInt(100)))).RoundBank(4))
		return map[string]any{"pRedAliq": *pRed, "pAliqEfet": eff}
	}

	gIBSUF := map[string]any{"pIBSUF": ibsUFAliq}
	if v := gDif(cfgStrPtr(cfg, "ibs_uf_p_dif")); v != nil {
		gIBSUF["gDif"] = v
	}
	if v := gRed(&ibsUFAliq, cfgStrPtr(cfg, "ibs_uf_p_red")); v != nil {
		gIBSUF["gRed"] = v
	}
	gIBSUF["vIBSUF"] = q2(vIBSUF.RoundBank(2))

	gIBSMun := map[string]any{"pIBSMun": ibsMunAliq}
	if v := gDif(cfgStrPtr(cfg, "ibs_mun_p_dif")); v != nil {
		gIBSMun["gDif"] = v
	}
	if v := gRed(&ibsMunAliq, cfgStrPtr(cfg, "ibs_mun_p_red")); v != nil {
		gIBSMun["gRed"] = v
	}
	gIBSMun["vIBSMun"] = q2(vIBSMun.RoundBank(2))

	gCBS := map[string]any{"pCBS": cbsAliq}
	if v := gDif(cfgStrPtr(cfg, "cbs_p_dif")); v != nil {
		gCBS["gDif"] = v
	}
	if v := gRed(&cbsAliq, cfgStrPtr(cfg, "cbs_p_red")); v != nil {
		gCBS["gRed"] = v
	}
	gCBS["vCBS"] = calcTaxValue(vBC, cbsAliq)

	outer["gIBSCBS"] = map[string]any{
		"vBC":     vBCStr,
		"gIBSUF":  gIBSUF,
		"gIBSMun": gIBSMun,
		"vIBS":    q2(vIBSUF.Add(vIBSMun).RoundBank(2)),
		"gCBS":    gCBS,
	}
	return outer
}

// ─── ISSQN ────────────────────────────────────────────────────────────────────

func buildISSQN(vBC decimal.Decimal, cfg map[string]any) map[string]any {
	if cfg == nil {
		return nil
	}
	indISS := cfgStrPtr(cfg, "issqn_ind_iss")
	if indISS == nil {
		return nil
	}
	aliq := cfgStr(cfg, "issqn_aliq", "0.00")
	vBCd := vBC.RoundBank(2)
	vDeducao := decimal.Zero
	if vDedStr := cfgStrPtr(cfg, "issqn_v_deducao"); vDedStr != nil {
		vDeducao = d(*vDedStr)
	}
	vBCEfetiva := decimal.Max(vBCd.Sub(vDeducao), decimal.Zero)
	vISSQN := q2(vBCEfetiva.Mul(d(aliq)).Div(decimal.NewFromInt(100)).RoundBank(2))

	node := map[string]any{
		"vBC":       q2(vBCd),
		"vAliq":     aliq,
		"vISSQN":    vISSQN,
		"cMunFG":    cfgStr(cfg, "issqn_c_mun_fg", "0000000"),
		"cListServ": cfgStr(cfg, "issqn_c_list_serv", ""),
		"indISS":    *indISS,
	}
	if vDedStr := cfgStrPtr(cfg, "issqn_v_deducao"); vDedStr != nil {
		node["vDeducao"] = *vDedStr
	}
	if vISSRet := cfgStrPtr(cfg, "issqn_v_iss_ret"); vISSRet != nil {
		node["vISSRet"] = *vISSRet
	}
	return map[string]any{"ISSQN": node}
}

// ─── DIFAL ────────────────────────────────────────────────────────────────────

// icmsPartCSTs são os CSTs em que a partilha do ICMS (ICMSPart) pode ocorrer.
// Não há CST próprio para ela: o que distingue é o par pBCOp + UFST.
var icmsPartCSTs = map[string]bool{"10": true, "90": true}

var icmsCSTDifalEligible = map[string]bool{
	"00": true, "20": true, "51": true, "70": true, "90": true,
}

func buildICMSUFDest(vBC decimal.Decimal, pICMSUFDest, pICMSInter, pFCPUFDest string) map[string]any {
	diff := d(pICMSUFDest).Sub(d(pICMSInter))
	vICMSUFDest := decimal.Max(vBC.Mul(diff).Div(decimal.NewFromInt(100)).RoundBank(2), decimal.Zero)
	node := map[string]any{
		"vBCUFDest":      q2(vBC.RoundBank(2)),
		"pICMSUFDest":    pICMSUFDest,
		"pICMSInter":     pICMSInter,
		"pICMSInterPart": "100.00",
		"vICMSUFDest":    q2(vICMSUFDest),
		"vICMSUFRemet":   "0.00",
	}
	if pFCPUFDest != "" && pFCPUFDest != "0.00" && pFCPUFDest != "0.0" && pFCPUFDest != "0" {
		vFCP := q2(vBC.Mul(d(pFCPUFDest)).Div(decimal.NewFromInt(100)).RoundBank(2))
		node["vBCFCPUFDest"] = q2(vBC.RoundBank(2))
		node["pFCPUFDest"] = pFCPUFDest
		node["vFCPUFDest"] = vFCP
	}
	return node
}

// ─── UF rules ─────────────────────────────────────────────────────────────────

func applyUFRules(emitUF, cst string, cfopEntry map[string]any, pICMSResolved string) map[string]any {
	if emitUF == "RJ" && cst == "40" {
		if cfgStrPtr(cfopEntry, "icms_mot_des") == nil {
			result := make(map[string]any, len(cfopEntry)+3)
			for k, v := range cfopEntry {
				result[k] = v
			}
			result["icms_mot_des"] = "9"
			if cfgStrPtr(cfopEntry, "icms_aliq_override") == nil {
				result["icms_aliq_override"] = pICMSResolved
			}
			if cfgStrPtr(cfopEntry, "icms_ind_deduz_deson") == nil {
				result["icms_ind_deduz_deson"] = "1"
			}
			return result
		}
	}
	return cfopEntry
}

// addICMSEfetivo acrescenta o grupo do ICMS efetivo (pRedBCEfet, vBCEfet,
// pICMSEfet, vICMSEfet), exigido por algumas UFs na revenda de mercadoria com
// ST retida. Vale para ICMS60, ICMSST e ICMSSN500 — a mesma quádrupla, com o
// mesmo cálculo, nos três; por isso uma função e não três cópias.
func addICMSEfetivo(node map[string]any, vProd decimal.Decimal, cfg map[string]any) {
	pICMSEfet := cfgStrPtr(cfg, "icms_p_icms_efet")
	if pICMSEfet == nil || *pICMSEfet == "" {
		return
	}
	pRed := cfgStr(cfg, "icms_p_red_bc_efet", "0.00")
	vBCEfet := vProd.Mul(decimal.NewFromInt(1).Sub(d(pRed).Div(decimal.NewFromInt(100)))).RoundBank(2)
	if pRed != "" && pRed != "0.00" {
		node["pRedBCEfet"] = pRed
	}
	node["vBCEfet"] = q2(vBCEfet)
	node["pICMSEfet"] = *pICMSEfet
	node["vICMSEfet"] = q2(vBCEfet.Mul(d(*pICMSEfet)).Div(decimal.NewFromInt(100)).RoundBank(2))
}

// addSTRetida acrescenta os valores do ICMS-ST retido anteriormente
// (vBCSTRet, pST, vICMSSTRet e o FCP-ST retido). São os mesmos campos, com o
// mesmo significado, em ICMS60, ICMSST e ICMSSN500.
func addSTRetida(node map[string]any, cfg map[string]any) {
	if vBCSTRet := cfgStrPtr(cfg, "icms_v_bc_st_ret"); vBCSTRet != nil && *vBCSTRet != "" {
		node["vBCSTRet"] = *vBCSTRet
		if pST := cfgStrPtr(cfg, "icms_p_st"); pST != nil && *pST != "" {
			node["pST"] = *pST
		}
		node["vICMSSTRet"] = cfgStr(cfg, "icms_v_icms_st_ret", "0.00")
	}
	if vBCFCPSTRet := cfgStrPtr(cfg, "icms_fcp_v_bc_st_ret"); vBCFCPSTRet != nil && *vBCFCPSTRet != "" {
		pFCPSTRet := cfgStr(cfg, "icms_fcp_st_ret_aliq", "0.00")
		node["vBCFCPSTRet"] = *vBCFCPSTRet
		node["pFCPSTRet"] = pFCPSTRet
		node["vFCPSTRet"] = q2(d(*vBCFCPSTRet).Mul(d(pFCPSTRet)).Div(decimal.NewFromInt(100)).RoundBank(2))
	}
}
