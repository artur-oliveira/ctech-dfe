package services

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestSplitProportional_FechaNoTotal(t *testing.T) {
	// 100 em 3 partes iguais: 33.33 + 33.33 + 33.34, não 99.99.
	got := SplitProportional(decimal.NewFromInt(100), []decimal.Decimal{
		decimal.NewFromInt(1), decimal.NewFromInt(1), decimal.NewFromInt(1),
	}, 2)
	want := []string{"33.33", "33.33", "33.34"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSplitProportional_PorPeso(t *testing.T) {
	got := SplitProportional(decimal.NewFromInt(100), []decimal.Decimal{
		decimal.NewFromInt(30), decimal.NewFromInt(70),
	}, 2)
	if got[0] != "30.00" || got[1] != "70.00" {
		t.Fatalf("got %v", got)
	}
}

func TestSplitProportional_SemBase(t *testing.T) {
	if SplitProportional(decimal.NewFromInt(10), nil, 2) != nil {
		t.Fatal("sem pesos não há rateio")
	}
	if SplitProportional(decimal.NewFromInt(10), []decimal.Decimal{decimal.Zero}, 2) != nil {
		t.Fatal("peso total zero não há rateio")
	}
}

func TestSplitEvenly(t *testing.T) {
	got := SplitEvenly(decimal.RequireFromString("10"), 2, 3)
	if got[0] != "5.000" || got[1] != "5.000" {
		t.Fatalf("got %v", got)
	}
}
