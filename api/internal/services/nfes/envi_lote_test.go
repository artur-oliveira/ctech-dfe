package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// NT 2023.002 v1.02 (GAP03a-4 e GAP03a-6): um lote só aceita 1 NFC-e (rejeição 126) e a
// autorização é sempre síncrona (indSinc=0 vira rejeição 129). O builder emite um documento
// por lote e indSinc fixo em 1, para NF-e e NFC-e.
func TestBuildEnviNFe_UmDocumentoPorLoteESincrono(t *testing.T) {
	for _, model := range []string{nfModel55, nfModel65} {
		org, receiver, productItems, payments := minimalEnviNFeArgs()
		result := BuildEnviNFe(
			org, receiver, "CNPJ_11222333000181",
			productItems, payments,
			1, 1, 2,
			"35260711222333000181550010000000011600000010", decimal.Zero, decimal.Zero,
			nil, time.Now(),
			nil, "1", "1", "1", "1",
			nil, nil, nil, nil,
			TechData{}, model, nil,
			nil, nil,
			EmissionMode{},
			docExtras{},
		)
		envi := result["enviNFe"].(map[string]any)
		if envi["indSinc"] != "1" {
			t.Errorf("modelo %s: indSinc = %v, want 1", model, envi["indSinc"])
		}
		if _, ok := envi["NFe"].(map[string]any); !ok {
			t.Errorf("modelo %s: NFe deve ser um único documento por lote, got %T", model, envi["NFe"])
		}
	}
}
