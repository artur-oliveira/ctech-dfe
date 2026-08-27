package mdfes

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

func ptr(s string) *string { return &s }

func basePayment() MdfePayment {
	return MdfePayment{
		Name:          ptr("Transportadora Exemplo"),
		CNPJ:          ptr("11.222.333/0001-81"),
		Components:    []MdfePaymentComponent{{Type: "01", Value: "1500.00"}},
		ContractValue: "1500.00",
		PaymentType:   "0",
		Bank:          MdfePaymentBank{PIX: ptr("chave@exemplo.com")},
	}
}

func TestBuildInfPagHappyPath(t *testing.T) {
	got, err := buildInfPag([]MdfePayment{basePayment()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d infPag, want 1", len(got))
	}
	node := got[0]
	if node["CNPJ"] != "11222333000181" {
		t.Errorf("CNPJ not normalized: %v", node["CNPJ"])
	}
	if _, ok := node["CPF"]; ok {
		t.Error("CPF present alongside CNPJ (choice violation)")
	}
	if node["vContrato"] != "1500.00" || node["indPag"] != "0" {
		t.Errorf("unexpected contract fields: %#v", node)
	}
	if banc := node["infBanc"].(map[string]any); banc["PIX"] != "chave@exemplo.com" || len(banc) != 1 {
		t.Errorf("infBanc must carry exactly one choice member: %#v", banc)
	}
	if _, ok := node["infPrazo"]; ok {
		t.Error("à vista must not emit infPrazo")
	}
}

func TestBuildInfPagRejections(t *testing.T) {
	noDoc := basePayment()
	noDoc.CNPJ = nil

	comp99 := basePayment()
	comp99.Components = []MdfePaymentComponent{{Type: "99", Value: "10.00"}}

	term := basePayment()
	term.PaymentType = "1"

	bankNoAgency := basePayment()
	bankNoAgency.Bank = MdfePaymentBank{BankCode: ptr("001")}

	noBank := basePayment()
	noBank.Bank = MdfePaymentBank{}

	for name, p := range map[string]MdfePayment{
		"missing payee document":  noDoc,
		"tpComp 99 without xComp": comp99,
		"a prazo without parcels": term,
		"bank without agency":     bankNoAgency,
		"no bank choice":          noBank,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := buildInfPag([]MdfePayment{p}); err == nil {
				t.Fatal("expected rejection, got nil")
			} else if _, ok := err.(*problem.Problem); !ok {
				t.Fatalf("expected *problem.Problem, got %T", err)
			}
		})
	}

	if _, err := buildInfPag(nil); err == nil {
		t.Error("empty payment list must be rejected")
	}
}

func TestBuildInfPagTermPayment(t *testing.T) {
	p := basePayment()
	p.PaymentType = "1"
	p.Installments = []MdfePaymentInstallment{
		{Number: "1", DueDate: "2026-09-10", Value: "750.00"},
		{Number: "2", DueDate: "2026-10-10", Value: "750.00"},
	}
	got, err := buildInfPag([]MdfePayment{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parcels := got[0]["infPrazo"].([]map[string]any)
	if len(parcels) != 2 || parcels[1]["dVenc"] != "2026-10-10" {
		t.Errorf("unexpected infPrazo: %#v", parcels)
	}
}
