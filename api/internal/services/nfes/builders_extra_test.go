package nfes

import "testing"

func TestBuildInfAdicTodosOsDestinos(t *testing.T) {
	got := buildInfAdic(
		"Beneficio fiscal 123", "Obrigado pela preferencia",
		[]map[string]any{{"@xCampo": "Pedido", "xTexto": "42"}},
		[]map[string]any{{"@xCampo": "Regime", "xTexto": "Especial"}},
		[]map[string]any{{"nProc": "0001/2026", "indProc": "0"}},
	)
	if got["infAdFisco"] != "Beneficio fiscal 123" || got["infCpl"] != "Obrigado pela preferencia" {
		t.Fatalf("textos errados: %v", got)
	}
	if len(got["obsCont"].([]map[string]any)) != 1 || len(got["procRef"].([]map[string]any)) != 1 {
		t.Fatalf("listas ausentes: %v", got)
	}
}

func TestBuildInfAdicVazioDevolveNil(t *testing.T) {
	if buildInfAdic("", "", nil, nil, nil) != nil {
		t.Fatal("infAdic vazio tem que ser omitido, não presente e vazio")
	}
}
