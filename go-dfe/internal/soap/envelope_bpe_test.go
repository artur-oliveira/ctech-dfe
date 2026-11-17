package soap

import (
	"bytes"
	"strings"
	"testing"
)

// BP-e: sem cabeçalho SOAP, corpo BPeDadosMsg no namespace do WSDL do serviço (MOC e NT 2025.002, 2.4).
func TestBuild_BPeServices(t *testing.T) {
	cases := []struct{ service, wsdl, op string }{
		{"BPeRecepcao", "BPeRecepcao", "bpeRecepcao"},
		{"BPeRecepcaoTM", "BPeRecepcaoTM", "bpeRecepcaoTM"},
		{"BPeRecepcaoTA", "BPeRecepcaoTA", "bpeRecepcaoTA"},
		{"BPeConsulta", "BPeConsulta", "bpeConsultaBP"},
		{"BPeStatusServico", "BPeStatusServico", "bpeStatusServicoBP"},
		{"BPeRecepcaoEvento", "BPeRecepcaoEvento", "bpeRecepcaoEvento"},
	}
	for _, c := range cases {
		b, err := NewBuilder("bpe", "SP", c.service)
		if err != nil {
			t.Fatalf("%s: %v", c.service, err)
		}
		env, err := b.Build([]byte("<x/>"), false, false)
		if err != nil {
			t.Fatal(err)
		}
		want := `<BPeDadosMsg xmlns="http://www.portalfiscal.inf.br/bpe/wsdl/` + c.wsdl + `"><x/></BPeDadosMsg>`
		if !bytes.Contains(env, []byte(want)) {
			t.Errorf("%s: faltou %s em %s", c.service, want, env)
		}
		if ct := b.ContentType(); !strings.Contains(ct, "/"+c.wsdl+"/"+c.op) {
			t.Errorf("%s: content type %s", c.service, ct)
		}
	}
	resp := syntheticResponse("BPeResultMsg", `<retBPe xmlns="http://www.portalfiscal.inf.br/bpe"><cStat>100</cStat></retBPe>`)
	got, err := ParseResult("bpe", resp)
	if err != nil || !bytes.Contains(got, []byte("<cStat>100</cStat>")) {
		t.Errorf("ParseResult: %v %s", err, got)
	}
}
