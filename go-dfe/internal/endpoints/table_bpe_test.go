package endpoints

import "testing"

func TestResolve_BPe(t *testing.T) {
	cases := []struct{ uf, env, service, want string }{
		{"SP", "prod", "BPeRecepcao", "https://bpe.fazenda.sp.gov.br/BPeWeb/services/BPeRecepcao.asmx"},
		{"SP", "hom", "BPeConsulta", "https://homologacao.bpe.fazenda.sp.gov.br/BPeWeb/services/BPeConsulta.asmx"},
		{"PR", "prod", "BPeRecepcaoEvento", "https://bpe.fazenda.pr.gov.br/bpe/BPeRecepcaoEvento"},
		{"PR", "hom", "BPeStatusServico", "https://homologacao.bpe.fazenda.pr.gov.br/bpe/BPeStatusServico"},
		{"MG", "prod", "BPeRecepcao", "https://bpe.fazenda.mg.gov.br/bpe/services/BPeRecepcao"},
		{"MS", "prod", "BPeRecepcaoEvento", "https://bpe.fazenda.ms.gov.br/ws/BPeRecepcaoEvento"},
		{"MT", "hom", "BPeStatusServico", "https://homologacao.sefaz.mt.gov.br/bpe-ws/services/BPeStatusServico"},
		// Sem autorizador próprio: SVRS.
		{"RJ", "prod", "BPeRecepcao", "https://bpe.svrs.rs.gov.br/ws/bpeRecepcao/bpeRecepcao.asmx"},
		{"BA", "hom", "BPeConsulta", "https://bpe-homologacao.svrs.rs.gov.br/ws/bpeConsulta/bpeConsulta.asmx"},
		// TM e TA: só a SVRS, para qualquer UF do emitente.
		{"SP", "prod", "BPeRecepcaoTM", "https://bpe.svrs.rs.gov.br/ws/bpeRecepcaoTM/bpeRecepcaoTM.asmx"},
		{"PR", "hom", "BPeRecepcaoTA", "https://bpe-homologacao.svrs.rs.gov.br/WS/BPeRecepcaoTA/BPeRecepcaoTA.asmx"},
	}
	for _, c := range cases {
		got, err := Resolve("bpe", c.uf, c.env, c.service)
		if err != nil {
			t.Errorf("%s/%s/%s: %v", c.uf, c.env, c.service, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s/%s/%s = %s, want %s", c.uf, c.env, c.service, got, c.want)
		}
	}
}

func TestResolve_BPeUnknown(t *testing.T) {
	if _, err := Resolve("bpe", "SP", "prod", "BPeInutilizacao"); err == nil {
		t.Error("BP-e não tem inutilização")
	}
	if _, err := Resolve("bpe", "ZZ", "prod", "BPeRecepcao"); err == nil {
		t.Error("UF desconhecida")
	}
}
