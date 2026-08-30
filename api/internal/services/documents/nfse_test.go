package documents

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"

	"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"
)

// testNFSeKey tem os 50 dígitos da chave da NFS-e nacional; o conteúdo não
// precisa ser uma chave real, só respeitar o comprimento validado por doc type.
const testNFSeKey = "22110010212345678000181000000000000012026082900001"

func TestBuildNFSeContextReadsAuthorizedXML(t *testing.T) {
	root, err := parseXML([]byte(sampleNFSeXML()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := buildNFSeContext(root, StateActive)
	if err != nil {
		t.Fatal(err)
	}

	ident := ctx["ident"].(map[string]any)
	if ident["chave"] != testNFSeKey {
		t.Errorf("chave = %v", ident["chave"])
	}
	if ident["numero"] != "1" || ident["competencia"] != "01/08/2026" {
		t.Errorf("identificação = %+v", ident)
	}
	if ctx["watermark"] != "" {
		t.Errorf("documento ativo não pode ter watermark, veio %q", ctx["watermark"])
	}
	if ctx["is_homologacao"] != true {
		t.Error("tpAmb=2 deveria marcar homologação")
	}
	if url, _ := ctx["url_consulta"].(string); !strings.HasSuffix(url, testNFSeKey) ||
		!strings.HasPrefix(url, nfseConsultaURL) {
		t.Errorf("url de consulta = %q", url)
	}
	if qr, _ := ctx["qr_uri"].(string); !strings.HasPrefix(qr, "data:image/png;base64,") {
		t.Errorf("QR não embutido: %q", qr)
	}

	// Rótulos vêm do catálogo gerado, nunca redigitados no builder.
	if got := ident["emitente_tipo"]; got != "1 - Prestador" {
		t.Errorf("emitente_tipo = %v", got)
	}
	tribMun := ctx["trib_mun"].(map[string]any)
	if got, _ := tribMun["tipo_tributacao"].(string); !strings.HasPrefix(got, "1 - ") {
		t.Errorf("tributação ISSQN sem rótulo do catálogo: %q", got)
	}

	totais := ctx["totais"].(map[string]any)
	if totais["valor_operacao"] != "1.000,00" || totais["valor_liquido"] != "980,00" {
		t.Errorf("totais = %+v", totais)
	}
	// Percentuais também saem em pt-BR, sem arredondar as casas do XML.
	if totais["perc_federal"] != "3,65" || tribMun["aliquota_aplicada"] != "2,00" {
		t.Errorf("percentuais não formatados: %v / %v", totais["perc_federal"], tribMun["aliquota_aplicada"])
	}
	// Partes ausentes viram nil para o template suprimir o quadro inteiro.
	if ctx["intermediario"] != nil {
		t.Error("intermediário ausente deveria ser nil")
	}
	if ctx["tomador"] == nil {
		t.Error("tomador presente no XML não pode sumir")
	}
	// O prestador funde emitente (nome/endereço resolvidos pelo fisco) com o
	// regime tributário declarado na DPS.
	prestador := ctx["prestador"].(map[string]any)
	if prestador["nome"] != "PRESTADOR TESTE LTDA" || prestador["simples_nacional"] == "" {
		t.Errorf("prestador = %+v", prestador)
	}
	// Total do IBS/CBS não existe pronto no XML: é IBS + CBS apurados.
	if totais["total_ibscbs"] != "10,50" || totais["liquido_ibscbs"] != "990,50" {
		t.Errorf("totais IBS/CBS = %v / %v", totais["total_ibscbs"], totais["liquido_ibscbs"])
	}
	// O canhoto do pé da folha repete número e chave.
	if ctx["canhoto"].(map[string]any)["numero"] != "1" {
		t.Errorf("canhoto = %+v", ctx["canhoto"])
	}
	// A classificação IBS/CBS vem da DPS; os valores apurados, do bloco IBSCBS
	// da NFS-e. O DANFSe imprime os dois lado a lado.
	ibscbs := ctx["ibscbs"].(map[string]any)
	if ibscbs["cst_class_trib"] != "000 / 000001" || ibscbs["indicador_operacao"] != "020101" {
		t.Errorf("classificação IBS/CBS = %+v", ibscbs)
	}
	if ctx["destinatario"] == nil {
		t.Error("destinatário presente no grupo IBSCBS da DPS não pode sumir")
	}
}

func TestBuildNFSeContextWatermarkByState(t *testing.T) {
	root, err := parseXML([]byte(sampleNFSeXML()))
	if err != nil {
		t.Fatal(err)
	}
	for state, want := range map[DocumentState]string{
		StateActive:      "",
		StateCancelled:   nfseWatermarkCancelled,
		StateSubstituted: nfseWatermarkSubstituted,
	} {
		ctx, err := buildNFSeContext(root, state)
		if err != nil {
			t.Fatal(err)
		}
		if ctx["watermark"] != want {
			t.Errorf("estado %s: watermark = %v, esperado %q", state, ctx["watermark"], want)
		}
	}
}

func TestBuildNFSeContextRejectsWrongKeyLength(t *testing.T) {
	short := strings.Replace(sampleNFSeXML(), testNFSeKey, testNFSeKey[:44], 1)
	root, err := parseXML([]byte(short))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildNFSeContext(root, StateActive); err == nil {
		t.Fatal("chave de 44 dígitos aceita na NFS-e")
	}
}

func TestFolioRendererGeneratesDANFSeSinglePage(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeNFSe, []byte(sampleNFSeXML()), StateCancelled)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("saída não é PDF: %q", pdf[:min(16, len(pdf))])
	}
	parsed, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if pages := parsed.PageCount(); pages != 1 {
		t.Errorf("DANFSe = %d páginas, a NT exige 1", pages)
	}
	page, err := parsed.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	// A4 retrato: 595 x 842 pt.
	if width := page.MediaBox.Width(); width < 590 || width > 600 {
		t.Errorf("largura = %.2f, esperado A4 retrato", width)
	}
}

