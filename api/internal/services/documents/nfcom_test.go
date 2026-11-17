package documents

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

const testNFComKey = "35261112345678000190620010000000101012345671"

func sampleNFComXML(tpEmis string) string {
	return `<nfcomProc xmlns="http://www.portalfiscal.inf.br/nfcom" versao="1.00"><NFCom><infNFCom versao="1.00" Id="NFCom` + testNFComKey + `">` +
		`<ide><cUF>35</cUF><tpAmb>1</tpAmb><mod>62</mod><serie>1</serie><nNF>10</nNF><dhEmi>2026-11-05T10:00:00-03:00</dhEmi><tpEmis>` + tpEmis + `</tpEmis></ide>` +
		`<emit><CNPJ>12345678000190</CNPJ><IE>110042490114</IE><xNome>PROVEDOR TESTE LTDA</xNome><enderEmit><xLgr>Rua A</xLgr><nro>10</nro><xBairro>Centro</xBairro><cMun>3550308</cMun><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF></enderEmit></emit>` +
		`<dest><xNome>CLIENTE EXEMPLO</xNome><CNPJ>33000167000101</CNPJ><indIEDest>9</indIEDest><enderDest><xLgr>Rua B</xLgr><nro>20</nro><xBairro>Bairro</xBairro><cMun>3550308</cMun><xMun>Sao Paulo</xMun><CEP>01002000</CEP><UF>SP</UF></enderDest></dest>` +
		`<assinante><iCodAssinante>ASS-001</iCodAssinante><tpAssinante>1</tpAssinante><tpServUtil>6</tpServUtil><NroTermPrinc>1130000000</NroTermPrinc></assinante>` +
		`<det nItem="1"><prod><cProd>PLANO100</cProd><xProd>Internet 100 Mbps</xProd><cClass>0100301</cClass><CFOP>5307</CFOP><uMed>4</uMed><qFaturada>1.0000</qFaturada><vItem>100.00</vItem><vDesc>10.00</vDesc><vOutro>2.50</vOutro><vProd>92.50</vProd></prod>` +
		`<imposto><ICMS00><CST>00</CST><vBC>92.50</vBC><pICMS>18</pICMS><vICMS>16.65</vICMS></ICMS00><PIS><CST>01</CST><vBC>92.50</vBC><pPIS>0.65</pPIS><vPIS>0.60</vPIS></PIS><COFINS><CST>01</CST><vBC>92.50</vBC><pCOFINS>3</pCOFINS><vCOFINS>2.78</vCOFINS></COFINS></imposto></det>` +
		`<total><vProd>92.50</vProd><ICMSTot><vBC>92.50</vBC><vICMS>16.65</vICMS><vICMSDeson>0.00</vICMSDeson><vFCP>0.00</vFCP></ICMSTot><vCOFINS>2.78</vCOFINS><vPIS>0.60</vPIS><vFUNTTEL>0.00</vFUNTTEL><vFUST>0.00</vFUST><vDesc>10.00</vDesc><vOutro>2.50</vOutro><vNF>92.50</vNF></total>` +
		`<gFat><CompetFat>202611</CompetFat><dVencFat>2026-12-10</dVencFat><codBarras>12345678901234567890</codBarras></gFat>` +
		`<infAdic><infAdFisco>Mensagem fiscal de teste</infAdFisco><infCpl>Obrigado</infCpl></infAdic></infNFCom>` +
		`<infNFComSupl><qrCodNFCom>https://dfe-portal.svrs.rs.gov.br/nfcom/qrCode?chNFCom=` + testNFComKey + `&amp;tpAmb=1</qrCodNFCom></infNFComSupl></NFCom>` +
		`<protNFCom versao="1.00"><infProt><chNFCom>` + testNFComKey + `</chNFCom><nProt>1352600000000011</nProt><dhRecbto>2026-11-05T10:00:05-03:00</dhRecbto></infProt></protNFCom></nfcomProc>`
}

func TestDANFEComContextFields(t *testing.T) {
	root, err := parseXML([]byte(sampleNFComXML("1")))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildNFComContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := data["emit"].(map[string]any)["doc"]; got != "12.345.678/0001-90" {
		t.Errorf("CNPJ do emitente = %v", got)
	}
	ide := data["ide"].(map[string]any)
	if ide["serie"] != "1" || ide["nNF"] != "000.000.010" || ide["dhEmi"] != "05/11/2026" {
		t.Errorf("ide = %v", ide)
	}
	cli := data["cliente"].(map[string]any)
	if cli["codigo"] != "ASS-001" || cli["vencimento"] != "10/12/2026" || cli["total"] != "92,50" || cli["referencia"] != "11/2026" {
		t.Errorf("cliente = %v", cli)
	}
	items := data["itens"].([]map[string]any)
	if len(items) != 1 || items[0]["desc"] != "Internet 100 Mbps" || items[0]["valor"] != "92,50" || items[0]["pis_cofins"] != "3,38" || items[0]["icms"] != "16,65" || items[0]["aliq"] != "18" {
		t.Errorf("itens = %v", items)
	}
	if tot := data["totais"].(map[string]any); tot["vNF"] != "92,50" || tot["pis"] != "0,60" || tot["cofins"] != "2,78" {
		t.Errorf("totais = %v", tot)
	}
	if p := data["protocolo"].(map[string]any); p["nProt"] != "1352600000000011" {
		t.Errorf("protocolo = %v", p)
	}
	if data["chave_fmt"] != keyBlocks(testNFComKey) || data["is_contingencia"] != false || !strings.HasPrefix(data["qr_uri"].(string), dataURIPNGPrefix) {
		t.Errorf("chave/contingência/QR = %v %v", data["chave_fmt"], data["is_contingencia"])
	}
	if data["msg_fiscal"] != "Mensagem fiscal de teste" {
		t.Errorf("msg_fiscal = %v", data["msg_fiscal"])
	}
}

// MOC DANFE-COM 2.1.6: na contingência o protocolo é suprimido e entra "EMITIDO EM CONTINGÊNCIA Pendente de autorização".
func TestDANFEComContingencyHasNoProtocol(t *testing.T) {
	x := sampleNFComXML("2")
	x = x[strings.Index(x, "<NFCom>"):strings.Index(x, "<protNFCom")]
	root, err := parseXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildNFComContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := data["protocolo"].(map[string]any); data["is_contingencia"] != true || len(p) != 0 {
		t.Errorf("contingência = %v protocolo = %v", data["is_contingencia"], data["protocolo"])
	}
}

func TestDANFEComRequiresModel62(t *testing.T) {
	root, _ := parseXML([]byte(strings.Replace(sampleNFComXML("1"), "<mod>62</mod>", "<mod>57</mod>", 1)))
	if _, err := buildNFComContext(root, false); err == nil {
		t.Error("modelo 57 não é DANFE-COM")
	}
}

func TestDANFEComRendersPDF(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeNFCom, []byte(sampleNFComXML("1")), StateActive, "")
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
	// A nota cancelada e a em contingência também renderizam.
	if _, err := renderer.Render(context.Background(), DocTypeNFCom, []byte(sampleNFComXML("1")), StateCancelled, ""); err != nil {
		t.Fatalf("cancelada: %v", err)
	}
	x := sampleNFComXML("2")
	x = x[strings.Index(x, "<NFCom>"):strings.Index(x, "<protNFCom")]
	if _, err := renderer.Render(context.Background(), DocTypeNFCom, []byte(x), StateContingencyPending, ""); err != nil {
		t.Fatalf("contingência: %v", err)
	}
}
