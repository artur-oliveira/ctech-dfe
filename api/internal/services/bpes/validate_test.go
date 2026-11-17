package bpes

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestValidateEmitBody(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(b *BpeEmitBody)
		wantErr string // vazio: válido
	}{
		{"corpo válido", func(b *BpeEmitBody) {}, ""},
		{"componente com valor inválido", func(b *BpeEmitBody) { b.Components[0].Value = "abc" }, "componente"},
		{"componente negativo", func(b *BpeEmitBody) { b.Components[0].Value = "-1" }, "componente"},
		{"desconto sem tipo", func(b *BpeEmitBody) { b.DiscountType = nil }, "tipo do desconto"},
		{"tipo de desconto sem descrição", func(b *BpeEmitBody) { b.DiscountDesc = nil }, "descrição do desconto"},
		{"desconto zero com tipo", func(b *BpeEmitBody) { b.Discount = strp("0") }, "maior que zero"},
		{"desconto acima do valor", func(b *BpeEmitBody) { b.Discount = strp("500.00") }, "desconto"},
		{"sem desconto e sem tipo", func(b *BpeEmitBody) {
			b.Discount, b.DiscountType, b.DiscountDesc = nil, nil, nil
			b.Payments[0].Value = "100.00"
		}, ""},
		{"pagamento insuficiente", func(b *BpeEmitBody) { b.Payments[0].Value = "50.00" }, "pagamentos"},
		{"pagamento com troco", func(b *BpeEmitBody) { b.Payments[0].Value = "100.00" }, ""},
		{"alíquota do ICMS 00 ausente", func(b *BpeEmitBody) { b.ICMS = BpeICMS{CST: "00"} }, "alíquota"},
		{"comprador sem documento", func(b *BpeEmitBody) { b.Buyer = &BpeBuyer{Name: "Comprador", Address: BpeAddress{}} }, "documento do comprador"},
		{"embarque depois da validade", func(b *BpeEmitBody) { b.Passage.Validity = "2026-11-01T10:00:00-03:00" }, "validade"},
		{"conexão exige a data-hora", func(b *BpeEmitBody) { b.Trips[0].LegType = "3" }, "conexão"},
		{"data-hora sem conexão", func(b *BpeEmitBody) { b.Trips[0].Connection = strp("2026-11-20T12:00:00-03:00") }, "conexão"},
		{"data-hora inválida", func(b *BpeEmitBody) { b.Passage.Boarding = "amanhã" }, "data-hora"},
		{"cartão sem dados", func(b *BpeEmitBody) { b.Payments[0].Type = "03" }, "cartão"},
	}
	for _, c := range cases {
		b := testBody()
		c.mutate(&b)
		err := validateEmitBody(b)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: erro inesperado: %v", c.name, err)
		case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
			t.Errorf("%s: erro = %v, quer conter %q", c.name, err, c.wantErr)
		}
	}
}

func TestResolveContingencyReason(t *testing.T) {
	cfgReason := strp("Falha de comunicação com a SEFAZ autorizadora")
	if _, err := resolveContingencyReason(nil, nil); err == nil {
		t.Error("sem justificativa a contingência offline deve ser recusada")
	}
	if got, err := resolveContingencyReason(nil, cfgReason); err != nil || got != *cfgReason {
		t.Errorf("usa a justificativa da configuração: %q %v", got, err)
	}
	own := strp("Queda do link de dados da empresa")
	if got, _ := resolveContingencyReason(own, cfgReason); got != *own {
		t.Errorf("a justificativa do corpo vence: %q", got)
	}
	if _, err := resolveContingencyReason(strp("curta"), nil); err == nil {
		t.Error("justificativa com menos de 15 caracteres")
	}
}

func TestAuthorizerUFRoutesTMAndTAToSVRS(t *testing.T) {
	for variant, want := range map[string]string{VariantNormal: "SP", "": "SP", VariantTM: "SVRS", VariantTA: "SVRS"} {
		if got := authorizerUF(variant, "SP"); got != want {
			t.Errorf("authorizerUF(%q) = %q, want %q", variant, got, want)
		}
	}
}

func TestValidateTABodyRejectsBadRates(t *testing.T) {
	for _, rate := range []string{"abc", "12,5", "-5"} {
		p := testTAParams(t)
		r := rate
		p.ta.Tax.IBSUFRate = &r
		if err := validateTABody(p.ta); err == nil {
			t.Errorf("alíquota %q deveria ser recusada", rate)
		}
	}
}

func TestTAComponentAcceptsServiceFee(t *testing.T) {
	if err := validator.New().Struct(BpeTAComponent{Type: "07", Value: "1.00"}); err != nil {
		t.Errorf("07 vale no TA: %v", err)
	}
	if err := validator.New().Struct(BpeTAComponent{Type: "02", Value: "1.00"}); err == nil {
		t.Error("02 não existe no TA")
	}
}
