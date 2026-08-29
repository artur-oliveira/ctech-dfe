package nfes

// rastro.go monta prod/rastro — a rastreabilidade por lote, obrigatória em
// medicamento e comum em alimento, bebida e agrotóxico. O lote vem do cadastro
// (organization_product_lots); a quantidade que saiu em cada lote é rateada da
// quantidade vendida, nunca digitada em duplicidade.

import (
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// qLotePlaces são as 3 casas decimais de qLote (TDec_0803v no leiaute).
const qLotePlaces = 3

// resolvedLot é um lote do cadastro já cruzado com o item: o que é do lote veio
// de organization_product_lots, e Quantity é a quantidade que a emissão
// declarou para ele (vazia quando o rateio decide).
type resolvedLot struct {
	NLote    string
	DFab     string
	DVal     string
	CAgreg   string
	Quantity string
}

// buildRastro monta a lista de rastro na ordem do XSD (nLote, qLote, dFab,
// dVal, cAgreg). Quando nenhum lote traz quantidade, a quantidade vendida é
// dividida entre eles com a última parte absorvendo o resíduo; quando os lotes
// trazem quantidade, ela é respeitada como veio.
func buildRastro(lots []resolvedLot, qty decimal.Decimal) []map[string]any {
	if len(lots) == 0 {
		return nil
	}
	shares := lotShares(lots, qty)
	out := make([]map[string]any, 0, len(lots))
	for i, l := range lots {
		node := map[string]any{
			"nLote": l.NLote,
			"qLote": shares[i],
			"dFab":  l.DFab,
			"dVal":  l.DVal,
		}
		if l.CAgreg != "" {
			node["cAgreg"] = l.CAgreg
		}
		out = append(out, node)
	}
	return out
}

// lotShares devolve a quantidade de cada lote. Quantidade informada vence; o
// que ficou em branco divide igualmente o que sobrou da quantidade vendida.
func lotShares(lots []resolvedLot, qty decimal.Decimal) []string {
	out := make([]string, len(lots))
	remaining := qty
	var blanks []int
	for i, l := range lots {
		if l.Quantity == "" {
			blanks = append(blanks, i)
			continue
		}
		v, err := decimal.NewFromString(l.Quantity)
		if err != nil {
			v = decimal.Zero
		}
		out[i] = v.StringFixed(qLotePlaces)
		remaining = remaining.Sub(v)
	}
	if len(blanks) == 0 {
		return out
	}
	if remaining.IsNegative() {
		remaining = decimal.Zero
	}
	shares := services.SplitEvenly(remaining, len(blanks), qLotePlaces)
	for k, i := range blanks {
		out[i] = shares[k]
	}
	return out
}
