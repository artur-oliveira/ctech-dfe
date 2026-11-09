package documents

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

const testCTeKey = "35260812345678000190570010000000011000000019"

func sampleCTeXML() string {
	return `<cteProc xmlns="http://www.portalfiscal.inf.br/cte"><CTe><infCte Id="CTe` + testCTeKey + `"><ide><mod>57</mod><serie>1</serie><nCT>1</nCT><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><tpEmis>1</tpEmis><tpAmb>2</tpAmb><tpCTe>0</tpCTe><tpServ>0</tpServ><modal>01</modal><CFOP>6353</CFOP><natOp>PRESTACAO DE SERVICO DE TRANSPORTE</natOp><xMunIni>Sao Paulo</xMunIni><UFIni>SP</UFIni><xMunFim>Rio de Janeiro</xMunFim><UFFim>RJ</UFFim><toma3><toma>0</toma></toma3></ide><compl><xObs>Observacao teste</xObs></compl><emit><CNPJ>12345678000190</CNPJ><IE>123</IE><xNome>TRANSPORTADORA TESTE</xNome><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF></enderEmit></emit><rem><CNPJ>98765432000110</CNPJ><IE>456</IE><xNome>REMETENTE TESTE</xNome><enderReme><xLgr>Rua B</xLgr><nro>2</nro><xBairro>Centro</xBairro><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF><xPais>Brasil</xPais></enderReme></rem><dest><CPF>11122233344</CPF><xNome>DESTINATARIO TESTE</xNome><enderDest><xLgr>Rua C</xLgr><nro>3</nro><xBairro>Centro</xBairro><xMun>Rio de Janeiro</xMun><CEP>20000000</CEP><UF>RJ</UF></enderDest></dest><vPrest><vTPrest>1500.50</vTPrest><vRec>1500.50</vRec><Comp><xNome>FRETE VALOR</xNome><vComp>1500.50</vComp></Comp></vPrest><imp><ICMS><ICMS00><CST>00</CST><vBC>1500.50</vBC><pICMS>12.00</pICMS><vICMS>180.06</vICMS></ICMS00></ICMS></imp><infCTeNorm><infCarga><vCarga>10000.00</vCarga><proPred>MERCADORIA TESTE</proPred><infQ><cUnid>02</cUnid><tpMed>PESO BRUTO</tpMed><qCarga>2.5000</qCarga></infQ><infQ><cUnid>03</cUnid><tpMed>VOLUMES</tpMed><qCarga>10.0000</qCarga></infQ></infCarga><infDoc><infNFe><chave>` + testNFeKey + `</chave></infNFe></infDoc><infModal versaoModal="4.00"><rodo><RNTRC>12345678</RNTRC></rodo></infModal></infCTeNorm></infCte><infCTeSupl><qrCodCTe>https://example.invalid/cte?chCTe=` + testCTeKey + `</qrCodCTe></infCTeSupl></CTe><protCTe><infProt><chCTe>` + testCTeKey + `</chCTe><nProt>135260000000009</nProt><dhRecbto>2026-08-29T12:00:01-03:00</dhRecbto></infProt></protCTe></cteProc>`
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

// Anexo II secao 3: Peso Bruto, Peso Base de Calculo e Peso Aferido sao caixas distintas, uma por tpMed; antes
// o DACTE somava tudo em "Peso bruto" (CT-e de terceiros com varios infQ imprimia 2200 kg).
func TestDACTEWeightsSplitByTpMed(t *testing.T) {
	x := strings.Replace(sampleCTeXML(), `<infQ><cUnid>03</cUnid>`, `<infQ><cUnid>01</cUnid><tpMed>PESO BASE DE CÁLCULO</tpMed><qCarga>1200.0000</qCarga></infQ><infQ><cUnid>01</cUnid><tpMed>PESO AFERIDO</tpMed><qCarga>900.0000</qCarga></infQ><infQ><cUnid>01</cUnid><tpMed>PESO DECLARADO</tpMed><qCarga>77.0000</qCarga></infQ><infQ><cUnid>03</cUnid>`, 1)
	root, err := parseXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildCTeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	c := data["carga"].(map[string]any)
	if c["kg"] != "2500.000" || c["kg_base"] != "1200.000" || c["kg_afer"] != "900.000" {
		t.Fatalf("carga = %v", c)
	}
}

// Offline (tpEmis 2) already authorized: the protocol is printed and the contingency legend is dropped.
func TestDACTEOfflineAuthorizedShowsProtocol(t *testing.T) {
	x := strings.Replace(sampleCTeXML(), "<tpEmis>1</tpEmis>", "<tpEmis>2</tpEmis>", 1)
	root, err := parseXML([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildCTeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := data["protocolo"].(map[string]any); p["nProt"] != "135260000000009" || data["is_contingencia"] != false {
		t.Fatalf("protocol = %v contingencia = %v", data["protocolo"], data["is_contingencia"])
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

const testCTeOSKey = "35260812345678000190670010000000011000000012"

func sampleCTeOSXML() string {
	return `<cteOSProc xmlns="http://www.portalfiscal.inf.br/cte"><CTeOS><infCte Id="CTe` + testCTeOSKey + `"><ide><mod>67</mod><serie>1</serie><nCT>1</nCT><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><tpEmis>1</tpEmis><tpAmb>2</tpAmb><tpCTe>0</tpCTe><tpServ>6</tpServ><modal>01</modal><CFOP>6353</CFOP><natOp>PRESTACAO DE SERVICO DE TRANSPORTE</natOp><xMunIni>Sao Paulo</xMunIni><UFIni>SP</UFIni><xMunFim>Rio de Janeiro</xMunFim><UFFim>RJ</UFFim><infPercurso><UFPer>MG</UFPer></infPercurso></ide><compl><xObs>Observacao OS</xObs><ObsCont xCampo="a"><xTexto>uso emissor</xTexto></ObsCont></compl><emit><CNPJ>12345678000190</CNPJ><IE>123</IE><xNome>TRANSPORTADORA TESTE</xNome><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF></enderEmit></emit><toma><CNPJ>98765432000110</CNPJ><IE>456</IE><xNome>TOMADOR TESTE</xNome><enderToma><xLgr>Rua B</xLgr><nro>2</nro><xBairro>Centro</xBairro><xMun>Sao Paulo</xMun><CEP>01001000</CEP><UF>SP</UF></enderToma></toma><vPrest><vTPrest>300.00</vTPrest><vRec>300.00</vRec><Comp><xNome>PASSAGEM</xNome><vComp>300.00</vComp></Comp></vPrest><imp><ICMS><ICMS00><CST>00</CST><vBC>300.00</vBC><pICMS>12.00</pICMS><vICMS>36.00</vICMS></ICMS00></ICMS></imp><infCTeNorm><infServico><xDescServ>Transporte de passageiros</xDescServ><infQ><qCarga>12</qCarga></infQ></infServico><infModal versaoModal="4.00"><rodoOS><TAF>123456789012</TAF><veic><placa>ABC1D23</placa><RENAVAM>12345678901</RENAVAM></veic></rodoOS></infModal></infCTeNorm></infCte><infCTeSupl><qrCodCTe>https://example.invalid/cte?chCTe=` + testCTeOSKey + `</qrCodCTe></infCTeSupl></CTeOS><protCTe><infProt><chCTe>` + testCTeOSKey + `</chCTe><nProt>135260000000010</nProt><dhRecbto>2026-08-29T12:00:01-03:00</dhRecbto></infProt></protCTe></cteOSProc>`
}

func TestDACTEOSContextAndPDF(t *testing.T) {
	root, err := parseXML([]byte(sampleCTeOSXML()))
	if err != nil {
		t.Fatal(err)
	}
	data, err := buildCTeOSContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if toma := data["toma"].(map[string]any); toma["nome"] != "TOMADOR TESTE" {
		t.Fatalf("toma = %v", toma)
	}
	if s := data["servico"].(map[string]any); s["qtd"] != "12" || s["desc"] != "Transporte de passageiros" {
		t.Fatalf("servico = %v", s)
	}
	if rd := data["rodo"].(map[string]any); rd["placa"] != "ABC1D23" || rd["taf"] != "123456789012" {
		t.Fatalf("rodo = %v", rd)
	}
	if data["ide"].(map[string]any)["percurso"] != "MG" || data["ide"].(map[string]any)["tpServ"] != "Transporte de Pessoas" || data["uso_emissor"] != "uso emissor" {
		t.Fatalf("ide = %v", data["ide"])
	}
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	// The model 57 DACTE builder keeps refusing model 67; the renderer routes it to the OS layout.
	pdf, err := renderer.Render(context.Background(), DocTypeCTe, []byte(sampleCTeOSXML()), StateActive, "")
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("render OS: %v", err)
	}
}

// dumpIfAsked writes the PDF to $DUMP_DIR for the visual check (unset: no-op).
func dumpIfAsked(t *testing.T, name string, pdf []byte) {
	t.Helper()
	if dir := os.Getenv("DUMP_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, name+".pdf"), pdf, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

const sampleRodoModal = "<rodo><RNTRC>12345678</RNTRC></rodo>"

// cteWithModal swaps the rodoviario sample for another modal (code of ide/modal + the infModal child).
func cteWithModal(code, infModal string) string {
	x := strings.Replace(sampleCTeXML(), "<modal>01</modal>", "<modal>"+code+"</modal>", 1)
	return strings.Replace(x, sampleRodoModal, infModal, 1)
}

func TestDACTEModalBoxes(t *testing.T) {
	cases := []struct {
		name, code, modal, title string
		wantFields               map[string]string
		wantRows                 int
	}{
		{"aereo", "02", `<aereo><nMinu>123456789</nMinu><nOCA>12345678901</nOCA><dPrevAereo>2026-09-01</dPrevAereo><natCarga><xDime>10x20x30</xDime><cInfManu>01</cInfManu><cInfManu>02</cInfManu></natCarga><tarifa><CL>M</CL><cTar>T1</cTar><vTar>12.50</vTar></tarifa></aereo>`,
			"Informações específicas do modal aéreo", map[string]string{
				"Número operacional do conhecimento aéreo": "12345678901", "Classe": "M", "Valor da tarifa": "12,50", "Número da minuta": "123456789",
				"Data prevista de entrega": "01/09/2026", "Natureza da carga: dimensão": "10x20x30", "Natureza da carga: informações do manuseio": "01, 02",
			}, 0},
		{"aquaviario", "03", `<aquav><vPrest>1.00</vPrest><vAFRMM>150.00</vAFRMM><xNavio>NAVIO A</xNavio><balsa><xBalsa>B1</xBalsa></balsa><balsa><xBalsa>B2</xBalsa></balsa><detCont><nCont>CONT1234567</nCont><lacre><nLacre>L1</nLacre></lacre><lacre><nLacre>L2</nLacre></lacre></detCont></aquav>`,
			"Informações específicas do modal aquaviário", map[string]string{
				"Identificação do navio/rebocador": "NAVIO A", "Valor do AFRMM": "150,00", "Identificação da balsa": "B1, B2",
			}, 1},
		{"ferroviario", "04", `<ferrov><tpTraf>1</tpTraf><trafMut><respFat>1</respFat><ferrEmi>2</ferrEmi><vFrete>99.90</vFrete><ferroEnv><CNPJ>11222333000181</CNPJ><cInt>FE1</cInt><IE>9</IE><xNome>FERROVIA ENVOLVIDA</xNome></ferroEnv></trafMut><fluxo>FLX1</fluxo></ferrov>`,
			"Informações específicas do modal ferroviário", map[string]string{
				"Tipo de tráfego": "Mútuo", "Fluxo ferroviário": "FLX1", "Ferrovia responsável pelo faturamento": "Ferrovia de origem",
				"Ferrovia emitente do CT-e": "Ferrovia de destino", "Valor do frete": "99,90",
			}, 1},
		{"dutoviario", "05", `<duto><vTar>1.00</vTar></duto>`, "Informações específicas do modal dutoviário", map[string]string{}, 0},
		{"multimodal", "06", `<multimodal><COTM>COTM123</COTM><indNegociavel>1</indNegociavel><seg><infSeg><xSeg>SEGURADORA</xSeg><CNPJ>11222333000181</CNPJ></infSeg><nApol>AP1</nApol><nAver>AV1</nAver></seg></multimodal>`,
			"Informações específicas do transporte multimodal de cargas", map[string]string{
				"Número do certificado do operador de transporte multimodal": "COTM123", "Indicador negociável": "Negociável",
				"Nome da seguradora": "SEGURADORA", "Número da apólice": "AP1", "Número da averbação": "AV1",
			}, 0},
	}
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			xml := cteWithModal(tc.code, tc.modal)
			root, err := parseXML([]byte(xml))
			if err != nil {
				t.Fatal(err)
			}
			data, err := buildCTeContext(root, false)
			if err != nil {
				t.Fatal(err)
			}
			info := data["modal_info"].(map[string]any)
			if info["title"] != tc.title {
				t.Fatalf("title = %v", info["title"])
			}
			got := map[string]string{}
			for _, f := range info["fields"].([]cteBox) {
				got[f["lbl"].(string)] = f["val"].(string)
			}
			for k, v := range tc.wantFields {
				if got[k] != v {
					t.Errorf("%s = %q, want %q", k, got[k], v)
				}
			}
			rows := 0
			for _, tb := range info["tables"].([]cteBox) {
				rows += len(tb["rows"].([][]string))
			}
			if rows != tc.wantRows {
				t.Errorf("table rows = %d, want %d", rows, tc.wantRows)
			}
			pdf, err := renderer.Render(context.Background(), DocTypeCTe, []byte(xml), StateActive, "")
			if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
				t.Fatalf("render: %v", err)
			}
			dumpIfAsked(t, "dacte_"+tc.name, pdf)
		})
	}
}

func TestDACTERodoviarioHasNoModalBox(t *testing.T) {
	root, _ := parseXML([]byte(sampleCTeXML()))
	data, _ := buildCTeContext(root, false)
	if data["modal_info"].(map[string]any)["title"] != "" {
		t.Fatal("rodoviario prints only the RNTRC line")
	}
}

func TestDACTEFlow(t *testing.T) {
	x := strings.Replace(sampleCTeXML(), "<xObs>", "<fluxo><xOrig>ORI</xOrig><pass><xPass>P1</xPass></pass><pass><xPass>P2</xPass></pass><xDest>DES</xDest></fluxo><xObs>", 1)
	root, _ := parseXML([]byte(x))
	data, _ := buildCTeContext(root, false)
	f := data["modal_info"].(map[string]any)["fluxo"].(map[string]any)
	if f["orig"] != "ORI" || f["pass"] != "P1, P2" || f["dest"] != "DES" || f["has"] != true {
		t.Fatalf("fluxo = %v", f)
	}
}

func TestDACTEA5(t *testing.T) {
	if ValidateFormat(DocTypeCTe, FormatA5) != nil || ValidateFormat(DocTypeCTe, FormatPaisagem) == nil {
		t.Fatal("cte formats: retrato and a5 only")
	}
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeCTe, []byte(sampleCTeXML()), StateActive, FormatA5)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	// A5 landscape: 210 x 148 mm = 595.3 x 419.5 pt.
	if page, _ := parsed.Page(0); page.MediaBox.Width() < 590 || page.MediaBox.Width() > 600 || page.MediaBox.Height() < 415 || page.MediaBox.Height() > 424 {
		t.Fatalf("not A5 landscape: %.1f x %.1f", page.MediaBox.Width(), page.MediaBox.Height())
	}
	dumpIfAsked(t, "dacte_a5", pdf)
	if _, err := renderer.Render(context.Background(), DocTypeCTe, []byte(sampleCTeOSXML()), StateActive, FormatA5); err == nil {
		t.Fatal("DACTE OS must refuse A5")
	} else if _, ok := err.(*FormatNotAllowedError); !ok {
		t.Fatalf("err = %v", err)
	}
	if a, b := cacheKey("O", DocTypeCTe, testCTeKey, StateActive, ""), cacheKey("O", DocTypeCTe, testCTeKey, StateActive, FormatA5); a == b || !strings.Contains(a, "/v2/") {
		t.Fatalf("cache keys %q %q", a, b)
	}
}
