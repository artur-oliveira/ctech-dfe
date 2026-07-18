package xmlops

import (
	"strings"
	"testing"
)

func TestBuildProcessedXML_Emission(t *testing.T) {
	req := []byte(`<NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe123"><ide><cUF>35</cUF></ide></infNFe></NFe>`)
	resp := []byte(`<retEnviNFe><protNFe><infProt><cStat>100</cStat></infProt></protNFe></retEnviNFe>`)

	xml, ok := BuildProcessedXML("nfe", "NFeAutorizacao", req, resp)
	if !ok {
		t.Fatal("expected ok=true for NFeAutorizacao")
	}
	for _, want := range []string{"<nfeProc", `versao="4.00"`, "<infNFe", "<protNFe>", "<cStat>100</cStat>"} {
		if !strings.Contains(xml, want) {
			t.Errorf("processed XML missing %q: %s", want, xml)
		}
	}
}

func TestBuildProcessedXML_Event(t *testing.T) {
	req := []byte(`<envEvento xmlns="http://www.portalfiscal.inf.br/nfe"><evento versao="1.00"><infEvento Id="ID1">Cancelamento</infEvento></evento></envEvento>`)
	resp := []byte(`<retEnvEvento><retEvento><infEvento><cStat>135</cStat></infEvento></retEvento></retEnvEvento>`)

	xml, ok := BuildProcessedXML("nfe", "RecepcaoEvento", req, resp)
	if !ok {
		t.Fatal("expected ok=true for RecepcaoEvento")
	}
	for _, want := range []string{"<procEventoNFe", `versao="1.00"`, "<evento ", "<retEvento>", "<cStat>135</cStat>"} {
		if !strings.Contains(xml, want) {
			t.Errorf("processed XML missing %q: %s", want, xml)
		}
	}
}

func TestBuildProcessedXML_UnsignedServiceReturnsFalse(t *testing.T) {
	if _, ok := BuildProcessedXML("nfe", "NfeStatusServico", []byte(`<a/>`), []byte(`<b/>`)); ok {
		t.Error("expected ok=false for a service with no processed form")
	}
}

func TestBuildProcessedXML_MalformedInputReturnsFalse(t *testing.T) {
	if _, ok := BuildProcessedXML("nfe", "NFeAutorizacao", []byte(`not xml`), []byte(`<retEnviNFe/>`)); ok {
		t.Error("expected ok=false for malformed request XML")
	}
}
