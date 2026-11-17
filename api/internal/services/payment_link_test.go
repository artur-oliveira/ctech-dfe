package services

import "testing"

func TestPaymentLinkCheck(t *testing.T) {
	ok := PaymentLink{Number: 1, TransactionID: "TX-123456", PaymentMethod: "17", ReceiverCNPJ: "12345678000190", PSPBaseCNPJ: "12345678"}
	if err := ok.Check(); err != nil {
		t.Fatalf("válido: %v", err)
	}
	for name, mut := range map[string]func(*PaymentLink){
		"nPag zero":      func(p *PaymentLink) { p.Number = 0 },
		"nPag 991":       func(p *PaymentLink) { p.Number = 991 },
		"id com espaço":  func(p *PaymentLink) { p.TransactionID = " TX" },
		"id curto":       func(p *PaymentLink) { p.TransactionID = "T" },
		"meio de pgto":   func(p *PaymentLink) { p.PaymentMethod = "1" },
		"cnpj minúsculo": func(p *PaymentLink) { p.ReceiverCNPJ = "1234567800019a" },
		"base curta":     func(p *PaymentLink) { p.PSPBaseCNPJ = "1234567" },
	} {
		p := ok
		mut(&p)
		if p.Check() == nil {
			t.Errorf("%s deveria ser recusado", name)
		}
	}
}
