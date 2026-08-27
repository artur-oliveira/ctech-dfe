package services

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Installment é uma parcela derivada de um prazo: número sequencial,
// vencimento e valor. Serve à cobrança da NF-e (dup) e ao pagamento do
// transportador autônomo no MDF-e (infPrazo) — é o mesmo parcelamento.
type Installment struct {
	Number  string
	DueDate time.Time
	Value   decimal.Decimal
}

// ExpandInstallments divide total em count parcelas vencendo a cada
// intervalDays, a primeira firstDueDays depois de from. A última parcela
// absorve o resíduo: R$ 100,00 em 3× vira 33,33 + 33,33 + 33,34, e não
// 33,33 × 3 = 99,99. count < 1 devolve nil — quem chama já validou.
func ExpandInstallments(
	total decimal.Decimal, count, intervalDays, firstDueDays int, from time.Time,
) []Installment {
	if count < 1 {
		return nil
	}
	base := total.Div(decimal.NewFromInt(int64(count))).RoundBank(2)
	out := make([]Installment, 0, count)
	accumulated := decimal.Zero
	for i := 0; i < count; i++ {
		value := base
		if i == count-1 {
			value = total.Sub(accumulated)
		}
		accumulated = accumulated.Add(value)
		out = append(out, Installment{
			Number:  fmt.Sprintf("%03d", i+1),
			DueDate: from.AddDate(0, 0, firstDueDays+i*intervalDays),
			Value:   value,
		})
	}
	return out
}
