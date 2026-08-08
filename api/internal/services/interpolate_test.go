package services

import (
	"strings"
	"testing"
)

func TestValidatePlaceholders_AcceptsEveryKnownKey(t *testing.T) {
	var b strings.Builder
	for _, k := range AllPlaceholders {
		b.WriteString("{{" + k + "}} ")
	}
	if err := ValidatePlaceholders(b.String()); err != nil {
		t.Fatalf("ValidatePlaceholders: %v", err)
	}
}

func TestValidatePlaceholders_RejectsUnknownKey(t *testing.T) {
	err := ValidatePlaceholders("Total {{v_nf}}, imposto {{v_iss}}")
	if err == nil {
		t.Fatal("esperada recusa de placeholder desconhecido")
	}
	// A mensagem tem que listar o que existe, senão o usuário adivinha.
	if !strings.Contains(err.Error(), "v_iss") || !strings.Contains(err.Error(), PlaceholderVNF) {
		t.Errorf("mensagem pouco acionável: %v", err)
	}
}

func TestInterpolate(t *testing.T) {
	got, err := Interpolate(
		"NF {{v_nf}} para {{ cliente }} — {{nat_op}}",
		map[string]string{
			PlaceholderVNF:     "1.234,56",
			PlaceholderCliente: "ACME",
			PlaceholderNatOp:   "Venda",
		},
	)
	if err != nil {
		t.Fatalf("Interpolate: %v", err)
	}
	want := "NF 1.234,56 para ACME — Venda"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Chave conhecida sem valor vira vazio: ICMS ST zerado é ausência legítima,
// não erro de emissão.
func TestInterpolate_KnownKeyWithoutValueBecomesEmpty(t *testing.T) {
	got, err := Interpolate("ST: {{v_icms_st}}.", nil)
	if err != nil {
		t.Fatalf("Interpolate: %v", err)
	}
	if got != "ST: ." {
		t.Errorf("got %q", got)
	}
}

// Um texto com chave desconhecida nunca é interpolado — falharia silencioso.
func TestInterpolate_RefusesUnknownKey(t *testing.T) {
	if _, err := Interpolate("{{nao_existe}}", nil); err == nil {
		t.Fatal("esperada recusa")
	}
}

func TestInterpolate_TextWithoutPlaceholdersIsUnchanged(t *testing.T) {
	const text = "Documento emitido em regime especial."
	got, err := Interpolate(text, nil)
	if err != nil || got != text {
		t.Errorf("got %q, err %v", got, err)
	}
}
