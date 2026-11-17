package documents

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

const testBPeKey = "35261112345678000190630010000000101012345671"

func sampleBPeXML(tpEmis, tpBPe string) string {
	return `<bpeProc xmlns="http://www.portalfiscal.inf.br/bpe" versao="1.00"><BPe><infBPe versao="1.00" Id="BPe` + testBPeKey + `">` +
		`<ide><cUF>35</cUF><tpAmb>1</tpAmb><mod>63</mod><serie>1</serie><nBP>10</nBP><dhEmi>2026-11-05T10:00:00-03:00</dhEmi><tpEmis>` + tpEmis + `</tpEmis><tpBPe>` + tpBPe + `</tpBPe><UFIni>SP</UFIni><UFFim>RJ</UFFim></ide>` +
		`<emit><CNPJ>12345678000190</CNPJ><IE>110042490114</IE><xNome>VIACAO TESTE LTDA</xNome><enderEmit><xLgr>Rua A</xLgr><nro>10</nro><xBairro>Centro</xBairro><cMun>3550308</cMun><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF></enderEmit></emit>` +
		`<infPassagem><cLocOrig>3550308</cLocOrig><xLocOrig>SAO PAULO</xLocOrig><cLocDest>3304557</cLocDest><xLocDest>RIO DE JANEIRO</xLocDest><dhEmb>2026-11-10T08:00:00-03:00</dhEmb><dhValidade>2026-11-20T08:00:00-03:00</dhValidade>` +
		`<infPassageiro><xNome>Maria Souza</xNome><tpDoc>1</tpDoc><nDoc>123456789</nDoc></infPassageiro></infPassagem>` +
		`<infViagem><cPercurso>L1</cPercurso><xPercurso>SAO PAULO X RIO</xPercurso><tpViagem>00</tpViagem><tpServ>6</tpServ><tpAcomodacao>1</tpAcomodacao><tpTrecho>1</tpTrecho><dhViagem>2026-11-10T08:00:00-03:00</dhViagem><prefixo>1234</prefixo><poltrona>12</poltrona></infViagem>` +
		`<infViagem><cPercurso>L2</cPercurso><xPercurso>RIO X NITEROI</xPercurso><tpViagem>00</tpViagem><tpServ>6</tpServ><tpAcomodacao>1</tpAcomodacao><tpTrecho>3</tpTrecho><dhViagem>2026-11-10T14:00:00-03:00</dhViagem><dhConexao>2026-11-10T13:00:00-03:00</dhConexao></infViagem>` +
		`<infValorBPe><vBP>100.00</vBP><vDesconto>10.00</vDesconto><vPgto>90.00</vPgto><vTroco>10.00</vTroco><tpDesconto>03</tpDesconto><xDesconto>Idoso</xDesconto><Comp><tpComp>01</tpComp><vComp>90.00</vComp></Comp><Comp><tpComp>03</tpComp><vComp>10.00</vComp></Comp></infValorBPe>` +
		`<pag><tPag>01</tPag><vPag>100.00</vPag></pag>` +
		`<infAdic><infAdFisco>Mensagem fiscal de teste</infAdFisco><infCpl>Boa viagem</infCpl></infAdic></infBPe>` +
		`<infBPeSupl><qrCodBPe>https://dfe-portal.svrs.rs.gov.br/bpe/qrCode?chBPe=` + testBPeKey + `&amp;tpAmb=1</qrCodBPe></infBPeSupl></BPe>` +
		`<protBPe versao="1.00"><infProt><chBPe>` + testBPeKey + `</chBPe><nProt>135260000000001</nProt><dhRecbto>2026-11-05T10:00:05-03:00</dhRecbto></infProt></protBPe></bpeProc>`
}

