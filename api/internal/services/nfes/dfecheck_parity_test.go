package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// Paridade builder x XSD: toda tag que o builder emite existe no leiaute (inclui o overlay das NTs) e os
// valores respeitam tipo, tamanho e casas decimais. Falta de campo obrigatório não conta aqui: os argumentos
// mínimos do teste não preenchem o documento inteiro.
func TestBuildEnviNFe_ParidadeComXSD(t *testing.T) {
	for _, model := range []string{nfModel55, nfModel65} {
		org, receiver, productItems, payments := minimalEnviNFeArgs()
		body := BuildEnviNFe(org, receiver, "CNPJ_11222333000181", productItems, payments,
			1, 1, 2, "35260711222333000181550010000000011600000010", decimal.Zero, decimal.Zero,
			nil, time.Now(), nil, "1", "1", "1", "1", nil, nil, nil, nil,
			TechData{}, model, nil, nil, nil, EmissionMode{}, docExtras{})
		env := body["enviNFe"].(map[string]any)
		for _, v := range dfecheck.Shadow(dfecheck.SpecNFe, "nfe", "teste", env["NFe"].(map[string]any)) {
			if v.Code == "required" || v.Code == "choice_missing" || v.Code == "rule" {
				continue
			}
			t.Errorf("modelo %s: %s %s: %s", model, v.Path, v.Code, v.Message)
		}
	}
}
