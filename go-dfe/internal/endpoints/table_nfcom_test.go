package endpoints

import "testing"

func TestResolve_NFCom(t *testing.T) {
	cases := []struct {
		uf, env, service, want string
	}{
		// SP não tem autorizador próprio: SVRS (MOC NFCom, relação de serviços).
		{"SP", "prod", "NFComRecepcao", "https://nfcom.svrs.rs.gov.br/WS/NFComRecepcao/NFComRecepcao.asmx"},
		{"SP", "hom", "NFComConsulta", "https://nfcom-homologacao.svrs.rs.gov.br/WS/NFComConsulta/NFComConsulta.asmx"},
		{"RS", "prod", "NFComStatusServico", "https://nfcom.svrs.rs.gov.br/WS/NFComStatusServico/NFComStatusServico.asmx"},
		{"PR", "hom", "NFComRecepcaoEvento", "https://nfcom-homologacao.svrs.rs.gov.br/WS/NFComRecepcaoEvento/NFComRecepcaoEvento.asmx"},
		// Autorizadores próprios.
		{"MG", "prod", "NFComRecepcao", "https://nfcom.fazenda.mg.gov.br/nfcom/services/NFComRecepcao"},
		{"MG", "hom", "NFComConsulta", "https://hnfcom.fazenda.mg.gov.br/nfcom/services/NFComConsulta"},
		{"MS", "prod", "NFComRecepcaoEvento", "https://nfcom.sefaz.ms.gov.br/ws/NFComRecepcaoEvento"},
		{"MS", "hom", "NFComStatusServico", "https://hom.nfcom.sefaz.ms.gov.br/ws/NFComStatusServico"},
		{"MT", "prod", "NFComRecepcao", "https://www.sefaz.mt.gov.br/nfcom/services/NFComRecepcao"},
		{"MT", "hom", "NFComRecepcaoEvento", "https://homologacao.sefaz.mt.gov.br/nfcom/services/NFComRecepcaoEvento"},
	}
	for _, c := range cases {
		got, err := Resolve("nfcom", c.uf, c.env, c.service)
		if err != nil {
			t.Errorf("%s/%s/%s: %v", c.uf, c.env, c.service, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s/%s/%s = %s, want %s", c.uf, c.env, c.service, got, c.want)
		}
	}
}

func TestResolve_NFComUnknown(t *testing.T) {
	if _, err := Resolve("nfcom", "SP", "prod", "NFComInutilizacao"); err == nil {
		t.Error("NFCom não tem inutilização: esperava erro")
	}
	if _, err := Resolve("nfcom", "ZZ", "prod", "NFComRecepcao"); err == nil {
		t.Error("UF desconhecida: esperava erro")
	}
	if _, err := Resolve("nfcom", AuthorizerSVCRS, "prod", "NFComRecepcao"); err == nil {
		t.Error("NFCom não tem SVC: esperava erro")
	}
}
