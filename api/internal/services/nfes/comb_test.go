package nfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestBuildCombEncerranteUsaLeituraAnterior(t *testing.T) {
	pump := map[string]any{"n_bico": "1", "n_bomba": "2", "n_tanque": "3", "last_v_enc_fin": "1000.000"}
	item := map[string]any{"comb_c_prod_anp": "320102001", "comb_v_enc_fin": "1050.000"}
	got := buildComb(item, pump, decimal.RequireFromString("50"))
	enc := got["encerrante"].(map[string]any)
	if enc["vEncIni"] != "1000.000" || enc["vEncFin"] != "1050.000" {
		t.Fatalf("encerrante errado: %v", enc)
	}
	if enc["nBico"] != "1" || enc["nBomba"] != "2" || enc["nTanque"] != "3" {
		t.Fatalf("bomba errada: %v", enc)
	}
}

// vEncFin menor que a leitura anterior é impossível fisicamente: erro, não
// número negativo silencioso.
func TestBuildCombEncerranteRegressivoRecusado(t *testing.T) {
	pump := map[string]any{"n_bico": "1", "last_v_enc_fin": "1000.000"}
	if err := validateEncerrante(pump, "999.000"); err == nil {
		t.Fatal("leitura regressiva deveria ser recusada")
	}
	if err := validateEncerrante(pump, "1000.000"); err != nil {
		t.Fatalf("leitura igual (bomba parada) é válida: %v", err)
	}
	if err := validateEncerrante(pump, ""); err != nil {
		t.Fatalf("venda sem encerrante não valida nada: %v", err)
	}
}

// A primeira venda da bomba parte de zero: sem leitura anterior, vEncIni é
// 0.000 e não some do XML.
func TestBuildCombPrimeiraVendaDaBomba(t *testing.T) {
	got := buildComb(
		map[string]any{"comb_c_prod_anp": "320102001", "comb_v_enc_fin": "10.500"},
		map[string]any{"n_bico": "1"},
		decimal.RequireFromString("10.5"),
	)
	enc := got["encerrante"].(map[string]any)
	if enc["vEncIni"] != "0.000" {
		t.Fatalf("vEncIni = %v, want 0.000", enc["vEncIni"])
	}
	if _, has := enc["nBomba"]; has {
		t.Fatalf("nBomba não cadastrado não deve sair: %v", enc)
	}
}

// vCIDE é derivado: base (quantidade vendida) × alíquota cadastrada.
func TestBuildCombCIDECalculada(t *testing.T) {
	got := buildComb(
		map[string]any{"comb_c_prod_anp": "320102001", "comb_cide_v_aliq_prod": "0.1000"},
		nil,
		decimal.RequireFromString("50"),
	)
	cide := got["CIDE"].(map[string]any)
	if cide["qBCProd"] != "50.0000" || cide["vAliqProd"] != "0.1000" || cide["vCIDE"] != "5.00" {
		t.Fatalf("CIDE = %v", cide)
	}
}

func TestBuildCombSemCIDENemEncerrante(t *testing.T) {
	got := buildComb(map[string]any{"comb_c_prod_anp": "320102001"}, nil, decimal.NewFromInt(1))
	for _, tag := range []string{"CIDE", "encerrante", "origComb", "qTemp"} {
		if _, has := got[tag]; has {
			t.Errorf("%s não informado não deve sair: %v", tag, got)
		}
	}
}

func TestBuildCombOrigComb(t *testing.T) {
	got := buildComb(map[string]any{
		"comb_c_prod_anp": "320102001",
		"comb_orig": []any{
			map[string]any{"ind_import": "0", "c_uf_orig": "35", "p_orig": "70.00"},
			map[string]any{"ind_import": "1", "c_uf_orig": "33", "p_orig": "30.00"},
		},
	}, nil, decimal.NewFromInt(1))
	orig := got["origComb"].([]map[string]any)
	if len(orig) != 2 || orig[1]["indImport"] != "1" || orig[0]["pOrig"] != "70.00" {
		t.Fatalf("origComb = %v", orig)
	}
}

// Item que não é combustível não ganha o grupo.
func TestBuildCombNaoCombustivel(t *testing.T) {
	if buildComb(map[string]any{}, nil, decimal.NewFromInt(1)) != nil {
		t.Fatal("item sem cProdANP não tem grupo comb")
	}
}
