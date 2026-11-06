package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildRetTribCalculaSobreABase(t *testing.T) {
	got := buildRetTrib(map[string]any{
		"p_ret_pis": "0.65", "p_ret_cofins": "3.00", "p_ret_csll": "1.00", "p_ret_irrf": "1.50",
	}, decimal.RequireFromString("1000.00"))
	if got["vRetPIS"] != "6.50" || got["vRetCOFINS"] != "30.00" || got["vRetCSLL"] != "10.00" {
		t.Fatalf("retenções erradas: %v", got)
	}
	if got["vBCIRRF"] != "1000.00" || got["vIRRF"] != "15.00" {
		t.Fatalf("IRRF errado: %v", got)
	}
	if _, ok := got["vBCRetPrev"]; ok {
		t.Fatal("INSS não configurado não pode aparecer")
	}
}

// Operação sem perfil de retenção não gera o grupo.
func TestBuildRetTribSemPerfil(t *testing.T) {
	if buildRetTrib(nil, decimal.RequireFromString("1000.00")) != nil {
		t.Fatal("sem perfil não há retTrib")
	}
	if buildRetTrib(map[string]any{"p_ret_pis": "0.00"}, decimal.RequireFromString("1000.00")) != nil {
		t.Fatal("percentual zerado é o mesmo que não configurado")
	}
}

func TestBuildImpostoDevol(t *testing.T) {
	got := buildImpostoDevol("100.00", decimal.RequireFromString("10.00"))
	if got["pDevol"] != "100.00" || got["IPI"].(map[string]any)["vIPIDevol"] != "10.00" {
		t.Fatalf("impostoDevol errado: %v", got)
	}
}

func TestValidateDevolucaoRecusaForaDaDevolucao(t *testing.T) {
	p := "100.00"
	items := []NfeProductItem{{PDevol: &p}}
	if err := validateDevolucao("1", items); err == nil {
		t.Fatal("p_devol em nota normal tinha que ser recusado")
	}
	if err := validateDevolucao("4", items); err != nil {
		t.Fatalf("devolução aceita p_devol: %v", err)
	}
}

// MOC W16-10 e W07-10: vNF fecha com a fórmula do MOC e vProd exclui serviço ISSQN.
func TestBuildTotal_VNFeVProdPeloMOC(t *testing.T) {
	tt := newTotals(decimal.RequireFromString("1000.00"), decimal.RequireFromString("50.00"))
	tt.VICMSDeson = decimal.RequireFromString("10.00")
	tt.VICMSST = decimal.RequireFromString("20.00")
	tt.VFCPST = decimal.RequireFromString("2.00")
	tt.VFrete = decimal.RequireFromString("30.00")
	tt.VIPI = decimal.RequireFromString("5.00")
	tt.VIPIDevol = decimal.RequireFromString("1.00")
	tt.VServ = decimal.RequireFromString("300.00")

	total := buildTotal(tt, time.Now(), nil)
	icms := total["ICMSTot"].(map[string]any)
	// 1000 - 50 - 10 + 20 + 2 + 30 + 5 + 1 = 998
	if got := icms["vNF"]; got != "998.00" {
		t.Errorf("vNF = %v, want 998.00", got)
	}
	if got := icms["vProd"]; got != "700.00" {
		t.Errorf("vProd = %v, want 700.00 (sem os 300 de serviço)", got)
	}
}

// NT 2026.005 §2.3: o MEI (CRT=4) não informa o grupo IBSCBS, nem no total.
func TestBuildTotal_MEIOmiteIBSCBSTot(t *testing.T) {
	tt := newTotals(decimal.RequireFromString("100.00"), decimal.Zero)
	if _, ok := buildTotal(tt, time.Now(), nil)["IBSCBSTot"]; !ok {
		t.Fatal("CRT diferente de 4 deve emitir IBSCBSTot")
	}
	tt.OmitIBSCBS = true
	if _, ok := buildTotal(tt, time.Now(), nil)["IBSCBSTot"]; ok {
		t.Fatal("MEI não informa IBSCBSTot")
	}
}

// NT 2026.008 (I11b e I11c): vProdLiq = vUnComLiq x qCom, com 2 casas.
func TestBuildProd_ValorLiquidoDoItem(t *testing.T) {
	item := map[string]any{"quantity": "3", "unit_value": "10.00", "v_un_com_liq": "8.3333"}
	prod := buildProd(item, prodParams{VProd: "30.00", Unit: "UN", TaxableUnit: "UN", QTrib: "3", VUnTrib: "10.00"})
	if prod["vUnComLiq"] != "8.3333" || prod["vProdLiq"] != "25.00" {
		t.Fatalf("vUnComLiq/vProdLiq = %v / %v, want 8.3333 / 25.00", prod["vUnComLiq"], prod["vProdLiq"])
	}
	if _, has := buildProd(map[string]any{"quantity": "1", "unit_value": "1.00"}, prodParams{})["vProdLiq"]; has {
		t.Error("sem v_un_com_liq o item não leva vProdLiq")
	}
}

// W01a-10 e W01a-20: o total líquido só existe se os itens têm vProdLiq, e é a soma; tudo ou nada.
func TestItemsHaveNetUnitETotalLiquido(t *testing.T) {
	com := map[string]any{"v_un_com_liq": "1.00"}
	sem := map[string]any{}
	if !itemsHaveNetUnit([]map[string]any{com, com}) {
		t.Error("todos com valor líquido deviam contar")
	}
	if itemsHaveNetUnit([]map[string]any{com, sem}) || itemsHaveNetUnit(nil) {
		t.Error("parcial ou vazio não conta (tudo ou nada)")
	}
	tt := newTotals(decimal.RequireFromString("30.00"), decimal.Zero)
	tt.HasVProdLiq, tt.VProdLiq = true, decimal.RequireFromString("25.00")
	if got := buildTotal(tt, time.Now(), nil)["vProdLiqTot"]; got != "25.00" {
		t.Errorf("vProdLiqTot = %v, want 25.00", got)
	}
	if _, has := buildTotal(newTotals(decimal.Zero, decimal.Zero), time.Now(), nil)["vProdLiqTot"]; has {
		t.Error("sem itens líquidos o total não leva vProdLiqTot")
	}
}
