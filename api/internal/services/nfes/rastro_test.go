package nfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestBuildRastroRateiaQuantidade(t *testing.T) {
	got := buildRastro([]resolvedLot{
		{NLote: "L1", DFab: "2026-01-01", DVal: "2027-01-01"},
		{NLote: "L2", DFab: "2026-02-01", DVal: "2027-02-01"},
	}, decimal.RequireFromString("10"))
	if got[0]["qLote"] != "5.000" || got[1]["qLote"] != "5.000" {
		t.Fatalf("rateio errado: %v", got)
	}
	if got[0]["nLote"] != "L1" || got[0]["dVal"] != "2027-01-01" {
		t.Fatalf("lote errado: %v", got[0])
	}
}

// O resíduo do rateio vai para o último lote: 10 em 3 lotes fecha em 10,000.
func TestBuildRastroResiduoNoUltimo(t *testing.T) {
	got := buildRastro([]resolvedLot{
		{NLote: "L1"}, {NLote: "L2"}, {NLote: "L3"},
	}, decimal.RequireFromString("10"))
	total := decimal.Zero
	for _, n := range got {
		total = total.Add(decimal.RequireFromString(n["qLote"].(string)))
	}
	if !total.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("somatório = %s, want 10: %v", total, got)
	}
	if got[2]["qLote"] != "3.334" {
		t.Fatalf("último lote deveria absorver o resíduo: %v", got)
	}
}

// Quantidade informada vence; os lotes em branco dividem o que sobrou.
func TestBuildRastroQuantidadeInformada(t *testing.T) {
	got := buildRastro([]resolvedLot{
		{NLote: "L1", Quantity: "7"},
		{NLote: "L2"},
	}, decimal.RequireFromString("10"))
	if got[0]["qLote"] != "7.000" || got[1]["qLote"] != "3.000" {
		t.Fatalf("got %v", got)
	}
}

// cAgreg é opcional: sem código de agregação o nó não sai.
func TestBuildRastroCAgregOpcional(t *testing.T) {
	got := buildRastro([]resolvedLot{{NLote: "L1"}, {NLote: "L2", CAgreg: "AG-2"}}, decimal.NewFromInt(2))
	if _, has := got[0]["cAgreg"]; has {
		t.Fatalf("cAgreg não informado não deve sair: %v", got[0])
	}
	if got[1]["cAgreg"] != "AG-2" {
		t.Fatalf("cAgreg = %v", got[1])
	}
}

func TestBuildRastroSemLotes(t *testing.T) {
	if buildRastro(nil, decimal.NewFromInt(1)) != nil {
		t.Fatal("sem lote, rastro não existe")
	}
}
