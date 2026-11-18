package validation

import (
	"strings"
	"testing"
)

func TestFiscalTextAndRefCode(t *testing.T) {
	type sample struct {
		Text string `json:"text" validate:"required,fiscaltext"`
		Code string `json:"code" validate:"required,refcode"`
	}
	if p := Struct(sample{Text: "Cobrança de março: plano Fibra 100", Code: "PLANO-100_A/1.2"}); p != nil {
		t.Fatalf("válido recusado: %+v", p.Errors)
	}
	for name, s := range map[string]sample{
		"espaço na ponta":   {Text: " texto", Code: "A"},
		"espaço no fim":     {Text: "texto ", Code: "A"},
		"menor que":         {Text: "a<b", Code: "A"},
		"maior que":         {Text: "a>b", Code: "A"},
		"controle":          {Text: "linha\nnova", Code: "A"},
		"fora do latin-1":   {Text: "preço € 10", Code: "A"},
		"código com espaço": {Text: "ok", Code: "A B"},
		"código com acento": {Text: "ok", Code: "AÇÃO"},
		"código longo":      {Text: "ok", Code: strings.Repeat("A", 61)},
	} {
		if p := Struct(s); p == nil {
			t.Errorf("%s: deveria ser recusado", name)
		}
	}
}
