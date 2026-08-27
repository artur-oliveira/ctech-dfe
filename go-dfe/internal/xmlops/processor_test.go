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

// Inutilização: a raiz do request É o documento (inutNFe), então firstByLocal
// precisa achá-la incluindo a própria raiz na busca.
func TestBuildProcessedXML_Inutilizacao(t *testing.T) {
	req := []byte(`<inutNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><infInut Id="ID2226..."><nNFIni>4</nNFIni><nNFFin>5</nNFFin></infInut><Signature xmlns="http://www.w3.org/2000/09/xmldsig#"><SignatureValue>abc</SignatureValue></Signature></inutNFe>`)
	resp := []byte(`<nfeResultMsg><retInutNFe versao="4.00" xmlns="http://www.portalfiscal.inf.br/nfe"><infInut><cStat>102</cStat><nProt>322260000058123</nProt></infInut></retInutNFe></nfeResultMsg>`)

	for _, docType := range []string{"nfe", "nfce"} {
		xml, ok := BuildProcessedXML(docType, "NfeInutilizacao", req, resp)
		if !ok {
			t.Fatalf("%s: expected ok=true for NfeInutilizacao", docType)
		}
		for _, want := range []string{
			"<ProcInutNFe", `versao="4.00"`, "<inutNFe", "<retInutNFe",
			"<cStat>102</cStat>", "<nProt>322260000058123</nProt>",
			"<SignatureValue>abc</SignatureValue>", // a assinatura do request precisa sobreviver
		} {
			if !strings.Contains(xml, want) {
				t.Errorf("%s: processed XML missing %q: %s", docType, want, xml)
			}
		}
		if strings.Contains(xml, "nfeResultMsg") {
			t.Errorf("%s: SOAP wrapper leaked into the processed XML: %s", docType, xml)
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
