package ctes

import (
	"context"
	"strings"
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
)

func validLink() PaymentLink {
	return PaymentLink{Number: 1, TransactionID: "E12345678202607011000abcdef", PaymentMethod: "17", ReceiverCNPJ: "12345678000195", PSPBaseCNPJ: "00000000"}
}

func TestPaymentLinkCheck(t *testing.T) {
	if err := validLink().check(); err != nil {
		t.Fatalf("valid link refused: %v", err)
	}
	alnum := validLink()
	alnum.ReceiverCNPJ, alnum.PSPBaseCNPJ = "12ABC345000195", "AB12CD34"
	if err := alnum.check(); err != nil {
		t.Fatalf("alphanumeric CNPJ is valid in the XSD: %v", err)
	}
	for name, mut := range map[string]func(*PaymentLink){
		"nPag 0":            func(p *PaymentLink) { p.Number = 0 },
		"nPag 991":          func(p *PaymentLink) { p.Number = 991 },
		"id 1 char":         func(p *PaymentLink) { p.TransactionID = "A" },
		"id 36 chars":       func(p *PaymentLink) { p.TransactionID = strings.Repeat("A", 36) },
		"id trailing space": func(p *PaymentLink) { p.TransactionID = "AB " },
		"method 1 digit":    func(p *PaymentLink) { p.PaymentMethod = "7" },
		"method letters":    func(p *PaymentLink) { p.PaymentMethod = "AA" },
		"cnpj lowercase":    func(p *PaymentLink) { p.ReceiverCNPJ = "12abc345000195" },
		"cnpj dv letters":   func(p *PaymentLink) { p.ReceiverCNPJ = "1234567800019A" },
		"base 7 chars":      func(p *PaymentLink) { p.PSPBaseCNPJ = "1234567" },
	} {
		p := validLink()
		mut(&p)
		if p.check() == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestPaymentLinkRefusedLocally(t *testing.T) {
	p := validLink()
	p.PaymentMethod = "x"
	_, err := (&CteService{}).RegisterPaymentLink(context.Background(), "org", eventKey, p, 0, "u", "U")
	if pr, ok := err.(*problem.Problem); !ok || pr.Status != 422 || pr.Code != CodeEventInvalid {
		t.Fatalf("want 422 %s, got %#v", CodeEventInvalid, err)
	}
}

func TestPaymentEventsXMLOrder(t *testing.T) {
	x := eventXML(t, TpEventoVinculoPagamento, 1, "evVincPgto", paymentLinkDet(validLink(), parityProt))
	inOrder(t, x, "<evVincPgto>", "<descEvento>", "<nProt>", "<pgto ", "<tpMeioPgto>", "<CNPJReceb>", "<CNPJBasePSP>", "</pgto>")
	if !strings.Contains(x, `nPag="1"`) || !strings.Contains(x, `idTransacao="E12345678202607011000abcdef"`) {
		t.Fatalf("pgto attributes missing: %s", x)
	}
	x = eventXML(t, TpEventoCancVinculoPagamento, 1, "evCancVincPgto", cancPaymentLinkDet(parityProt, "135250000000002"))
	inOrder(t, x, "<evCancVincPgto>", "<descEvento>", "<nProt>", "<nProtVincPgto>")
}
