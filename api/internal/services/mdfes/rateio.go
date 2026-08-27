package mdfes

// rateio.go calcula o rateio (qtdRat) da unidade de transporte/carga entre os
// documentos que ela leva. O percentual é derivado do peso de cada documento —
// o operador nunca digita percentual, e o somatório fecha em 100,00 por
// construção.

import "github.com/shopspring/decimal"

// rateCargo distribui 100% da unidade de carga entre os documentos que ela
// transporta, proporcionalmente ao peso. A última chave absorve o resíduo de
// arredondamento — sem isso o somatório fecha em 99.99 e a SEFAZ rejeita.
func rateCargo(weights map[string]decimal.Decimal, keys []string) map[string]string {
	total := decimal.Zero
	for _, k := range keys {
		total = total.Add(weights[k])
	}
	out := make(map[string]string, len(keys))
	if total.IsZero() {
		return out
	}
	hundred := decimal.NewFromInt(100)
	acc := decimal.Zero
	for i, k := range keys {
		if i == len(keys)-1 {
			out[k] = hundred.Sub(acc).StringFixed(2)
			continue
		}
		part := weights[k].Mul(hundred).Div(total).RoundBank(2)
		acc = acc.Add(part)
		out[k] = part.StringFixed(2)
	}
	return out
}
