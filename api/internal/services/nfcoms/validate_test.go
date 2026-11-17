package nfcoms

import (
	"strings"
	"testing"
)

func TestValidateEmitBody(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(b *NfcomEmitBody)
		wantErr string // vazio: válido
	}{
		{"corpo válido", func(b *NfcomEmitBody) {}, ""},
		{"sem documento do cliente", func(b *NfcomEmitBody) { b.Customer.CNPJ = nil }, "exatamente um"},
		{"cnpj e cpf juntos", func(b *NfcomEmitBody) { b.Customer.CPF = strp("52998224725") }, "exatamente um"},
		{"fatura obrigatória no faturamento normal", func(b *NfcomEmitBody) { b.Invoice = nil }, "fatura"},
		{"pré-pago dispensa a fatura", func(b *NfcomEmitBody) { b.Invoice = nil; b.Prepaid = true }, ""},
		{"CFOP obrigatório com CST", func(b *NfcomEmitBody) { b.Items[0].CFOP = nil }, "CFOP"},
		{"sem CST dispensa o CFOP", func(b *NfcomEmitBody) { b.Items[0].CFOP = nil; b.Items[0].ICMS = NfcomICMS{CST: icmsSemCST} }, ""},
		{"sem CST proíbe o CFOP", func(b *NfcomEmitBody) { b.Items[0].ICMS = NfcomICMS{CST: icmsSemCST} }, "CFOP"},
		{"alíquota do ICMS 00", func(b *NfcomEmitBody) { b.Items[0].ICMS = NfcomICMS{CST: "00"} }, "alíquota"},
		{"quantidade zero", func(b *NfcomEmitBody) { b.Items[0].Quantity = "0" }, "quantidade"},
		{"valor negativo", func(b *NfcomEmitBody) { b.Items[0].UnitValue = "-1" }, "valor"},
		{"desconto acima do item", func(b *NfcomEmitBody) { b.Items[0].Discount = strp("1000.00") }, "desconto"},
		{"fim do contrato antes do início", func(b *NfcomEmitBody) { b.Subscriber.ContractEnd = strp("2025-01-01") }, "contrato"},
		{"IE do contribuinte ausente", func(b *NfcomEmitBody) { b.Customer.IEIndicator = "1" }, "IE"},
		{"IE em não contribuinte", func(b *NfcomEmitBody) { b.Customer.IE = strp("123456") }, "IE"},
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
	own := strp("Queda do link de dados do provedor")
	if got, _ := resolveContingencyReason(own, cfgReason); got != *own {
		t.Errorf("a justificativa do corpo vence: %q", got)
	}
	if _, err := resolveContingencyReason(strp("curta"), nil); err == nil {
		t.Error("justificativa com menos de 15 caracteres")
	}
}
