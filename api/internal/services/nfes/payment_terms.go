package nfes

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Campos da condição de pagamento lidos na expansão.
const (
	ptFieldPaymentType  = "payment_type"
	ptFieldIndPag       = "ind_pag"
	ptFieldInstallments = "installments"
	ptFieldIntervalDays = "interval_days"
	ptFieldFirstDueDays = "first_due_days"
	ptFieldCard         = "card"

	// indPagAVista / indPagAPrazo são os domínios de indPag do leiaute.
	indPagAVista = "0"
	indPagAPrazo = "1"

	// dueDateLayout é o formato de dVenc no XML da NF-e.
	dueDateLayout = "2006-01-02"
)

// loadPaymentTerm carrega a condição de pagamento referenciada na emissão.
func loadPaymentTerm(
	ctx context.Context, repo *repositories.PaymentTermRepository, orgPK string, termID *string,
) (map[string]any, error) {
	if termID == nil || *termID == "" {
		return nil, nil
	}
	item, err := repo.Get(ctx, orgPK, *termID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("condição de pagamento não encontrada: " + *termID)
	}
	var term map[string]any
	if err := attributevalue.UnmarshalMap(item, &term); err != nil {
		return nil, problem.InternalServer("failed to decode payment term")
	}
	return term, nil
}

// ExpandPaymentTerm transforma uma condição de pagamento e o total do documento
// em pagamentos, fatura e duplicatas.
//
// Função pura, testável isoladamente. **A última parcela absorve o resíduo de
// arredondamento**: a soma das duplicatas tem que fechar com vNF centavo a
// centavo, sob pena de rejeição da SEFAZ.
func ExpandPaymentTerm(
	term map[string]any, total decimal.Decimal, issueDate time.Time,
) ([]NfePaymentItem, *NfeFatItem, []NfeDuplicataItem, error) {
	if term == nil {
		return nil, nil, nil, nil
	}

	paymentType, _ := term[ptFieldPaymentType].(string)
	if paymentType == "" {
		return nil, nil, nil, problem.BadRequest("condição de pagamento sem forma de pagamento")
	}
	installments := intFromAny(term[ptFieldInstallments], 1)
	if installments < 1 {
		return nil, nil, nil, problem.BadRequest("condição de pagamento com número de parcelas inválido")
	}
	intervalDays := intFromAny(term[ptFieldIntervalDays], 0)
	firstDueDays := intFromAny(term[ptFieldFirstDueDays], 0)

	indPag, _ := term[ptFieldIndPag].(string)
	if indPag == "" {
		// Uma parcela com vencimento imediato é à vista; qualquer outra coisa é
		// a prazo. Derivar evita um cadastro internamente incoerente.
		if installments == 1 && firstDueDays == 0 {
			indPag = indPagAVista
		} else {
			indPag = indPagAPrazo
		}
	}

	payment := NfePaymentItem{PaymentType: paymentType, Value: q2(total), IndPag: &indPag}
	if card, ok := term[ptFieldCard].(map[string]any); ok && len(card) > 0 {
		payment.Card = card
	}

	// À vista com parcela única não gera cobrança: não há o que parcelar.
	if indPag == indPagAVista && installments == 1 {
		return []NfePaymentItem{payment}, nil, nil, nil
	}

	// O parcelamento é o mesmo do MDF-e (infPrazo): um algoritmo só, em
	// services.ExpandInstallments.
	dups := make([]NfeDuplicataItem, 0, installments)
	for _, inst := range services.ExpandInstallments(total, installments, intervalDays, firstDueDays, issueDate) {
		number, due := inst.Number, inst.DueDate.Format(dueDateLayout)
		dups = append(dups, NfeDuplicataItem{NDup: &number, DVenc: &due, VDup: q2(inst.Value)})
	}

	totalStr := q2(total)
	fat := &NfeFatItem{VOrig: &totalStr, VLiq: &totalStr}
	return []NfePaymentItem{payment}, fat, dups, nil
}

// intFromAny lê um inteiro que o DynamoDB pode ter devolvido como float64.
func intFromAny(v any, def int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return def
}
