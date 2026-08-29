package nfes

// builders_total.go — acumuladores por item e o nó total da NF-e (ICMSTot,
// IBSCBSTot e ISSQNtot). Extraído de builders_doc.go.

import (
	"time"

	"github.com/shopspring/decimal"
)

// totals agrupa os acumuladores por item que viram o nó total.
type totals struct {
	VBC, VICMS, VICMSDeson, VFCP, VBCST, VICMSST, VFCPST decimal.Decimal
	VPIS, VCOFINS, VIPI, VFrete, VSeg, VOutro            decimal.Decimal
	VICMSUFDest, VFCPUFDest                              decimal.Decimal
	VServ, VBCISSQN, VISSQN, VPISISSQN, VCOFINSISSQN     decimal.Decimal
	// Restante do ISSQNtot: deduções, descontos e ISS retido, somados dos itens.
	VDeducaoISSQN, VOutroISSQN                decimal.Decimal
	VDescIncondISSQN, VDescCondISSQN, VISSRet decimal.Decimal
	// RegTribISSQN é o regime especial de tributação do prestador (cRegTrib).
	RegTribISSQN                          string
	IBSBC, IBSUF, IBSMun, IBS, CBSBC, CBS decimal.Decimal
	// Reforma tributária: cada grupo novo do item tem seu acumulador no total.
	// Diferimento e devolução de tributo, por esfera.
	IBSUFDif, IBSMunDif, CBSDif             decimal.Decimal
	IBSUFDevTrib, IBSMunDevTrib, CBSDevTrib decimal.Decimal
	// Crédito presumido, separando o crédito do crédito com condição suspensiva
	// (o choice do XSD) em cada esfera.
	IBSCredPres, IBSCredPresCondSus decimal.Decimal
	CBSCredPres, CBSCredPresCondSus decimal.Decimal
	// Monofasia: padrão, com retenção e já retido — os três somam em gMono.
	IBSMono, CBSMono           decimal.Decimal
	IBSMonoReten, CBSMonoReten decimal.Decimal
	IBSMonoRet, CBSMonoRet     decimal.Decimal
	// Estorno de crédito.
	IBSEstCred, CBSEstCred decimal.Decimal
	// VIS é o Imposto Seletivo somado dos itens (total/ISTot).
	VIS decimal.Decimal
	// HasIBSCBSMono e HasEstornoCred marcam a presença dos sub-grupos
	// opcionais do total: gMono e gEstornoCred só existem quando algum item os
	// trouxe — nó vazio é rejeição.
	HasIBSCBSMono, HasEstornoCred bool
	Products, Discount            decimal.Decimal
	// VIPIDevol é o IPI devolvido (impostoDevol) somado dos itens; VII é o
	// imposto de importação somado.
	VIPIDevol, VII decimal.Decimal
	HasISSQN       bool
}

// newTotals devolve um acumulador zerado com os totais de produto e desconto
// que já vêm calculados da requisição.
func newTotals(products, discount decimal.Decimal) totals {
	z := decimal.Zero
	return totals{
		VBC: z, VICMS: z, VICMSDeson: z, VFCP: z, VBCST: z, VICMSST: z, VFCPST: z,
		VPIS: z, VCOFINS: z, VIPI: z, VFrete: z, VSeg: z, VOutro: z,
		VICMSUFDest: z, VFCPUFDest: z,
		VServ: z, VBCISSQN: z, VISSQN: z, VPISISSQN: z, VCOFINSISSQN: z,
		VDeducaoISSQN: z, VOutroISSQN: z, VDescIncondISSQN: z, VDescCondISSQN: z, VISSRet: z,
		IBSBC: z, IBSUF: z, IBSMun: z, IBS: z, CBSBC: z, CBS: z,
		IBSUFDif: z, IBSMunDif: z, CBSDif: z,
		IBSUFDevTrib: z, IBSMunDevTrib: z, CBSDevTrib: z,
		IBSCredPres: z, IBSCredPresCondSus: z, CBSCredPres: z, CBSCredPresCondSus: z,
		IBSMono: z, CBSMono: z, IBSMonoReten: z, CBSMonoReten: z,
		IBSMonoRet: z, CBSMonoRet: z, IBSEstCred: z, CBSEstCred: z, VIS: z,
		VIPIDevol: z, VII: z,
		Products: products, Discount: discount,
	}
}

