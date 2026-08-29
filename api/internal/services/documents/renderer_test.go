package documents

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

const (
	testNFeKey  = "35260812345678000190550010000000011000000010"
	testNFCeKey = "35260812345678000190650010000000011000000015"
	testMDFeKey = "35260812345678000190580010000000011000000012"
)

func TestFolioRendererGeneratesAuxiliaryDocuments(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		docType  string
		xml      string
		minWidth float64
		maxWidth float64
	}{
		{"DANFE", DocTypeNFe, sampleNFeXML(), 590, 600},
		{"DANFCe", DocTypeNFCe, sampleNFCeXML(), 160, 170},
		{"DAMDFE", DocTypeMDFe, sampleMDFeXML(), 590, 600},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pdf, err := renderer.Render(context.Background(), test.docType, []byte(test.xml), false)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(pdf, []byte("%PDF")) {
				t.Fatalf("output does not start with PDF signature: %q", pdf[:min(16, len(pdf))])
			}
			parsed, err := reader.Parse(pdf)
			if err != nil {
				t.Fatal(err)
			}
			page, err := parsed.Page(0)
			if err != nil {
				t.Fatal(err)
			}
			if width := page.MediaBox.Width(); width < test.minWidth || width > test.maxWidth {
				t.Fatalf("page width = %.2f, want %.2f..%.2f", width, test.minWidth, test.maxWidth)
			}
		})
	}
}

func TestNFeFSContingencyIncludesDadosNFeBarcode(t *testing.T) {
	root, err := parseXML([]byte(strings.Replace(sampleNFeXML(), "<tpEmis>1</tpEmis>", "<tpEmis>2</tpEmis>", 1)))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildNFeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := data["dados_nfe_code"].(string)
	barcode, _ := data["dados_nfe_barcode"].(string)
	if len(code) != 36 {
		t.Fatalf("dados_nfe_code length = %d, want 36", len(code))
	}
	if !strings.HasPrefix(barcode, dataURIPNGPrefix) {
		t.Fatalf("dados_nfe_barcode = %q", barcode)
	}
}

func TestNFCeOfflineContingencyGeneratesTwoVias(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	xml := strings.Replace(sampleNFCeXML(), "<tpEmis>1</tpEmis>", "<tpEmis>9</tpEmis>", 1)
	pdf, err := renderer.Render(context.Background(), DocTypeNFCe, []byte(xml), false)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.PageCount() != 2 {
		t.Fatalf("offline contingency pages = %d, want 2", parsed.PageCount())
	}
}

func TestFolioRendererRejectsWrongModel(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderer.Render(context.Background(), DocTypeNFe, []byte(sampleNFCeXML()), false); err == nil {
		t.Fatal("expected model mismatch error")
	}
}

func sampleNFeXML() string {
	return `<nfeProc><NFe><infNFe Id="NFe` + testNFeKey + `"><ide><mod>55</mod><tpNF>1</tpNF><nNF>1</nNF><serie>1</serie><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><tpEmis>1</tpEmis><tpAmb>2</tpAmb><natOp>VENDA</natOp></ide><emit><CNPJ>12345678000190</CNPJ><xNome>EMITENTE TESTE</xNome><IE>123</IE><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro><xMun>São Paulo</xMun><UF>SP</UF><CEP>01001000</CEP></enderEmit></emit><dest><CPF>11122233344</CPF><xNome>DESTINATÁRIO TESTE</xNome><enderDest><xLgr>Rua B</xLgr><nro>2</nro><xBairro>Centro</xBairro><xMun>São Paulo</xMun><UF>SP</UF><CEP>01001000</CEP></enderDest></dest><det nItem="1"><prod><cProd>1</cProd><xProd>PRODUTO TESTE</xProd><NCM>00000000</NCM><CFOP>5102</CFOP><uCom>UN</uCom><qCom>1.0000</qCom><vUnCom>10.00</vUnCom><vProd>10.00</vProd></prod><imposto><ICMS><ICMS00><orig>0</orig><CST>00</CST><vBC>10.00</vBC><pICMS>18.00</pICMS><vICMS>1.80</vICMS></ICMS00></ICMS></imposto></det><total><ICMSTot><vBC>10.00</vBC><vICMS>1.80</vICMS><vProd>10.00</vProd><vNF>10.00</vNF></ICMSTot></total><transp><modFrete>9</modFrete></transp><infAdic><infCpl>Documento sintético</infCpl></infAdic></infNFe></NFe><protNFe><infProt><chNFe>` + testNFeKey + `</chNFe><nProt>135260000000001</nProt><dhRecbto>2026-08-29T12:00:01-03:00</dhRecbto></infProt></protNFe></nfeProc>`
}