func TestFolioRendererDANFSeMinimalDocument(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := renderer.Render(context.Background(), DocTypeNFSe, []byte(minimalNFSeXML()), StateActive)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("documento mínimo não renderizou")
	}
}

// sampleNFSeXML é uma NFS-e nacional autorizada com todos os blocos que a NT
// 008 v1.02 manda imprimir: partes, comércio exterior, obra, ISSQN, federal,
// IBS/CBS, totais e complementares.
func sampleNFSeXML() string {
	return `<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.01">
<infNFSe Id="NFS` + testNFSeKey + `">
<xLocEmi>Teresina</xLocEmi><xLocPrestacao>Teresina</xLocPrestacao>
<nNFSe>1</nNFSe><cLocIncid>2211001</cLocIncid><xLocIncid>Teresina</xLocIncid>
<xTribNac>Análise e desenvolvimento de sistemas</xTribNac>
<xTribMun>Serviços de informática</xTribMun><xNBS>Serviços de TI</xNBS>
<verAplic>SNNFSE-1.0</verAplic><ambGer>1</ambGer><tpEmis>1</tpEmis><procEmi>1</procEmi>
<cStat>100</cStat><dhProc>2026-08-29T12:00:05-03:00</dhProc><nDFSe>10</nDFSe>
<emit><CNPJ>11222333000181</CNPJ><IM>123456</IM><xNome>PRESTADOR TESTE LTDA</xNome>
<xFant>PRESTADOR</xFant>
<enderNac><xLgr>Rua A</xLgr><nro>100</nro><xBairro>Centro</xBairro>
<cMun>2211001</cMun><UF>PI</UF><CEP>64000000</CEP></enderNac>
<fone>8632000000</fone><email>fiscal@example.invalid</email></emit>
<valores><vCalcDR>10.00</vCalcDR><vBC>990.00</vBC><pAliqAplic>2.00</pAliqAplic>
<vISSQN>19.80</vISSQN><vTotalRet>20.00</vTotalRet><vLiq>980.00</vLiq></valores>
<xOutInf>Documento sintético para teste</xOutInf>
<IBSCBS><cLocalidadeIncid>2211001</cLocalidadeIncid><xLocalidadeIncid>Teresina</xLocalidadeIncid>
<valores><vBC>1000.00</vBC>
<uf><pIBSUF>0.10</pIBSUF><pAliqEfetUF>0.10</pAliqEfetUF></uf>
<mun><pIBSMun>0.05</pIBSMun><pAliqEfetMun>0.05</pAliqEfetMun></mun>
<fed><pCBS>0.90</pCBS><pAliqEfetCBS>0.90</pAliqEfetCBS></fed></valores>
<totCIBS><vTotNF>1010.50</vTotNF>
<gIBS><vIBSTot>1.50</vIBSTot>
<gIBSUFTot><vIBSUF>1.00</vIBSUF></gIBSUFTot>
<gIBSMunTot><vIBSMun>0.50</vIBSMun></gIBSMunTot></gIBS>
<gCBS><vCBS>9.00</vCBS></gCBS></totCIBS></IBSCBS>
<DPS versao="1.01"><infDPS Id="DPS2211001211222333000181000010000000000000001">
<tpAmb>2</tpAmb><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><verAplic>ctech-1.0</verAplic>
<serie>1</serie><nDPS>1</nDPS><dCompet>2026-08-01</dCompet><tpEmit>1</tpEmit>
<cLocEmi>2211001</cLocEmi>
<prest><CNPJ>11222333000181</CNPJ><IM>123456</IM>
<regTrib><opSimpNac>1</opSimpNac><regEspTrib>0</regEspTrib></regTrib></prest>
<toma><CPF>12345678909</CPF><xNome>TOMADOR TESTE</xNome>
<end><endNac><cMun>2211001</cMun><CEP>64000000</CEP></endNac>
<xLgr>Rua B</xLgr><nro>200</nro><xBairro>Centro</xBairro></end></toma>
<serv><locPrest><cLocPrestacao>2211001</cLocPrestacao></locPrest>
<cServ><cTribNac>010101</cTribNac><cTribMun><cTribMun>0101</cTribMun></cTribMun>
<xDescServ>Análise e desenvolvimento de sistemas sob demanda</xDescServ>
<cNBS>115011000</cNBS><cIntContrib>SRV-1</cIntContrib></cServ>
<comExt><mdPrestacao>1</mdPrestacao><vincPrest>0</vincPrest><tpMoeda>790</tpMoeda>
<vServMoeda>200.00</vServMoeda><mecAFComexP>01</mecAFComexP><mecAFComexT>01</mecAFComexT>
<movTempBens>0</movTempBens><nDI>1234567890</nDI><mdic>0</mdic></comExt>
<obra><cObra>OBRA-1</cObra>
<end><CEP>64000000</CEP><xLgr>Rua C</xLgr><nro>300</nro><xBairro>Centro</xBairro></end></obra>
<infoCompl><docRef>Contrato 42</docRef><xPed>PED-7</xPed>
<gItemPed><xItemPed>Item 1</xItemPed><xItemPed>Item 2</xItemPed></gItemPed>
<xInfComp>Informações complementares do prestador</xInfComp></infoCompl></serv>
<valores><vServPrest><vServ>1000.00</vServ></vServPrest>
<vDescCondIncond><vDescIncond>0.00</vDescIncond></vDescCondIncond>
<vDedRed><pDR>1.00</pDR><vDR>10.00</vDR></vDedRed>
<trib><tribMun><tribISSQN>1</tribISSQN><tpRetISSQN>1</tpRetISSQN><pAliq>2.00</pAliq></tribMun>
<tribFed><piscofins><CST>01</CST><vBCPisCofins>1000.00</vBCPisCofins>
<pAliqPis>0.65</pAliqPis><pAliqCofins>3.00</pAliqCofins>
<vPis>6.50</vPis><vCofins>30.00</vCofins><tpRetPisCofins>1</tpRetPisCofins></piscofins>
<vRetIRRF>15.00</vRetIRRF><vRetCSLL>5.00</vRetCSLL></tribFed>
<totTrib><vTotTrib><vTotTribFed>36.50</vTotTribFed><vTotTribEst>0.00</vTotTribEst>
<vTotTribMun>19.80</vTotTribMun></vTotTrib>
<pTotTrib><pTotTribFed>3.65</pTotTribFed><pTotTribEst>0.00</pTotTribEst>
<pTotTribMun>1.98</pTotTribMun></pTotTrib>
<indTotTrib>0</indTotTrib><pTotTribSN>0.00</pTotTribSN></totTrib></trib></valores>
<IBSCBS><finNFSe>0</finNFSe><indFinal>1</indFinal><cIndOp>020101</cIndOp><tpOper>1</tpOper>
<tpEnteGov>4</tpEnteGov><indDest>1</indDest>
<dest><CNPJ>77888999000155</CNPJ><xNome>DESTINATARIO TESTE SA</xNome>
<end><endNac><cMun>2211001</cMun><CEP>64000222</CEP></endNac>
<xLgr>Av. E</xLgr><nro>500</nro><xBairro>Centro</xBairro></end>
<fone>8632222222</fone><email>dest@example.invalid</email></dest>
<imovel><inscImobFisc>123456789</inscImobFisc><cCIB>12345678</cCIB></imovel>
<valores><trib><gIBSCBS><CST>000</CST><cClassTrib>000001</cClassTrib></gIBSCBS></trib></valores>
</IBSCBS>
</infDPS></DPS></infNFSe></NFSe>`
}

