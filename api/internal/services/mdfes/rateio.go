package mdfes

// rateio.go calcula o rateio (qtdRat) da unidade de transporte/carga entre os
// documentos que ela leva. O percentual é derivado do peso de cada documento —
// o operador nunca digita percentual, e o somatório fecha em 100,00 por
// construção.

import (
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// rateCargo distribui 100% da unidade de carga entre os documentos que ela
// transporta, proporcionalmente ao peso. O fechamento exato no total (a última
// parte absorvendo o resíduo) é o mesmo problema do rateio de lote da NF-e —
// ver services.SplitProportional.
func rateCargo(weights map[string]decimal.Decimal, keys []string) map[string]string {
	parts := make([]decimal.Decimal, len(keys))
	for i, k := range keys {
		parts[i] = weights[k]
	}
	shares := services.SplitProportional(decimal.NewFromInt(100), parts, 2)
	out := make(map[string]string, len(keys))
	for i, k := range keys {
		if shares == nil {
			return out
		}
		out[k] = shares[i]
	}
	return out
}
