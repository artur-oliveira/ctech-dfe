package nfes

import (
	"testing"

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
