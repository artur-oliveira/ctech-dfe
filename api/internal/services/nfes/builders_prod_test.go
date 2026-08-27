package nfes

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestBuildDIDerivaNumeroDaAdicao(t *testing.T) {
	di := map[string]any{
		"n_di": "2026/0000001", "d_di": "2026-01-15", "x_loc_desemb": "PORTO DE ITAQUI",
		"uf_desemb": "MA", "d_desemb": "2026-01-20", "tp_via_transp": "01",
		"v_afrmm": "150.00", "tp_intermedio": "1", "c_exportador": "EXP-1",
		"additions": []any{
			map[string]any{"n_adicao": "1", "c_fabricante": "F1", "v_desc_di": "0.00"},
			map[string]any{"n_adicao": "2", "c_fabricante": "F2", "v_desc_di": "5.00"},
		},
	}
	got := buildDI(di, 2, 1, "")
	if got["nDI"] != "2026/0000001" || got["UFDesemb"] != "MA" || got["vAFRMM"] != "150.00" {
		t.Fatalf("cabeçalho da DI errado: %v", got)
	}
	adi := got["adi"].([]map[string]any)
	if len(adi) != 1 || adi[0]["nAdicao"] != "2" || adi[0]["nSeqAdic"] != "1" || adi[0]["cFabricante"] != "F2" {
		t.Fatalf("adição derivada errada: %v", adi)
	}
}

// O nDraw do embarque vence o cadastrado na adição.
func TestBuildDINDrawDaEmissaoVence(t *testing.T) {
	di := map[string]any{"additions": []any{map[string]any{"n_adicao": "1", "n_draw": "CADASTRO"}}}
	if got := buildDI(di, 1, 1, "EMBARQUE")["adi"].([]map[string]any)[0]; got["nDraw"] != "EMBARQUE" {
		t.Fatalf("nDraw errado: %v", got)
	}
	if got := buildDI(di, 1, 1, "")["adi"].([]map[string]any)[0]; got["nDraw"] != "CADASTRO" {
		t.Fatalf("nDraw do cadastro perdido: %v", got)
	}
}

// Índice fora da lista não inventa adição — o nó sai sem adi e a SEFAZ recusa,
// que é melhor que emitir uma adição errada.
func TestBuildDIAdicaoInexistente(t *testing.T) {
	if _, ok := buildDI(map[string]any{"additions": []any{}}, 3, 1, "")["adi"]; ok {
		t.Fatal("adição inexistente não pode virar nó")
	}
}

func TestBuildProdNVEnFCIeCodigosDeBarra(t *testing.T) {
	item := map[string]any{
		"product_code": "P1", "nve": []any{"AA0001", "BB0002"},
		"n_fci":   "0A1B2C3D-4E5F-6789-ABCD-EF0123456789",
		"c_barra": "7891234567890", "c_barra_trib": "7899999999999",
	}
	prod := buildProd(item, prodParams{
		Description: "Produto", Unit: "UN", TaxableUnit: "UN",
		QTrib: "1.0000", VUnTrib: "10.00", VProd: "10.00",
		Disc: decimal.Zero, VFrete: decimal.Zero, VSeg: decimal.Zero, VOutro: decimal.Zero,
	})
	nve := prod["NVE"].([]string)
	if len(nve) != 2 || nve[0] != "AA0001" {
		t.Fatalf("NVE errado: %v", prod["NVE"])
	}
	if prod["nFCI"] != "0A1B2C3D-4E5F-6789-ABCD-EF0123456789" {
		t.Fatalf("nFCI ausente: %v", prod)
	}
	if prod["cBarra"] != "7891234567890" || prod["cBarraTrib"] != "7899999999999" {
		t.Fatalf("códigos de barra ausentes: %v", prod)
	}
}

func TestBuildExportaUsaLocalDeRetiradaSalvo(t *testing.T) {
	op := map[string]any{"export_uf_saida_pais": "PI", "export_loc_despacho_index": 0}
	pickups := []any{map[string]any{"x_lgr": "Porto de Luís Correia", "x_mun": "Luis Correia"}}
	got := buildExporta(op, pickups)
	if got["UFSaidaPais"] != "PI" || got["xLocDespacho"] != "Porto de Luís Correia" {
		t.Fatalf("exporta errado: %v", got)
	}
	if got["xLocExporta"] != "Luis Correia" {
		t.Fatalf("município de saída ausente: %v", got)
	}
}

// Índice fora da lista não inventa local: só a UF sai.
func TestBuildExportaIndiceInvalido(t *testing.T) {
	got := buildExporta(map[string]any{"export_uf_saida_pais": "PI", "export_loc_despacho_index": 3}, nil)
	if len(got) != 1 || got["UFSaidaPais"] != "PI" {
		t.Fatalf("want só UFSaidaPais: %v", got)
	}
	if buildExporta(map[string]any{}, nil) != nil {
		t.Fatal("operação sem UF de saída não é exportação")
	}
}

func TestBuildDetExportExportIndTudoOuNada(t *testing.T) {
	got := buildDetExport([]map[string]any{
		{"n_draw": "D1", "n_re": "123456789012", "ch_nfe": testDetExportKey, "q_export": "10.0000"},
		{"n_draw": "D2", "n_re": "123456789012"},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 nós: %v", got)
	}
	if got[0]["exportInd"].(map[string]any)["nRE"] != "123456789012" {
		t.Fatalf("exportInd errado: %v", got[0])
	}
	if _, ok := got[1]["exportInd"]; ok {
		t.Fatalf("exportInd incompleto não pode sair: %v", got[1])
	}
	if buildDetExport(nil) != nil {
		t.Fatal("sem exportação não há detExport")
	}
}

const testDetExportKey = "22260811647612000197550010000000011100000015"

func TestBuildIISoComValoresDeclarados(t *testing.T) {
	got := buildII(map[string]any{"ii_v_ii": "50.00", "ii_v_desp_adu": "10.00"},
		decimal.RequireFromString("100.00"))
	if got["vBC"] != "100.00" || got["vII"] != "50.00" || got["vDespAdu"] != "10.00" || got["vIOF"] != "0.00" {
		t.Fatalf("II errado: %v", got)
	}
	if buildII(map[string]any{}, decimal.RequireFromString("100.00")) != nil {
		t.Fatal("item sem II declarado não gera o grupo")
	}
}