// buildTotal monta o nó total a partir dos acumuladores do laço de itens.
func buildTotal(t totals, now time.Time, retTrib map[string]any) map[string]any {
	vNF := t.Products.Sub(t.Discount).
		Add(t.VFrete).Add(t.VSeg).Add(t.VOutro).
		Add(t.VIPI).Add(t.VICMSST).Add(t.VII).
		RoundBank(2)

	icmsTot := map[string]any{
		"vBC":        q2(t.VBC.RoundBank(2)),
		"vICMS":      q2(t.VICMS.RoundBank(2)),
		"vICMSDeson": q2(t.VICMSDeson.RoundBank(2)),
		"vFCP":       q2(t.VFCP.RoundBank(2)),
		"vBCST":      q2(t.VBCST.RoundBank(2)),
		"vST":        q2(t.VICMSST.RoundBank(2)),
		"vFCPST":     q2(t.VFCPST.RoundBank(2)),
		"vFCPSTRet":  "0.00",
		"vProd":      q2(t.Products.RoundBank(2)),
		"vFrete":     q2(t.VFrete.RoundBank(2)),
		"vSeg":       q2(t.VSeg.RoundBank(2)),
		"vDesc":      q2(t.Discount.RoundBank(2)),
		"vII":        q2(t.VII.RoundBank(2)),
		"vIPI":       q2(t.VIPI.RoundBank(2)),
		"vIPIDevol":  q2(t.VIPIDevol.RoundBank(2)),
		"vPIS":       q2(t.VPIS.RoundBank(2)),
		"vCOFINS":    q2(t.VCOFINS.RoundBank(2)),
		"vOutro":     q2(t.VOutro.RoundBank(2)),
		"vNF":        q2(vNF),
		"vTotTrib":   "0.00",
	}
	if t.VICMSUFDest.GreaterThan(decimal.Zero) || t.VFCPUFDest.GreaterThan(decimal.Zero) {
		icmsTot["vFCPUFDest"] = q2(t.VFCPUFDest.RoundBank(2))
		icmsTot["vICMSUFDest"] = q2(t.VICMSUFDest.RoundBank(2))
		icmsTot["vICMSUFRemet"] = "0.00"
	}

	totalNode := map[string]any{
		"ICMSTot":   icmsTot,
		"IBSCBSTot": buildIBSCBSTot(t),
	}
	// ISTot e vNFTot só existem quando a reforma incide: o vNFTot é o total do
	// documento com os tributos por fora, e um total igual ao vNF só polui.
	if t.VIS.IsPositive() {
		totalNode["ISTot"] = map[string]any{"vIS": q2(t.VIS.RoundBank(2))}
	}
	if vNFTot := reformDocumentTotal(t, vNF); vNFTot != nil {
		totalNode["vNFTot"] = *vNFTot
	}
	if len(retTrib) > 0 {
		totalNode["retTrib"] = retTrib
	}
	if t.HasISSQN {
		// Ordem XSD: vServ, vBC, vISS, vPIS, vCOFINS, dCompet, vDeducao,
		// vOutro, vDescIncond, vDescCond, vISSRet, cRegTrib.
		issqnTot := map[string]any{
			"vServ":   q2(t.VServ.RoundBank(2)),
			"vBC":     q2(t.VBCISSQN.RoundBank(2)),
			"vISS":    q2(t.VISSQN.RoundBank(2)),
			"vPIS":    q2(t.VPISISSQN.RoundBank(2)),
			"vCOFINS": q2(t.VCOFINSISSQN.RoundBank(2)),
			"dCompet": now.Format("2006-01-02"),
		}
		for tag, v := range map[string]decimal.Decimal{
			"vDeducao":    t.VDeducaoISSQN,
			"vOutro":      t.VOutroISSQN,
			"vDescIncond": t.VDescIncondISSQN,
			"vDescCond":   t.VDescCondISSQN,
			"vISSRet":     t.VISSRet,
		} {
			if v.IsPositive() {
				issqnTot[tag] = q2(v.RoundBank(2))
			}
		}
		if t.RegTribISSQN != "" {
			issqnTot["cRegTrib"] = t.RegTribISSQN
		}
		totalNode["ISSQNtot"] = issqnTot
	}
	return totalNode
}

// buildRetTrib monta total/retTrib — as retenções federais da nota. O perfil de
// retenção é da operação (nível 2); os valores saem da base do documento, então
// nunca são digitados. Ordem XSD: vRetPIS, vRetCOFINS, vRetCSLL, vBCIRRF,
// vIRRF, vBCRetPrev, vRetPrev.
func buildRetTrib(profile map[string]any, base decimal.Decimal) map[string]any {
	if len(profile) == 0 {
		return nil
	}
	hundred := decimal.NewFromInt(100)
	pct := func(key string) (decimal.Decimal, bool) {
		v, _ := profile[key].(string)
		if v == "" || v == "0.00" {
			return decimal.Zero, false
		}
		return base.Mul(d(v)).Div(hundred).RoundBank(2), true
	}
	node := map[string]any{}
	if v, ok := pct("p_ret_pis"); ok {
		node["vRetPIS"] = q2(v)
	}
	if v, ok := pct("p_ret_cofins"); ok {
		node["vRetCOFINS"] = q2(v)
	}
	if v, ok := pct("p_ret_csll"); ok {
		node["vRetCSLL"] = q2(v)
	}
	// vBCIRRF e vBCRetPrev só existem acompanhados do respectivo valor.
	if v, ok := pct("p_ret_irrf"); ok {
		node["vBCIRRF"] = q2(base.RoundBank(2))
		node["vIRRF"] = q2(v)
	}
	if v, ok := pct("p_ret_prev_inss"); ok {
		node["vBCRetPrev"] = q2(base.RoundBank(2))
		node["vRetPrev"] = q2(v)
	}
	if len(node) == 0 {
		return nil
	}
	return node
}

