package bpes

import (
	"strings"
	"testing"
)

func TestQRCodeBaseByAuthorizer(t *testing.T) {
	key := func(cuf string) string { return cuf + strings.Repeat("0", 42) }
	cases := []struct {
		variant, cuf string
		env          int
		want         string
	}{
		{VariantNormal, "35", 1, "https://bpe.fazenda.sp.gov.br/BPe/qrcode"},
		{VariantNormal, "35", 2, "https://homologacao.bpe.fazenda.sp.gov.br/BPe/qrcode"},
		{VariantNormal, "51", 2, "https://homologacao.sefaz.mt.gov.br/BPe/qrcode"},
		{VariantNormal, "25", 1, "http://www.receita.pb.gov.br/bpe/qrCode"},
		{VariantNormal, "33", 1, bpeQRCodeSVRS}, // RJ: sem página própria
		{VariantTA, "35", 1, bpeQRCodeSVRS},     // TA só na SVRS, qualquer UF
	}
	for _, c := range cases {
		if got := qrCodeBase(c.variant, key(c.cuf), c.env); got != c.want {
			t.Errorf("%s cUF %s env %d = %s, want %s", c.variant, c.cuf, c.env, got, c.want)
		}
	}
}
