package soap

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

// NFCom: sem cabeçalho SOAP (o client nunca o envia), corpo nfcomDadosMsg no namespace do WSDL do serviço, a
// autorização com GZip+Base64 e o método nfcomRecepcao (MOC NFCom, páginas de cada serviço).
func TestBuild_NFComServices(t *testing.T) {
	cases := []struct{ service, wsdl, op string }{
		{"NFComRecepcao", "NFComRecepcao", "nfcomRecepcao"},
		{"NFComConsulta", "NFComConsulta", "nfcomConsultaNF"},
		{"NFComStatusServico", "NFComStatusServico", "nfcomStatusServicoNF"},
		{"NFComRecepcaoEvento", "NFComRecepcaoEvento", "nfcomRecepcaoEvento"},
	}
	for _, c := range cases {
		b, err := NewBuilder("nfcom", "SP", c.service)
		if err != nil {
			t.Fatalf("%s: %v", c.service, err)
		}
		env, err := b.Build([]byte("<x/>"), false, false)
		if err != nil {
			t.Fatal(err)
		}
		want := `<nfcomDadosMsg xmlns="http://www.portalfiscal.inf.br/nfcom/wsdl/` + c.wsdl + `"><x/></nfcomDadosMsg>`
		if !bytes.Contains(env, []byte(want)) {
			t.Errorf("%s: faltou %s em %s", c.service, want, env)
		}
		if ct := b.ContentType(); !strings.Contains(ct, "/"+c.wsdl+"/"+c.op) {
			t.Errorf("%s: content type %s", c.service, ct)
		}
	}
}

func TestBuild_NFComGzipRoundTrip(t *testing.T) {
	b, err := NewBuilder("nfcom", "SP", "NFComRecepcao")
	if err != nil {
		t.Fatal(err)
	}
	env, _ := b.Build([]byte("<NFCom/>"), true, false)
	s := string(env)
	i := strings.Index(s, ">") // fim do envelope aberto não importa: pega o corpo entre as tags do body
	_ = i
	start := strings.Index(s, `<nfcomDadosMsg`)
	body := s[start:]
	body = body[strings.Index(body, ">")+1 : strings.Index(body, "</nfcomDadosMsg>")]
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(zr)
	if string(out) != "<NFCom/>" {
		t.Errorf("got %q", out)
	}
	resp := syntheticResponse("nfcomResultMsg", `<retNFCom xmlns="http://www.portalfiscal.inf.br/nfcom"><cStat>100</cStat></retNFCom>`)
	got, err := ParseResult("nfcom", resp)
	if err != nil || !bytes.Contains(got, []byte("<cStat>100</cStat>")) {
		t.Errorf("ParseResult: %v %s", err, got)
	}
}
