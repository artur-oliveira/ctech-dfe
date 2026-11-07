package nfes

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
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

func svcConfig(since, reason string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		repositories.AttrContingencyMode:   &types.AttributeValueMemberS{Value: repositories.ContingencyModeSVC},
		repositories.AttrContingencySince:  &types.AttributeValueMemberS{Value: since},
		repositories.AttrContingencyReason: &types.AttributeValueMemberS{Value: reason},
	}
}

func TestResolveNFeEmission(t *testing.T) {
	// janela fechada: normal, sem autorizador
	m, svc, err := resolveNFeEmission(map[string]types.AttributeValue{}, "SP")
	if err != nil || m.TpEmis != tpEmisNormal || svc != "" {
		t.Fatalf("normal: %+v %q %v", m, svc, err)
	}
	// SP está na SVC-AN (tpEmis 6); PR na SVC-RS (tpEmis 7)
	cfg := svcConfig("2026-11-08T10:00:00Z", "SEFAZ de origem indisponivel")
	for uf, want := range map[string][2]string{"SP": {TpEmisSVCAN, "SVC-AN"}, "PR": {TpEmisSVCRS, "SVC-RS"}} {
		m, svc, err := resolveNFeEmission(cfg, uf)
		if err != nil || m.TpEmis != want[0] || svc != want[1] || !m.IsContingency() || m.Validate() != nil {
			t.Errorf("%s: %+v %q %v", uf, m, svc, err)
		}
	}
	if _, _, err := resolveNFeEmission(cfg, "EX"); err == nil {
		t.Error("UF sem SVC deve falhar")
	}
	if _, _, err := resolveNFeEmission(svcConfig("lixo", "x"), "SP"); err == nil {
		t.Error("data inválida deve falhar")
	}
}

func TestSvcAuthorizerForKey(t *testing.T) {
	key := func(tp string) string {
		return "35261112345678000195" + "55" + "001" + "000000001" + tp + "12345678" + "0"
	}
	for tp, want := range map[string]string{"1": "", "6": "SVC-AN", "7": "SVC-RS", "4": ""} {
		if got := svcAuthorizerForKey(key(tp)); got != want {
			t.Errorf("tpEmis %s: got %q want %q", tp, got, want)
		}
	}
	if svcAuthorizerForKey("123") != "" {
		t.Error("chave malformada")
	}
}
