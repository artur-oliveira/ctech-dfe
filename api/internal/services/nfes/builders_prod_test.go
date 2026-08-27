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