// minimalNFSeXML só tem o obrigatório do XSD — prova que o template suprime
// quadros ausentes em vez de imprimir campos vazios.
func minimalNFSeXML() string {
	return `<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.01">
<infNFSe Id="NFS` + testNFSeKey + `">
<xLocEmi>Teresina</xLocEmi><xLocPrestacao>Teresina</xLocPrestacao><nNFSe>2</nNFSe>
<xTribNac>Serviço</xTribNac><verAplic>SNNFSE-1.0</verAplic><ambGer>1</ambGer>
<tpEmis>1</tpEmis><cStat>100</cStat><dhProc>2026-08-29T12:00:05-03:00</dhProc><nDFSe>11</nDFSe>
<emit><CNPJ>11222333000181</CNPJ><xNome>PRESTADOR TESTE LTDA</xNome>
<enderNac><xLgr>Rua A</xLgr><nro>100</nro><xBairro>Centro</xBairro>
<cMun>2211001</cMun><UF>PI</UF><CEP>64000000</CEP></enderNac></emit>
<valores><vLiq>100.00</vLiq></valores>
<DPS versao="1.01"><infDPS Id="DPS2211001211222333000181000010000000000000002">
<tpAmb>1</tpAmb><dhEmi>2026-08-29T12:00:00-03:00</dhEmi><verAplic>ctech-1.0</verAplic>
<serie>1</serie><nDPS>2</nDPS><dCompet>2026-08-01</dCompet><tpEmit>1</tpEmit>
<cLocEmi>2211001</cLocEmi>
<prest><CNPJ>11222333000181</CNPJ>
<regTrib><opSimpNac>1</opSimpNac><regEspTrib>0</regEspTrib></regTrib></prest>
<serv><locPrest><cLocPrestacao>2211001</cLocPrestacao></locPrest>
<cServ><cTribNac>010101</cTribNac><xDescServ>Serviço</xDescServ></cServ></serv>
<valores><vServPrest><vServ>100.00</vServ></vServPrest>
<trib><tribMun><tribISSQN>1</tribISSQN><tpRetISSQN>1</tpRetISSQN></tribMun>
<totTrib><vTotTrib><vTotTribFed>0.00</vTotTribFed><vTotTribEst>0.00</vTotTribEst>
<vTotTribMun>0.00</vTotTribMun></vTotTrib>
<pTotTrib><pTotTribFed>0.00</pTotTribFed><pTotTribEst>0.00</pTotTribEst>
<pTotTribMun>0.00</pTotTribMun></pTotTrib>
<indTotTrib>0</indTotTrib><pTotTribSN>0.00</pTotTribSN></totTrib></trib></valores>
</infDPS></DPS></infNFSe></NFSe>`
}

