package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestNormalEmissionHasNoContingencyGroup(t *testing.T) {
	for model, wantTpImp := range map[string]string{nfModel55: tpImpDANFERetrato, nfModel65: tpImpDANFENFCe} {
		m := NormalEmission(model)
		if m.TpEmis != tpEmisNormal || m.TpImp != wantTpImp {
			t.Errorf("%s: got %+v", model, m)
		}
		if m.IsContingency() {
			t.Errorf("%s: normal emission must not be contingency", model)
		}
		if err := m.Validate(); err != nil {
			t.Errorf("%s: %v", model, err)
		}
	}
}

func TestContingencyRequiresTimestampAndJustification(t *testing.T) {
	base := EmissionMode{TpEmis: TpEmisSVCAN, TpImp: TpImpDANFESimpl}
	if err := base.Validate(); err == nil {
		t.Error("contingency without dhCont must be rejected")
	}

	withTime := base
	withTime.ContingencyAt = time.Now()
	if err := withTime.Validate(); err == nil {
		t.Error("contingency without xJust must be rejected")
	}

	withTime.Justification = "curta"
	if err := withTime.Validate(); err == nil {
		t.Error("xJust below 15 characters must be rejected")
	}

	withTime.Justification = "autorizador da UF indisponivel"
	if err := withTime.Validate(); err != nil {
		t.Errorf("valid contingency rejected: %v", err)
	}
}

// dhCont/xJust must appear in ide only for tpEmis != 1, and tpEmis must reach
// both ide and the access key.
func TestBuildEnviNFeEmitsContingencyGroup(t *testing.T) {
	org, receiver, productItems, payments := minimalEnviNFeArgs()
	at := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	mode := EmissionMode{
		TpEmis:        TpEmisSVCAN,
		TpImp:         TpImpDANFESimpl,
		ContingencyAt: at,
		Justification: "autorizador da UF indisponivel",
	}
	result := BuildEnviNFe(
		org, receiver, "CNPJ_11222333000181",
		productItems, payments,
		1, 1, 2,
		"35260711222333000181550010000000011600000010", decimal.Zero, decimal.Zero,
		nil, time.Now(),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{}, nfModel55, nil,
		nil, nil,
		mode,
		docExtras{},
	)
	ide := result["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)["ide"].(map[string]any)

	if ide["tpEmis"] != TpEmisSVCAN || ide["tpImp"] != TpImpDANFESimpl {
		t.Errorf("mode not applied to ide: %#v", ide)
	}
	if ide["xJust"] != mode.Justification {
		t.Errorf("xJust = %v", ide["xJust"])
	}
	if ide["dhCont"] != fmtDhEmi(at) {
		t.Errorf("dhCont = %v, want %v", ide["dhCont"], fmtDhEmi(at))
	}

	normal := BuildEnviNFe(
		org, receiver, "CNPJ_11222333000181",
		productItems, payments,
		1, 1, 2,
		"35260711222333000181550010000000011000000010", decimal.Zero, decimal.Zero,
		nil, time.Now(),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{}, nfModel55, nil,
		nil, nil,
		NormalEmission(nfModel55),
		docExtras{},
	)
	normalIde := normal["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)["ide"].(map[string]any)
	if _, ok := normalIde["dhCont"]; ok {
		t.Error("normal emission must not emit dhCont")
	}
	if _, ok := normalIde["xJust"]; ok {
		t.Error("normal emission must not emit xJust")
	}
}

// The access key embeds tpEmis at position 35 (index 34).
func TestAccessKeyCarriesTpEmis(t *testing.T) {
	now := time.Now()
	key, err := generateAccessKey("CNPJ_11222333000181", orgWithUF("SP"), 1, 1, now, nfModel55, TpEmisSVCAN)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(key[34]); got != TpEmisSVCAN {
		t.Errorf("key[34] = %q, want %q (key %s)", got, TpEmisSVCAN, key)
	}
}
