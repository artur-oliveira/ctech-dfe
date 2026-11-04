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