// TestNFSeTribISSQNCodesMatchCatalog prova que as constantes de TSTribISSQN
// batem com o catálogo gerado do XSD: trocar 2 por 3 aqui esconderia a alíquota
// na nota errada, e nenhum teste de renderização pegaria isso.
func TestNFSeTribISSQNCodesMatchCatalog(t *testing.T) {
	for code, want := range map[string]string{
		tribISSQNImune:      "Imunidade",
		tribISSQNExportacao: "Exportação de serviço",
		tribISSQNNaoIncid:   "Não Incidência",
	} {
		label, ok := tables.EnumLabel(tables.EnumTribISSQN, code)
		if !ok || label != want {
			t.Errorf("TSTribISSQN %q = %q (%v), esperado %q", code, label, ok, want)
		}
	}
}

func TestNFSeSuppressesAliquotaWithoutISSQNDue(t *testing.T) {
	for _, code := range []string{tribISSQNImune, tribISSQNExportacao, tribISSQNNaoIncid} {
		xml := strings.Replace(sampleNFSeXML(), "<tribISSQN>1</tribISSQN>", "<tribISSQN>"+code+"</tribISSQN>", 1)
		root, err := parseXML([]byte(xml))
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := buildNFSeContext(root, StateActive)
		if err != nil {
			t.Fatal(err)
		}
		if ctx["trib_mun"].(map[string]any)["sem_issqn_devido"] != true {
			t.Errorf("tribISSQN %s deveria suprimir a alíquota", code)
		}
	}
	root, _ := parseXML([]byte(sampleNFSeXML()))
	ctx, _ := buildNFSeContext(root, StateActive)
	if ctx["trib_mun"].(map[string]any)["sem_issqn_devido"] != false {
		t.Error("operação tributável deve imprimir a alíquota")
	}
}
