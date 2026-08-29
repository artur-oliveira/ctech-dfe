package services

// rateio.go distribui um total entre partes sem perder centavo (ou milésimo)
// no arredondamento. Serve ao rateio da unidade de carga do MDF-e (qtdRat, base
// 100) e ao rateio do lote da NF-e (qLote, base = quantidade vendida): em
// ambos, o somatório tem que fechar no total exato ou a SEFAZ rejeita.

import "github.com/shopspring/decimal"

// SplitProportional divide total entre len(weights) partes, proporcionalmente
// aos pesos, com places casas decimais. A última parte absorve o resíduo do
// arredondamento — sem isso o somatório fecha em 99,99 em vez de 100,00.
//
// Peso total zero devolve nil: não há como ratear sem base.
func SplitProportional(total decimal.Decimal, weights []decimal.Decimal, places int32) []string {
	if len(weights) == 0 {
		return nil
	}
	sum := decimal.Zero
	for _, w := range weights {
		sum = sum.Add(w)
	}
	if sum.IsZero() {
		return nil
	}
	out := make([]string, len(weights))
	acc := decimal.Zero
	for i, w := range weights {
		if i == len(weights)-1 {
			out[i] = total.Sub(acc).StringFixed(places)
			continue
		}
		part := total.Mul(w).Div(sum).RoundBank(places)
		acc = acc.Add(part)
		out[i] = part.StringFixed(places)
	}
	return out
}

// SplitEvenly divide total em n partes iguais, com a última absorvendo o
// resíduo. É o caso "não sei o peso de cada parte, então divido igual".
func SplitEvenly(total decimal.Decimal, n int, places int32) []string {
	weights := make([]decimal.Decimal, n)
	one := decimal.NewFromInt(1)
	for i := range weights {
		weights[i] = one
	}
	return SplitProportional(total, weights, places)
}