// buildImpostoDevol monta det/impostoDevol — o IPI devolvido na devolução de
// mercadoria a não contribuinte. Só faz sentido com finNFe=4, o que a emissão
// valida antes de chegar aqui.
func buildImpostoDevol(pDevol string, vIPI decimal.Decimal) map[string]any {
	if pDevol == "" {
		return nil
	}
	vIPIDevol := vIPI.Mul(d(pDevol)).Div(decimal.NewFromInt(100)).RoundBank(2)
	return map[string]any{
		"pDevol": pDevol,
		"IPI":    map[string]any{"vIPIDevol": q2(vIPIDevol)},
	}
}

// buildIBSCBSTot monta total/IBSCBSTot (type TIBSCBSMonoTot). Ordem XSD:
// vBCIBSCBS, gIBS, gCBS, gMono, gEstornoCred.
//
// Todo valor aqui é a **soma dos itens**, lida dos nós que já foram emitidos —
// nunca um segundo cálculo sobre a mesma base, que é como total e itens deixam
// de fechar.
func buildIBSCBSTot(t totals) map[string]any {
	node := map[string]any{
		"vBCIBSCBS": q2(t.IBSBC.RoundBank(2)),
		"gIBS": map[string]any{
			"gIBSUF": map[string]any{
				"vDif":     q2(t.IBSUFDif.RoundBank(2)),
				"vDevTrib": q2(t.IBSUFDevTrib.RoundBank(2)),
				"vIBSUF":   q2(t.IBSUF.RoundBank(2)),
			},
			"gIBSMun": map[string]any{
				"vDif":     q2(t.IBSMunDif.RoundBank(2)),
				"vDevTrib": q2(t.IBSMunDevTrib.RoundBank(2)),
				"vIBSMun":  q2(t.IBSMun.RoundBank(2)),
			},
			"vIBS":             q2(t.IBS.RoundBank(2)),
			"vCredPres":        q2(t.IBSCredPres.RoundBank(2)),
			"vCredPresCondSus": q2(t.IBSCredPresCondSus.RoundBank(2)),
		},
		"gCBS": map[string]any{
			"vDif":             q2(t.CBSDif.RoundBank(2)),
			"vDevTrib":         q2(t.CBSDevTrib.RoundBank(2)),
			"vCBS":             q2(t.CBS.RoundBank(2)),
			"vCredPres":        q2(t.CBSCredPres.RoundBank(2)),
			"vCredPresCondSus": q2(t.CBSCredPresCondSus.RoundBank(2)),
		},
	}
	if t.HasIBSCBSMono {
		node["gMono"] = map[string]any{
			"vIBSMono":      q2(t.IBSMono.RoundBank(2)),
			"vCBSMono":      q2(t.CBSMono.RoundBank(2)),
			"vIBSMonoReten": q2(t.IBSMonoReten.RoundBank(2)),
			"vCBSMonoReten": q2(t.CBSMonoReten.RoundBank(2)),
			"vIBSMonoRet":   q2(t.IBSMonoRet.RoundBank(2)),
			"vCBSMonoRet":   q2(t.CBSMonoRet.RoundBank(2)),
		}
	}
	if t.HasEstornoCred {
		node["gEstornoCred"] = map[string]any{
			"vIBSEstCred": q2(t.IBSEstCred.RoundBank(2)),
			"vCBSEstCred": q2(t.CBSEstCred.RoundBank(2)),
		}
	}
	return node
}

// reformDocumentTotal devolve o vNFTot — o total do documento com os tributos
// por fora (IBS, CBS e IS) somados ao vNF. Devolve nil quando a reforma não
// incide em item nenhum: repetir o vNF numa tag nova não informa nada.
func reformDocumentTotal(t totals, vNF decimal.Decimal) *string {
	extra := t.IBS.Add(t.CBS).
		Add(t.IBSMono).Add(t.CBSMono).
		Add(t.IBSMonoReten).Add(t.CBSMonoReten).
		Add(t.VIS)
	if !extra.IsPositive() {
		return nil
	}
	out := q2(vNF.Add(extra).RoundBank(2))
	return &out
}