func TestDABPEContextFields(t *testing.T) {
	root, err := parseXML([]byte(sampleBPeXML("1", "0")))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildBPeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := data["emit"].(map[string]any)["doc"]; got != "12.345.678/0001-90" {
		t.Errorf("CNPJ do emitente = %v", got)
	}
	if ide := data["ide"].(map[string]any); ide["nBP"] != "000.000.010" || ide["serie"] != "1" {
		t.Errorf("ide = %v", ide)
	}
	trips := data["viagens"].([]map[string]any)
	if len(trips) != 2 || trips[0]["conexao"] != false || trips[1]["conexao"] != true || trips[0]["poltrona"] != "12" || trips[0]["tipo"] != "Executivo" {
		t.Errorf("viagens = %v", trips)
	}
	v := data["valores"].(map[string]any)
	if v["total"] != "100,00" || v["a_pagar"] != "90,00" || v["pago"] != "100,00" || v["troco"] != "10,00" || v["tem_desconto"] != true {
		t.Errorf("valores = %v", v)
	}
	px := data["passageiro"].(map[string]any)
	if px["linha"] != "PASSAGEIRO: RG: 123456789 MARIA SOUZA" || px["desconto"] != "TIPO DE DESCONTO: IDOSO" {
		t.Errorf("passageiro = %v", px)
	}
	if p := data["protocolo"].(map[string]any); p["nProt"] != "135260000000001" || data["is_contingencia"] != false {
		t.Errorf("protocolo = %v", data["protocolo"])
	}
	if !strings.HasPrefix(data["qr_uri"].(string), dataURIPNGPrefix) {
		t.Error("QR Code ausente")
	}
}

func TestDABPEContingencyHasNoProtocol(t *testing.T) {
	x := sampleBPeXML("2", "0")
	x = x[strings.Index(x, "<BPe>"):strings.Index(x, "<protBPe")]
	root, err := parseXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildBPeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := data["protocolo"].(map[string]any); data["is_contingencia"] != true || len(p) != 0 {
		t.Errorf("contingência = %v protocolo = %v", data["is_contingencia"], data["protocolo"])
	}
}

func TestDABPERejectsTMAndWrongModel(t *testing.T) {
	root, _ := parseXML([]byte(sampleBPeXML("1", "4")))
	if _, err := buildBPeContext(root, false); err == nil {
		t.Error("TM não gera DABPE")
	}
	root, _ = parseXML([]byte(strings.Replace(sampleBPeXML("1", "0"), "<mod>63</mod>", "<mod>57</mod>", 1)))
	if _, err := buildBPeContext(root, false); err == nil {
		t.Error("modelo 57 não é DABPE")
	}
}

func TestDABPERendersPDF(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeBPe, []byte(sampleBPeXML("1", "0")), StateActive, "")
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
	if _, err := renderer.Render(context.Background(), DocTypeBPe, []byte(sampleBPeXML("1", "0")), StateCancelled, ""); err != nil {
		t.Fatalf("cancelado: %v", err)
	}
	x := sampleBPeXML("2", "0")
	x = x[strings.Index(x, "<BPe>"):strings.Index(x, "<protBPe")]
	pdfCont, err := renderer.Render(context.Background(), DocTypeBPe, []byte(x), StateContingencyPending, "")
	if err != nil {
		t.Fatalf("contingência: %v", err)
	}
	// MOC DABPE 2.2.10: contingência imprime também a "Via do Estabelecimento" (2 páginas).
	if contParsed, err := reader.Parse(pdfCont); err != nil || contParsed.PageCount() != 2 {
		t.Fatalf("contingência deveria ter 2 vias: %v", err)
	}
}

func TestDABPEReducedFormat(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeBPe, []byte(sampleBPeXML("1", "0")), StateActive, FormatReduzido)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	// 58 mm = 164,4 pt; o MOC pede no mínimo 56 mm.
	if page, _ := parsed.Page(0); page.MediaBox.Width() < 158 || page.MediaBox.Width() > 170 {
		t.Fatalf("largura da bobina = %.2f pt", page.MediaBox.Width())
	}
	if p := ValidateFormat(DocTypeBPe, "a3"); p == nil {
		t.Error("formato desconhecido deveria ser recusado")
	}
}

func TestDABPEPaymentLabelPIX(t *testing.T) {
	root, _ := parseXML([]byte(strings.Replace(sampleBPeXML("1", "0"), "<tPag>01</tPag>", "<tPag>06</tPag>", 1)))
	data, err := buildBPeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := data["valores"].(map[string]any)["pagamentos"].([]map[string]any)[0]["forma"]; got != "PIX" {
		t.Errorf("forma = %v", got)
	}
}
