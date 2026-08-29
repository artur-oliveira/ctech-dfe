package mdfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestRateCargoSomaCem(t *testing.T) {
	w := map[string]decimal.Decimal{
		"A": decimal.RequireFromString("100"),
		"B": decimal.RequireFromString("100"),
		"C": decimal.RequireFromString("100"),
	}
	got := rateCargo(w, []string{"A", "B", "C"})
	sum := decimal.Zero
	for _, v := range got {
		sum = sum.Add(decimal.RequireFromString(v))
	}
	if !sum.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("rateio tem que somar 100.00, deu %s (%v)", sum, got)
	}
	if got["C"] != "33.34" {
		t.Fatalf("resíduo tem que cair na última chave: %v", got)
	}
}

func TestRateCargoProporcionalAoPeso(t *testing.T) {
	w := map[string]decimal.Decimal{
		"A": decimal.RequireFromString("300"),
		"B": decimal.RequireFromString("100"),
	}
	got := rateCargo(w, []string{"A", "B"})
	if got["A"] != "75.00" || got["B"] != "25.00" {
		t.Fatalf("rateio não seguiu o peso: %v", got)
	}
}

// Carga sem peso não tem rateio a declarar — dividir por zero seria pior.
func TestRateCargoSemPeso(t *testing.T) {
	if got := rateCargo(map[string]decimal.Decimal{}, []string{"A"}); len(got) != 0 {
		t.Fatalf("want vazio, got %v", got)
	}
}

func TestBuildUnidCargaRateioSomaCem(t *testing.T) {
	got := buildUnidCarga([]cargoUnit{
		{TpUnid: "1", IdUnid: "CONT-1", Seals: []string{"L1"}},
		{TpUnid: "1", IdUnid: "CONT-2"},
		{TpUnid: "1", IdUnid: "CONT-3"},
	})
	sum := decimal.Zero
	for _, n := range got {
		sum = sum.Add(decimal.RequireFromString(n["qtdRat"].(string)))
	}
	if !sum.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("rateio das unidades de carga tem que somar 100.00: %v", got)
	}
	if len(got[0]["lacUnidCarga"].([]map[string]any)) != 1 {
		t.Fatalf("lacre da unidade perdido: %v", got[0])
	}
	if _, ok := got[1]["lacUnidCarga"]; ok {
		t.Fatalf("unidade sem lacre não devia ter o nó: %v", got[1])
	}
}
