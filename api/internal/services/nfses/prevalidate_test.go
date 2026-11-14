package nfses

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

const (
	testCNPJPrest = "11222333000181"
	testCNPJTom   = "11444777000161"
	testCPFTom    = "52998224725"
)

func validDoc() nfse.Document {
	return nfse.Document{
		DhEmi:       "2026-11-14T10:00:00-03:00",
		Competencia: "2026-11-14",
		Prestador:   nfse.Prestador{Pessoa: nfse.Pessoa{CNPJ: testCNPJPrest}},
		Tomador:     &nfse.Pessoa{CNPJ: testCNPJTom},
		Servico:     nfse.Servico{CServ: nfse.CServ{XDescServ: "Servico"}},
		Valores: nfse.Valores{
			VServPrest: nfse.VServPrest{VServ: "100.00"},
			Trib:       nfse.Tributacao{TribMun: nfse.TribMunicipal{PAliq: "2.00", TpRetISSQN: 1}},
		},
	}
}

func TestPreValidate(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*nfse.Document)
		want string // "" = aceita
	}{
		{"valido", func(*nfse.Document) {}, ""},
		{"tomador CPF valido", func(d *nfse.Document) { d.Tomador = &nfse.Pessoa{CPF: testCPFTom} }, ""},
		{"E0202 prestador igual tomador", func(d *nfse.Document) { d.Tomador = &nfse.Pessoa{CNPJ: testCNPJPrest} }, "E0202"},
		{"E0250 intermediario igual prestador", func(d *nfse.Document) { d.Intermediario = &nfse.Pessoa{CNPJ: testCNPJPrest} }, "E0250"},
		{"E0080 CNPJ prestador invalido", func(d *nfse.Document) { d.Prestador.CNPJ = "11222333000182" }, "E0080"},
		{"E0188 CNPJ tomador invalido", func(d *nfse.Document) { d.Tomador.CNPJ = "11111111111111" }, "E0188"},
		{"E0206 CPF tomador invalido", func(d *nfse.Document) { d.Tomador = &nfse.Pessoa{CPF: "52998224726"} }, "E0206"},
		{"E0015 competencia futura", func(d *nfse.Document) { d.Competencia = "2026-11-15" }, "E0015"},
		{"competencia anterior ok", func(d *nfse.Document) { d.Competencia = "2026-10-31" }, ""},
		{"E0341 sem descricao", func(d *nfse.Document) { d.Servico.CServ.XDescServ = "  " }, "E0341"},
		{"E0404 valor zero", func(d *nfse.Document) { d.Valores.VServPrest.VServ = "0.00" }, "E0404"},
		{"E0429 aliquota acima de 5", func(d *nfse.Document) { d.Valores.Trib.TribMun.PAliq = "5.01" }, "E0429"},
		{"aliquota 5 ok", func(d *nfse.Document) { d.Valores.Trib.TribMun.PAliq = "5.00" }, ""},
		{"E0204 retencao sem tomador identificado", func(d *nfse.Document) {
			d.Tomador = nil
			d.Valores.Trib.TribMun.TpRetISSQN = 2
		}, "E0204"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := validDoc()
			c.mut(&d)
			err := preValidate(d)
			if c.want == "" {
				if err != nil {
					t.Fatalf("esperava aceitar, veio %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("esperava %s, veio %v", c.want, err)
			}
		})
	}
}
