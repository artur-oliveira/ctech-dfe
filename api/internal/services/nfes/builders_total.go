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
	IBSBC, IBSUF, IBSMun, IBS, CBSBC, CBS                decimal.Decimal
	Products, Discount                                   decimal.Decimal
	HasISSQN                                             bool
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
		IBSBC: z, IBSUF: z, IBSMun: z, IBS: z, CBSBC: z, CBS: z,
		Products: products, Discount: discount,
	}
}

// buildTotal monta o nó total a partir dos acumuladores do laço de itens.
func buildTotal(t totals, now time.Time) map[string]any {
	vNF := t.Products.Sub(t.Discount).
		Add(t.VFrete).Add(t.VSeg).Add(t.VOutro).
		Add(t.VIPI).Add(t.VICMSST).
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
		"vII":        "0.00",
		"vIPI":       q2(t.VIPI.RoundBank(2)),
		"vIPIDevol":  "0.00",
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
		"ICMSTot": icmsTot,
		"IBSCBSTot": map[string]any{
			"vBCIBSCBS": q2(t.IBSBC.RoundBank(2)),
			"gIBS": map[string]any{
				"gIBSUF": map[string]any{
					"vDif": "0.00", "vDevTrib": "0.00",
					"vIBSUF": q2(t.IBSUF.RoundBank(2)),
				},
				"gIBSMun": map[string]any{
					"vDif": "0.00", "vDevTrib": "0.00",
					"vIBSMun": q2(t.IBSMun.RoundBank(2)),
				},
				"vIBS":             q2(t.IBS.RoundBank(2)),
				"vCredPres":        "0.00",
				"vCredPresCondSus": "0.00",
			},
			"gCBS": map[string]any{
				"vDif": "0.00", "vDevTrib": "0.00",
				"vCBS":             q2(t.CBS.RoundBank(2)),
				"vCredPres":        "0.00",
				"vCredPresCondSus": "0.00",
			},
		},
	}
	if t.HasISSQN {
		totalNode["ISSQNtot"] = map[string]any{
			"vServ":   q2(t.VServ.RoundBank(2)),
			"vBC":     q2(t.VBCISSQN.RoundBank(2)),
			"vISS":    q2(t.VISSQN.RoundBank(2)),
			"vPIS":    q2(t.VPISISSQN.RoundBank(2)),
			"vCOFINS": q2(t.VCOFINSISSQN.RoundBank(2)),
			"dCompet": now.Format("2006-01-02"),
		}
	}
	return totalNode
}
