package mdfes

import (
	"testing"
	"time"
)

func strPtr(v string) *string { return &v }

func TestExpandMdfeInstallmentsDerivaPrazo(t *testing.T) {
	got := expandMdfeInstallments(MdfePaymentBody{
		ContractValue: "3000.00", Installments: 2, IntervalDays: 15, FirstDueDays: 15,
	}, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	if len(got) != 2 {
		t.Fatalf("parcelas não derivadas: %v", got)
	}
	if got[0].DueDate != "2026-09-11" || got[0].Value != "1500.00" || got[0].Number != "001" {
		t.Fatalf("primeira parcela errada: %+v", got[0])
	}
	if got[1].DueDate != "2026-09-26" {
		t.Fatalf("intervalo ignorado: %+v", got[1])
	}
}

// O adiantamento sai do que é parcelado: quem já recebeu R$ 1.000 de R$ 3.000
// tem duas parcelas de R$ 1.000, não de R$ 1.500.
func TestExpandMdfeInstallmentsDescontaAdiantamento(t *testing.T) {
	got := expandMdfeInstallments(MdfePaymentBody{
		ContractValue: "3000.00", AdvanceValue: strPtr("1000.00"), Installments: 2,
	}, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	if got[0].Value != "1000.00" || got[1].Value != "1000.00" {
		t.Fatalf("parcelas ignoraram o adiantamento: %+v", got)
	}
}

// A prazo sem número de parcelas é parcela única — infPrazo vazio o XSD recusa.
func TestExpandMdfeInstallmentsParcelaUnicaPorPadrao(t *testing.T) {
	got := expandMdfeInstallments(MdfePaymentBody{ContractValue: "800.00"}, time.Now())
	if len(got) != 1 || got[0].Value != "800.00" {
		t.Fatalf("want parcela única: %+v", got)
	}
}

func TestPersonBankLeCadastro(t *testing.T) {
	got := personBank(map[string]any{"bank": map[string]any{"pix_key": "joao@pix.com"}})
	if got.PIX == nil || *got.PIX != "joao@pix.com" {
		t.Fatalf("PIX ausente: %+v", got)
	}
	if got.BankCode != nil {
		t.Fatalf("campo vazio virou string vazia em vez de nil: %+v", got)
	}
	if empty := personBank(map[string]any{}); empty.PIX != nil {
		t.Fatalf("pessoa sem bank devia devolver choice vazio: %+v", empty)
	}
}

// A emissão reusa buildInfPag dos eventos: o grupo é o mesmo do leiaute.
func TestBuildInfPagNaEmissaoIncluiIndAltoDesemp(t *testing.T) {
	got, err := buildInfPag([]MdfePayment{{
		Name: strPtr("João"), CPF: strPtr("11144477735"),
		Components:      []MdfePaymentComponent{{Type: "01", Value: "3000.00"}},
		ContractValue:   "3000.00",
		PaymentType:     paymentIndicatorTerm,
		HighPerformance: strPtr("1"),
		Installments:    []MdfePaymentInstallment{{Number: "001", DueDate: "2026-09-11", Value: "3000.00"}},
		Bank:            MdfePaymentBank{PIX: strPtr("joao@pix.com")},
	}})
	if err != nil {
		t.Fatalf("buildInfPag: %v", err)
	}
	if got[0]["indAltoDesemp"] != "1" {
		t.Fatalf("indAltoDesemp ausente: %v", got[0])
	}
}

func TestBuildInfANTTIncluiPagamento(t *testing.T) {
	p := buildParams{infPag: []map[string]any{{"xNome": "João"}}}
	if len(p.buildInfANTT()["infPag"].([]map[string]any)) != 1 {
		t.Fatalf("infPag ausente do infANTT")
	}
}

func TestValidateFreightDeclarationContratanteExigePagamento(t *testing.T) {
	req := MdfeEmitBody{Contractors: []MdfeContractorBody{{PersonDoc: "11111111111111"}}}
	if err := validateFreightDeclaration(req); err == nil {
		t.Fatal("contratante sem pagamento tinha que ser recusado")
	}
	req.Payments = []MdfePaymentBody{{PersonDoc: "11144477735"}}
	if err := validateFreightDeclaration(req); err != nil {
		t.Fatalf("contratante com pagamento é válido: %v", err)
	}
	if err := validateFreightDeclaration(MdfeEmitBody{}); err != nil {
		t.Fatalf("frota própria não declara nem contratante nem pagamento: %v", err)
	}
}
