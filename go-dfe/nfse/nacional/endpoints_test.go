package nacional

import "testing"

func TestResolveBase(t *testing.T) {
	cases := []struct {
		system, env, want string
	}{
		{SystemSefin, "prod", "https://sefin.nfse.gov.br/SefinNacional"},
		{SystemSefin, "hom", "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional"},
		{SystemADN, "prod", "https://adn.nfse.gov.br/contribuintes"},
		{SystemADN, "hom", "https://adn.producaorestrita.nfse.gov.br/contribuintes"},
		{SystemDANFSE, "prod", "https://adn.nfse.gov.br/danfse"},
		{SystemDANFSE, "hom", "https://adn.producaorestrita.nfse.gov.br/danfse"},
		{SystemParametros, "prod", "https://adn.nfse.gov.br/parametrizacao"},
		{SystemParametros, "hom", "https://adn.producaorestrita.nfse.gov.br/parametrizacao"},
	}
	for _, c := range cases {
		got, err := ResolveBase(c.system, c.env)
		if err != nil {
			t.Fatalf("ResolveBase(%q,%q): %v", c.system, c.env, err)
		}
		if got != c.want {
			t.Errorf("ResolveBase(%q,%q) = %q, esperado %q", c.system, c.env, got, c.want)
		}
	}
}

func TestResolveBase_Unknown(t *testing.T) {
	if _, err := ResolveBase("inexistente", "prod"); err == nil {
		t.Error("esperado erro para sistema desconhecido")
	}
	if _, err := ResolveBase(SystemSefin, "staging"); err == nil {
		t.Error("esperado erro para ambiente desconhecido")
	}
}
