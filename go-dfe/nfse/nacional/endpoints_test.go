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

func TestResolveOperation_Teresina(t *testing.T) {
	const (
		hom  = "https://nfse2-the.dsfweb.com.br/notafiscal-ws"
		prod = "https://nfseapi.teresina.pi.gov.br/notafiscal-ws"
	)
	cases := []struct {
		name string
		op   Operation
		env  string
		args []any
		want string
	}{
		{"emissao hom", OpEmit, "hom", nil, hom + "/nfse"},
		{"emissao prod", OpEmit, "prod", nil, prod + "/nfse"},
		{"evento hom", OpEvent, "hom", []any{"123"}, hom + "/nfse/123/eventos"},
		{"consulta hom", OpQueryByKey, "hom", []any{"123"}, hom + "/nfse/123"},
		{"consulta dps hom", OpQueryByDPSID, "hom", []any{"DPS1"}, hom + "/nfse/dps/DPS1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveOperation(c.op, c.env, "2211001", c.args...)
			if err != nil {
				t.Fatalf("ResolveOperation: %v", err)
			}
			if got != c.want {
				t.Errorf("endpoint = %q, esperado %q", got, c.want)
			}
		})
	}
}

// Município sem autorizador próprio usa o Sefin Nacional em todas as operações.
func TestResolveOperation_OtherMunicipalityUsesNational(t *testing.T) {
	const sefin = "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional"
	cases := []struct {
		op   Operation
		args []any
		want string
	}{
		{OpEmit, nil, sefin + "/nfse"},
		{OpEvent, []any{"123"}, sefin + "/nfse/123/eventos"},
		{OpQueryByKey, []any{"123"}, sefin + "/nfse/123"},
		{OpQueryByDPSID, []any{"DPS1"}, sefin + "/dps/DPS1"},
	}
	for _, c := range cases {
		got, err := ResolveOperation(c.op, "hom", "3550308", c.args...)
		if err != nil {
			t.Fatalf("ResolveOperation(%q): %v", c.op, err)
		}
		if got != c.want {
			t.Errorf("endpoint(%q) = %q, esperado %q", c.op, got, c.want)
		}
	}
}

// Operação que o município não publica cai no nacional mesmo com base municipal
// registrada — o fallback é por operação, não pela base.
func TestResolveOperation_UnpublishedOperationFallsBack(t *testing.T) {
	saved := municipalAuthorizers[municipalityTeresina]
	municipalAuthorizers[municipalityTeresina] = municipalAuthorizer{
		bases: saved.bases,
		paths: map[Operation]string{OpEmit: PathNFSe},
	}
	t.Cleanup(func() { municipalAuthorizers[municipalityTeresina] = saved })

	got, err := ResolveOperation(OpQueryByKey, "hom", municipalityTeresina, "123")
	if err != nil {
		t.Fatalf("ResolveOperation: %v", err)
	}
	want := "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional/nfse/123"
	if got != want {
		t.Errorf("endpoint = %q, esperado %q", got, want)
	}
}

func TestResolveOperation_Unknown(t *testing.T) {
	if _, err := ResolveOperation("inexistente", "hom", "2211001"); err == nil {
		t.Error("esperado erro para operação desconhecida")
	}
}
