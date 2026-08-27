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
		return map[string]any{"ICMSSN500": map[string]any{"orig": origin, "CSOSN": csosn}}
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
	addFCPST := func(nd map[string]any) map[string]any {
		if hasFCPST {
			nd["vBCFCPST"] = vBCST
			nd["pFCPST"] = pFCPST
			nd["vFCPST"] = vFCPST
		}
		return nd
	}

	switch cst {
	case "00":
		node := map[string]any{"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC, "pICMS": pICMS, "vICMS": vICMS}
		return map[string]any{"ICMS00": addFCP(node)}

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
		return map[string]any{"ICMS40": addDeson(map[string]any{"orig": origin, "CST": cst})}

	case "51":
		pDifd := decimal.Zero
		if pDif != "" && pDif != "0.00" {
			pDifd = d(pDif)
		}
		vICMSOp := vICMSd
		vICMSDif := vICMSOp.Mul(pDifd).Div(decimal.NewFromInt(100)).RoundBank(2)
		vICMSEfetivo := vICMSOp.Sub(vICMSDif).RoundBank(2)
		return map[string]any{"ICMS51": map[string]any{
			"orig": origin, "CST": cst, "modBC": modBC, "pRedBC": pRedBC,
			"vBC": vBC, "pICMS": pICMS, "vICMSOp": q2(vICMSOp), "pDif": pDif,
			"vICMSDif": q2(vICMSDif), "vICMS": q2(vICMSEfetivo),
		}}

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
		if vBCSTRet := cfgStrPtr(cfg, "icms_v_bc_st_ret"); vBCSTRet != nil {
			if vICMSSTRet := cfgStrPtr(cfg, "icms_v_icms_st_ret"); vICMSSTRet != nil {
				node["vBCSTRet"] = *vBCSTRet
				if pST := cfgStrPtr(cfg, "icms_p_st"); pST != nil {
					node["pST"] = *pST
				}
				node["vICMSSTRet"] = *vICMSSTRet
			}
		}
		if vBCFCPSTRet := cfgStrPtr(cfg, "icms_fcp_v_bc_st_ret"); vBCFCPSTRet != nil {
			if pFCPSTRet := cfgStrPtr(cfg, "icms_fcp_st_ret_aliq"); pFCPSTRet != nil {
				vFCPRet := q2(d(*vBCFCPSTRet).Mul(d(*pFCPSTRet)).Div(decimal.NewFromInt(100)).RoundBank(2))
				node["vBCFCPSTRet"] = *vBCFCPSTRet
				node["pFCPSTRet"] = *pFCPSTRet
				node["vFCPSTRet"] = vFCPRet
			}
		}
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
		return map[string]any{"ICMS70": node}

	case "90":
		node := map[string]any{"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC}
		if pRedBC != "" && pRedBC != "0.00" {
			node["pRedBC"] = pRedBC
		}
		node["pICMS"] = pICMS
		node["vICMS"] = vICMS
		addFCP(node)
		addDeson(node)
		return map[string]any{"ICMS90": node}
	}

	return map[string]any{"ICMS40": map[string]any{"orig": origin, "CST": "40"}}
}

// ─── IPI ─────────────────────────────────────────────────────────────────────

var ipiCSTTributado = map[string]bool{"50": true, "51": true}

func buildIPI(ipiCST string, vProd decimal.Decimal, ipiAliq *string) map[string]any {
	if ipiCST == "" {
		return nil
	}
	aliq := "0.0000"
	if ipiAliq != nil && *ipiAliq != "" {
		aliq = *ipiAliq
	}
	if ipiCSTTributado[ipiCST] {
		vIPI := q2(vProd.Mul(d(aliq)).Div(decimal.NewFromInt(100)).RoundBank(2))
		return map[string]any{"IPI": map[string]any{
			"cEnq": "999",
			"IPITrib": map[string]any{
				"CST":  ipiCST,
				"vBC":  q2(vProd.RoundBank(2)),
				"pIPI": aliq,
				"vIPI": vIPI,
			},
		}}
	}
	return map[string]any{"IPI": map[string]any{"cEnq": "999", "IPINT": map[string]any{"CST": ipiCST}}}
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