func sampleNFCeXML() string {
	return `<nfeProc><NFe><infNFe Id="NFe` + testNFCeKey + `"><ide><mod>65</mod><nNF>1</nNF><serie>1</serie><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><tpEmis>1</tpEmis><tpAmb>2</tpAmb></ide><emit><CNPJ>12345678000190</CNPJ><xNome>LOJA TESTE</xNome><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro><xMun>São Paulo</xMun><UF>SP</UF></enderEmit></emit><det nItem="1"><prod><cProd>1</cProd><xProd>PRODUTO TESTE</xProd><qCom>1.0000</qCom><uCom>UN</uCom><vUnCom>10.00</vUnCom><vProd>10.00</vProd></prod></det><total><ICMSTot><vProd>10.00</vProd><vNF>10.00</vNF><vTotTrib>1.00</vTotTrib></ICMSTot></total><pag><detPag><tPag>01</tPag><vPag>10.00</vPag></detPag></pag><infNFeSupl><qrCode>https://example.invalid/qrcode?p=` + testNFCeKey + `</qrCode><urlChave>https://example.invalid</urlChave></infNFeSupl></infNFe></NFe><protNFe><infProt><chNFe>` + testNFCeKey + `</chNFe><nProt>135260000000002</nProt><dhRecbto>2026-08-29T12:00:01-03:00</dhRecbto></infProt></protNFe></nfeProc>`
}

func sampleMDFeXML() string {
	return `<mdfeProc><MDFe><infMDFe Id="MDFe` + testMDFeKey + `"><ide><mod>58</mod><serie>1</serie><nMDF>1</nMDF><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><tpEmis>1</tpEmis><tpAmb>2</tpAmb><modal>1</modal><tpEmit>2</tpEmit><tpTransp>1</tpTransp><UFIni>SP</UFIni><UFFim>RJ</UFFim><infMunCarrega><xMunCarrega>São Paulo</xMunCarrega></infMunCarrega></ide><emit><CNPJ>12345678000190</CNPJ><xNome>TRANSPORTADOR TESTE</xNome><IE>123</IE><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro><xMun>São Paulo</xMun><UF>SP</UF><CEP>01001000</CEP></enderEmit></emit><infModal><rodo><infANTT><RNTRC>12345678</RNTRC></infANTT><veicTracao><placa>ABC1D23</placa><UF>SP</UF><tara>1000</tara><condutor><xNome>MOTORISTA TESTE</xNome><CPF>11122233344</CPF></condutor></veicTracao></rodo></infModal><infDoc><infMunDescarga><xMunDescarga>Rio de Janeiro</xMunDescarga><infNFe><chNFe>` + testNFeKey + `</chNFe></infNFe></infMunDescarga></infDoc><prodPred><tpCarga>05</tpCarga><xProd>PRODUTO TESTE</xProd><NCM>00000000</NCM></prodPred><tot><qNFe>1</qNFe><vCarga>10.00</vCarga><qCarga>1.0000</qCarga><cUnid>01</cUnid></tot></infMDFe><infMDFeSupl><qrCodMDFe>https://example.invalid/mdfe?p=` + testMDFeKey + `</qrCodMDFe></infMDFeSupl></MDFe><protMDFe><infProt><chMDFe>` + testMDFeKey + `</chMDFe><nProt>935260000000001</nProt><dhRecbto>2026-08-29T12:00:01-03:00</dhRecbto></infProt></protMDFe></mdfeProc>`
}
