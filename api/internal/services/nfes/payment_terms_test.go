package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func term(fields map[string]any) map[string]any {
	m := map[string]any{"payment_type": "15", "installments": 1}
	for k, v := range fields {
		m[k] = v
	}
	return m
}

func issueDate() time.Time {
	return time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
}

func sumDups(dups []NfeDuplicataItem) decimal.Decimal {
	total := decimal.Zero
	for _, d := range dups {
		total = total.Add(decimal.RequireFromString(d.VDup))
	}
	return total
}

// A regra que a SEFAZ cobra: a soma das duplicatas fecha com vNF centavo a
// centavo. R$ 100,00 em 3× é 33,33 + 33,33 + 33,34, nunca 99,99.
func TestExpandPaymentTerm_LastInstallmentAbsorbsRounding(t *testing.T) {
	cases := []struct {
		total        string
		installments int
		want         []string
	}{
		{"100.00", 3, []string{"33.33", "33.33", "33.34"}},
		// 0,005 arredonda para 0,00 (banker's rounding); a última parcela leva o
		// centavo inteiro. O que importa é a soma fechar.
		{"0.01", 2, []string{"0.00", "0.01"}},
		{"10.00", 3, []string{"3.33", "3.33", "3.34"}},
		{"999.99", 7, []string{"142.86", "142.86", "142.86", "142.86", "142.86", "142.86", "142.83"}},
		{"120.00", 3, []string{"40.00", "40.00", "40.00"}},
	}

	for _, tc := range cases {
		total := decimal.RequireFromString(tc.total)
		_, _, dups, err := ExpandPaymentTerm(
			term(map[string]any{"installments": tc.installments, "interval_days": 30, "first_due_days": 30}),
			total, issueDate())
		if err != nil {
			t.Fatalf("ExpandPaymentTerm(%s em %dx): %v", tc.total, tc.installments, err)
		}
		if len(dups) != tc.installments {
			t.Fatalf("%s em %dx: %d duplicatas", tc.total, tc.installments, len(dups))
		}
		for i, want := range tc.want {
			if dups[i].VDup != want {
				t.Errorf("%s em %dx: parcela %d = %s, esperado %s", tc.total, tc.installments, i+1, dups[i].VDup, want)
			}
		}
		if got := sumDups(dups); !got.Equal(total) {
			t.Errorf("%s em %dx: soma das duplicatas = %s, esperado %s", tc.total, tc.installments, got, total)
		}
	}
}

func TestExpandPaymentTerm_DueDates(t *testing.T) {
	_, _, dups, err := ExpandPaymentTerm(
		term(map[string]any{"installments": 3, "interval_days": 30, "first_due_days": 30}),
		decimal.RequireFromString("300.00"), issueDate())
	if err != nil {
		t.Fatalf("ExpandPaymentTerm: %v", err)
	}
	want := []string{"2026-09-07", "2026-10-07", "2026-11-06"}
	for i, w := range want {
		if dups[i].DVenc == nil || *dups[i].DVenc != w {
			t.Errorf("parcela %d vence em %v, esperado %s", i+1, dups[i].DVenc, w)
		}
	}
}

// À vista com parcela única não gera cobrança: não há o que parcelar.
func TestExpandPaymentTerm_CashSaleHasNoBilling(t *testing.T) {
	payments, fat, dups, err := ExpandPaymentTerm(
		term(nil), decimal.RequireFromString("50.00"), issueDate())
	if err != nil {
		t.Fatalf("ExpandPaymentTerm: %v", err)
	}
	if len(payments) != 1 || payments[0].Value != "50.00" {
		t.Errorf("pagamentos = %+v", payments)
	}
	if payments[0].IndPag == nil || *payments[0].IndPag != indPagAVista {
		t.Errorf("ind_pag = %v, esperado à vista", payments[0].IndPag)
	}
	if fat != nil || dups != nil {
		t.Error("venda à vista não pode gerar fatura nem duplicatas")
	}
}

// Uma parcela só, mas com prazo, é a prazo — e gera cobrança.
func TestExpandPaymentTerm_SingleInstallmentWithDelayIsCredit(t *testing.T) {
	payments, fat, dups, err := ExpandPaymentTerm(
		term(map[string]any{"first_due_days": 28}), decimal.RequireFromString("50.00"), issueDate())
	if err != nil {
		t.Fatalf("ExpandPaymentTerm: %v", err)
	}
	if payments[0].IndPag == nil || *payments[0].IndPag != indPagAPrazo {
		t.Errorf("ind_pag = %v, esperado a prazo", payments[0].IndPag)
	}
	if fat == nil || len(dups) != 1 {
		t.Fatalf("esperada 1 duplicata com fatura, obtido fat=%v dups=%d", fat, len(dups))
	}
	if *dups[0].DVenc != "2026-09-05" {
		t.Errorf("vencimento = %s", *dups[0].DVenc)
	}
}

func TestExpandPaymentTerm_NilTermExpandsToNothing(t *testing.T) {
	payments, fat, dups, err := ExpandPaymentTerm(nil, decimal.RequireFromString("10.00"), issueDate())
	if err != nil || payments != nil || fat != nil || dups != nil {
		t.Errorf("sem condição, nada é expandido: %v %v %v %v", payments, fat, dups, err)
	}
}

func TestExpandPaymentTerm_RejectsIncompleteTerm(t *testing.T) {
	if _, _, _, err := ExpandPaymentTerm(map[string]any{"installments": 2}, decimal.NewFromInt(10), issueDate()); err == nil {
		t.Error("esperada recusa de condição sem forma de pagamento")
	}
	if _, _, _, err := ExpandPaymentTerm(term(map[string]any{"installments": 0}), decimal.NewFromInt(10), issueDate()); err == nil {
		t.Error("esperada recusa de condição com 0 parcelas")
	}
}

// O DynamoDB devolve números como float64 depois do unmarshal genérico.
func TestExpandPaymentTerm_AcceptsFloatCounts(t *testing.T) {
	_, _, dups, err := ExpandPaymentTerm(
		term(map[string]any{"installments": float64(2), "interval_days": float64(15), "first_due_days": float64(15)}),
		decimal.RequireFromString("100.00"), issueDate())
	if err != nil {
		t.Fatalf("ExpandPaymentTerm: %v", err)
	}
	if len(dups) != 2 {
		t.Fatalf("%d duplicatas, esperado 2", len(dups))
	}
}
