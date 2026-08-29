package services_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

func TestExpandInstallmentsUltimaAbsorveResiduo(t *testing.T) {
	got := services.ExpandInstallments(decimal.RequireFromString("100.00"), 3, 30, 30,
		time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	if len(got) != 3 {
		t.Fatalf("want 3 parcelas, got %d", len(got))
	}
	sum := decimal.Zero
	for _, i := range got {
		sum = sum.Add(i.Value)
	}
	if !sum.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("soma %s != total", sum)
	}
	if got[2].Value.String() != "33.34" {
		t.Fatalf("resíduo tem que cair na última: %s", got[2].Value)
	}
}

func TestExpandInstallmentsVencimentos(t *testing.T) {
	from := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	got := services.ExpandInstallments(decimal.RequireFromString("90.00"), 3, 15, 10, from)
	for i, want := range []string{"2026-09-06", "2026-09-21", "2026-10-06"} {
		if d := got[i].DueDate.Format("2006-01-02"); d != want {
			t.Fatalf("parcela %d vence %s, want %s", i+1, d, want)
		}
	}
	if got[0].Number != "001" || got[2].Number != "003" {
		t.Fatalf("numeração errada: %v", got)
	}
}
