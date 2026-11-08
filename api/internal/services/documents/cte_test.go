package documents

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

const testCTeKey = "35260812345678000190570010000000011000000019"

func sampleCTeXML() string {
	return `<cteProc xmlns="http://www.portalfiscal.inf.br/cte"><CTe><infCte Id="CTe` + testCTeKey + `"><ide><mod>57</mod><serie>1</serie><nCT>1</nCT><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><tpEmis>1</tpEmis><tpAmb>2</tpAmb><tpCTe>0</tpCTe><tpServ>0</tpServ><modal>01</modal><CFOP>6353</CFOP><natOp>PRESTACAO DE SERVICO DE TRANSPORTE</natOp><xMunIni>Sao Paulo</xMunIni><UFIni>SP</UFIni><xMunFim>Rio de Janeiro</xMunFim><UFFim>RJ</UFFim><toma3><toma>0</toma></toma3></ide><compl><xObs>Observacao teste</xObs></compl><emit><CNPJ>12345678000190</CNPJ><IE>123</IE><xNome>TRANSPORTADORA TESTE</xNome><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF></enderEmit></emit><rem><CNPJ>98765432000110</CNPJ><IE>456</IE><xNome>REMETENTE TESTE</xNome><enderReme><xLgr>Rua B</xLgr><nro>2</nro><xBairro>Centro</xBairro><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF><xPais>Brasil</xPais></enderReme></rem><dest><CPF>11122233344</CPF><xNome>DESTINATARIO TESTE</xNome><enderDest><xLgr>Rua C</xLgr><nro>3</nro><xBairro>Centro</xBairro><xMun>Rio de Janeiro</xMun><CEP>20000000</CEP><UF>RJ</UF></enderDest></dest><vPrest><vTPrest>1500.50</vTPrest><vRec>1500.50</vRec><Comp><xNome>FRETE VALOR</xNome><vComp>1500.50</vComp></Comp></vPrest><imp><ICMS><ICMS00><CST>00</CST><vBC>1500.50</vBC><pICMS>12.00</pICMS><vICMS>180.06</vICMS></ICMS00></ICMS></imp><infCTeNorm><infCarga><vCarga>10000.00</vCarga><proPred>MERCADORIA TESTE</proPred><infQ><cUnid>02</cUnid><tpMed>PESO</tpMed><qCarga>2.5000</qCarga></infQ><infQ><cUnid>03</cUnid><tpMed>VOLUMES</tpMed><qCarga>10.0000</qCarga></infQ></infCarga><infDoc><infNFe><chave>` + testNFeKey + `</chave></infNFe></infDoc><infModal versaoModal="4.00"><rodo><RNTRC>12345678</RNTRC></rodo></infModal></infCTeNorm></infCte><infCTeSupl><qrCodCTe>https://example.invalid/cte?chCTe=` + testCTeKey + `</qrCodCTe></infCTeSupl></CTe><protCTe><infProt><chCTe>` + testCTeKey + `</chCTe><nProt>135260000000009</nProt><dhRecbto>2026-08-29T12:00:01-03:00</dhRecbto></infProt></protCTe></cteProc>`
}

func TestDACTEContextFields(t *testing.T) {
	root, err := parseXML([]byte(sampleCTeXML()))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildCTeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	carga := data["carga"].(map[string]any)
	if carga["kg"] != "2500.000" || carga["vol"] != "10" {
		t.Fatalf("carga = %v", carga)
	}
	if data["vTPrest"] != "1.500,50" || data["imp"].(map[string]any)["valor"] != "180,06" {
		t.Fatalf("values = %v %v", data["vTPrest"], data["imp"])
	}
	if toma := data["toma"].(map[string]any); toma["nome"] != "REMETENTE TESTE" {
		t.Fatalf("toma3=0 must be the remetente, got %v", toma)
	}
	if data["chave_fmt"] != keyBlocks(testCTeKey) || data["rntrc"] != "12345678" {
		t.Fatalf("key/rntrc = %v %v", data["chave_fmt"], data["rntrc"])
	}
	if p := data["protocolo"].(map[string]any); p["nProt"] != "135260000000009" {
		t.Fatalf("protocol = %v", p)
	}
	if !strings.HasPrefix(data["qr_uri"].(string), dataURIPNGPrefix) {
		t.Fatal("QR Code missing")
	}
}

func TestDACTERendersPDFNormalAndOfflinePending(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeCTe, []byte(sampleCTeXML()), StateActive, "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("not a PDF")
	}
	parsed, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := parsed.Page(0); page.MediaBox.Width() < 590 || page.MediaBox.Width() > 600 {
		t.Fatalf("not A4 portrait: %.2f", page.MediaBox.Width())
	}
	// Offline pending: bare signed <CTe>, no protocol, contingency legend.
	x := strings.Replace(sampleCTeXML(), "<tpEmis>1</tpEmis>", "<tpEmis>2</tpEmis>", 1)
	x = x[strings.Index(x, "<CTe>"):strings.Index(x, "<protCTe>")]
	root, _ := parseXML([]byte(x))
	data, err := buildCTeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if data["is_contingencia"] != true || len(data["protocolo"].(map[string]any)) != 0 {
		t.Fatalf("offline context = %v %v", data["is_contingencia"], data["protocolo"])
	}
	if _, err := renderer.Render(context.Background(), DocTypeCTe, []byte(x), StateContingencyPending, ""); err != nil {
		t.Fatal(err)
	}
}

func TestDACTERejectsWrongModel(t *testing.T) {
	root, _ := parseXML([]byte(strings.Replace(sampleCTeXML(), "<mod>57</mod>", "<mod>67</mod>", 1)))
	if _, err := buildCTeContext(root, false); err == nil {
		t.Fatal("expected model mismatch error")
	}
}
