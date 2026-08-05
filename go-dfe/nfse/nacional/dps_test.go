package nacional

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func minimalDoc() nfse.Document {
	return nfse.Document{
		Ambiente: 2, VerAplic: "ctech-1.0", TpEmit: 1,
		Competencia: "2026-08-01", Serie: "1", Numero: 42, CLocEmi: "2211001",
		Prestador: nfse.Prestador{
			Pessoa:  nfse.Pessoa{CNPJ: "11222333000181", IM: "123456"},
			RegTrib: nfse.RegTrib{OpSimpNac: 1, RegEspTrib: 0},
		},
		Tomador: &nfse.Pessoa{CPF: "12345678909", XNome: "Tomador Teste"},
		Servico: nfse.Servico{
			LocPrest: nfse.LocPrest{CLocPrestacao: "2211001"},
			CServ:    nfse.CServ{CTribNac: "010101", XDescServ: "Análise de sistemas"},
		},
		Valores: nfse.Valores{
			VServPrest: nfse.VServPrest{VServ: "1000.00"},
			Trib: nfse.Tributacao{
				TribMun: nfse.TribMunicipal{TribISSQN: 1, TpRetISSQN: 1, PAliq: "2.00"},
			},
		},
	}
}

func TestBuildIDDPS(t *testing.T) {
	got := BuildIDDPS("2211001", "2", "11222333000181", "1", 42)
	want := "DPS" + "2211001" + "2" + "11222333000181" + "00001" + "000000000000042"
	if got != want {
		t.Fatalf("BuildIDDPS = %q, esperado %q", got, want)
	}
	if len(got) != 45 {
		t.Errorf("len = %d, esperado 45 (TSIdDPS)", len(got))
	}
}

func TestBuildDPS_ElementOrder(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	xmlBytes, idDPS, err := BuildDPS(minimalDoc(), now)
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)

	if !strings.Contains(s, `Id="`+idDPS+`"`) {
		t.Errorf("infDPS sem Id=%q", idDPS)
	}
	if !strings.Contains(s, `xmlns="`+nfse.Namespace+`"`) {
		t.Errorf("namespace ausente")
	}
	// A ordem de TCInfDPS é normativa: tpAmb, dhEmi, verAplic, serie, nDPS,
	// dCompet, tpEmit, cLocEmi, [subst], prest, [toma], [interm], serv,
	// valores, [IBSCBS].
	order := []string{"<tpAmb>", "<dhEmi>", "<verAplic>", "<serie>", "<nDPS>",
		"<dCompet>", "<tpEmit>", "<cLocEmi>", "<prest>", "<toma>", "<serv>", "<valores>"}
	prev := -1
	for _, tag := range order {
		i := strings.Index(s, tag)
		if i < 0 {
			t.Fatalf("tag %s ausente no XML", tag)
		}
		if i < prev {
			t.Errorf("tag %s fora de ordem", tag)
		}
		prev = i
	}
}

func TestBuildDPS_OmitsEmptyOptionalGroups(t *testing.T) {
	xmlBytes, _, err := BuildDPS(minimalDoc(), time.Now())
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)
	for _, tag := range []string{"<subst>", "<interm>", "<IBSCBS>", "<comExt>",
		"<obra>", "<atvEvento>", "<vDedRed>", "<end>"} {
		if strings.Contains(s, tag) {
			t.Errorf("grupo opcional vazio %s não deveria aparecer", tag)
		}
	}
}

func TestBuildDPS_RejectsInvalidTribNacional(t *testing.T) {
	doc := minimalDoc()
	doc.Servico.CServ.CTribNac = "99999"
	if _, _, err := BuildDPS(doc, time.Now()); err == nil {
		t.Fatal("esperado erro para cTribNac inexistente no Anexo B")
	}
}

func TestBuildDPS_RequiresMotivoWhenTpEmitNotPrestador(t *testing.T) {
	doc := minimalDoc()
	doc.TpEmit = 2
	if _, _, err := BuildDPS(doc, time.Now()); err == nil {
		t.Fatal("esperado erro: cMotivoEmisTI é obrigatório quando tpEmit != 1")
	}
}

func TestBuildDPS_MatchesGolden(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	got, _, err := BuildDPS(minimalDoc(), now)
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	want, err := os.ReadFile("../testdata/dps_minima.xml")
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	if string(got) != strings.TrimSpace(string(want)) {
		t.Errorf("DPS divergiu do golden.\ngot:  %s\nwant: %s", got, want)
	}
}

// TestBuildDPS_OptionalGroupsMatchXSDShape exercita Obra, AtvEvento,
// InfoCompl e BM — grupos cujo shape (nomes/ordem de elemento) precisa
// bater exatamente com tiposComplexos_v1.01.xsd; um regression test para o
// bug em que essas structs foram inicialmente modeladas com campos/tipos
// que não existem no XSD real (ex.: obra sem escolha cObra|cCIB|end,
// BM com tBM/vlRed em vez de nBM+vRedBCBM|pRedBCBM).
func TestBuildDPS_OptionalGroupsMatchXSDShape(t *testing.T) {
	doc := minimalDoc()
	doc.Servico.Obra = &nfse.Obra{InscImobFisc: "123", CCIB: "CIB001"}
	doc.Servico.AtvEvento = &nfse.AtvEvento{
		XNome: "Feira", DtIni: "2026-08-01", DtFim: "2026-08-02",
		End: &nfse.EnderecoSimples{CEP: "01000000", XLgr: "Rua A", Nro: "10", XBairro: "Centro"},
	}
	doc.Servico.InfoCompl = &nfse.InfoCompl{XPed: "PED1", ItensPed: []string{"1", "2"}}
	doc.Valores.Trib.TribMun.BM = &nfse.BenefMun{NBM: "2211001010001", PRedBCBM: "50.00"}

	xmlBytes, _, err := BuildDPS(doc, time.Now())
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)

	for _, want := range []string{
		"<obra><inscImobFisc>123</inscImobFisc><cCIB>CIB001</cCIB></obra>",
		"<atvEvento><xNome>Feira</xNome><dtIni>2026-08-01</dtIni><dtFim>2026-08-02</dtFim>" +
			"<end><CEP>01000000</CEP><xLgr>Rua A</xLgr><nro>10</nro><xBairro>Centro</xBairro></end></atvEvento>",
		"<infoCompl><xPed>PED1</xPed><gItemPed><xItemPed>1</xItemPed><xItemPed>2</xItemPed></gItemPed></infoCompl>",
		"<BM><nBM>2211001010001</nBM><pRedBCBM>50.00</pRedBCBM></BM>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("XML não contém %q\nXML: %s", want, s)
		}
	}
}

// TestToXMLTotTrib_ChoosesExactlyOneBranch é um regression test para o bug
// em que TCTribTotal (xs:choice) era serializado com múltiplos ramos
// simultâneos e indTotTrib=0 (o ramo padrão) nunca aparecia por causa de
// omitempty num int não-ponteiro.
func TestToXMLTotTrib_ChoosesExactlyOneBranch(t *testing.T) {
	cases := []struct {
		name string
		in   nfse.TotTrib
		want string
	}{
		{"default indTotTrib", nfse.TotTrib{}, "<indTotTrib>0</indTotTrib>"},
		{"vTotTrib", nfse.TotTrib{VTotTribFed: "1.00", VTotTribEst: "2.00", VTotTribMun: "3.00"},
			"<vTotTrib><vTotTribFed>1.00</vTotTribFed><vTotTribEst>2.00</vTotTribEst><vTotTribMun>3.00</vTotTribMun></vTotTrib>"},
		{"pTotTrib", nfse.TotTrib{PTotTribFed: "1.00", PTotTribEst: "2.00", PTotTribMun: "3.00"},
			"<pTotTrib><pTotTribFed>1.00</pTotTribFed><pTotTribEst>2.00</pTotTribEst><pTotTribMun>3.00</pTotTribMun></pTotTrib>"},
		{"pTotTribSN", nfse.TotTrib{PTotTribSN: "4.00"}, "<pTotTribSN>4.00</pTotTribSN>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := xml.Marshal(toXMLTotTrib(c.in))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(got), c.want) {
				t.Errorf("xmlTotTrib = %s, esperado conter %q", got, c.want)
			}
		})
	}
}

// TestBuildDPS_IBSCBSTribShape é um regression test para o bug em que
// IBSCBS.valores.trib.gIBSCBS era serializado com campos fabricados
// (vBC/gIBSUF/gIBSMun/gCBS/vTotIBS/vTotCBS) que não existem em
// TCRTCInfoTributosSitClas — o shape real é CST/cClassTrib/cCredPres?/
// gTribRegular?/gDif?.
func TestBuildDPS_IBSCBSTribShape(t *testing.T) {
	doc := minimalDoc()
	doc.IBSCBS = &nfse.IBSCBS{
		FinNFSe: 0, CIndOp: "020101", IndDest: 0,
		GRefNFSe: &nfse.RefNFSe{Chaves: []string{"chave1", "chave2"}},
		Imovel:   &nfse.Imovel{InscImobFisc: "999", CIB: "CIB002"},
		Valores: nfse.IBSCBSValores{
			Trib: nfse.TribIBSCBS{
				CST: "000", CClassTrib: "000001",
				TribRegular: &nfse.TribRegular{CSTReg: "000", CClassTribReg: "000001"},
				Dif:         &nfse.DifIBSCBS{PDifUF: "10.00", PDifMun: "10.00", PDifCBS: "10.00"},
			},
		},
	}
	xmlBytes, _, err := BuildDPS(doc, time.Now())
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)
	for _, want := range []string{
		"<gRefNFSe><refNFSe>chave1</refNFSe><refNFSe>chave2</refNFSe></gRefNFSe>",
		"<imovel><inscImobFisc>999</inscImobFisc><cCIB>CIB002</cCIB></imovel>",
		"<valores><trib><gIBSCBS><CST>000</CST><cClassTrib>000001</cClassTrib>" +
			"<gTribRegular><CSTReg>000</CSTReg><cClassTribReg>000001</cClassTribReg></gTribRegular>" +
			"<gDif><pDifUF>10.00</pDifUF><pDifMun>10.00</pDifMun><pDifCBS>10.00</pDifCBS></gDif>" +
			"</gIBSCBS></trib></valores>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("XML não contém %q\nXML: %s", want, s)
		}
	}
	for _, unwanted := range []string{"<vBC>", "<gIBSUF>", "<gIBSMun>", "<gCBS>", "<vTotIBS>", "<vTotCBS>"} {
		if strings.Contains(s, unwanted) {
			t.Errorf("XML contém campo fabricado %q que não existe em TCRTCInfoTributosSitClas", unwanted)
		}
	}
}

// TestBuildDPS_DhEmiUsesNumericOffsetNotZ é um regression test: TSDateTimeUTC
// (tiposSimples_v1.01.xsd) exige TZD numérico (+hh:mm|-hh:mm) e não aceita o
// sufixo "Z" — time.RFC3339 emite "Z" para UTC, o que produzia dhEmi
// schema-inválido em toda DPS gerada.
func TestBuildDPS_DhEmiUsesNumericOffsetNotZ(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	xmlBytes, _, err := BuildDPS(minimalDoc(), now)
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)
	if strings.Contains(s, "<dhEmi>2026-08-04T12:00:00Z</dhEmi>") {
		t.Error("dhEmi usa sufixo Z — TSDateTimeUTC não aceita, exige offset numérico")
	}
	if !strings.Contains(s, "<dhEmi>2026-08-04T12:00:00+00:00</dhEmi>") {
		t.Errorf("dhEmi não usa o offset numérico esperado: %s", s)
	}
}

// TestBuildDPS_ComExtRequiredFieldsNeverOmitted é um regression test: todos
// os campos de TCComExterior até movTempBens (mais mdic) são obrigatórios no
// XSD; omitempty num int/string zero-value os descartava silenciosamente, e
// mecAFComexP/T são enums string de 2 dígitos ("00"), nunca int.
func TestBuildDPS_ComExtRequiredFieldsNeverOmitted(t *testing.T) {
	doc := minimalDoc()
	doc.Servico.ComExt = &nfse.ComExt{
		MdPrestacao: 0, VincPrest: 0, TpMoeda: "USD", VServMoeda: "100.00",
		MecAFComexP: "00", MecAFComexT: "00", MovTempBens: 0, MDIC: 0,
	}
	xmlBytes, _, err := BuildDPS(doc, time.Now())
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)
	for _, want := range []string{
		"<mdPrestacao>0</mdPrestacao>", "<vincPrest>0</vincPrest>",
		"<mecAFComexP>00</mecAFComexP>", "<mecAFComexT>00</mecAFComexT>",
		"<movTempBens>0</movTempBens>", "<mdic>0</mdic>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("campo obrigatório %q com valor zero foi descartado\nXML: %s", want, s)
		}
	}
}

// TestBuildDPS_ZeroValueEnumsPreserved é um regression test para campos cujo
// domínio inclui 0 como valor legítimo ("não informado"/"não"): cNaoNIF,
// tpImunidade, tpRetPisCofins, indFinal — omitempty num int simples
// descartava esse 0 e deixava o xs:choice/grupo sem nenhum membro.
func TestBuildDPS_ZeroValueEnumsPreserved(t *testing.T) {
	doc := minimalDoc()
	zero := 0
	doc.Tomador.CPF = ""
	doc.Tomador.NIF = "12345"
	doc.Tomador.CNaoNIF = &zero
	doc.Valores.Trib.TribMun.TpImunidade = &zero
	doc.Valores.Trib.TribFed = &nfse.TribFederal{CST: "01", TpRetPisCofins: &zero}
	doc.IBSCBS = &nfse.IBSCBS{
		CIndOp: "020101", IndDest: 0, IndFinal: &zero,
		Valores: nfse.IBSCBSValores{Trib: nfse.TribIBSCBS{CST: "000", CClassTrib: "000001"}},
	}

	xmlBytes, _, err := BuildDPS(doc, time.Now())
	if err != nil {
		t.Fatalf("BuildDPS: %v", err)
	}
	s := string(xmlBytes)
	for _, want := range []string{
		"<cNaoNIF>0</cNaoNIF>", "<tpImunidade>0</tpImunidade>",
		"<tpRetPisCofins>0</tpRetPisCofins>", "<indFinal>0</indFinal>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("valor 0 legítimo de enum foi descartado, esperado %q\nXML: %s", want, s)
		}
	}
}

// TestBuildDPS_IBSCBSDestRejectsCAEPFAndIM é um regression test: TCRTCInfoDest
// não tem CAEPF nem IM (diferente de TCInfoPessoa, usado em toma/interm) —
// BuildDPS deve falhar explicitamente, nunca descartar em silêncio.
func TestBuildDPS_IBSCBSDestRejectsCAEPFAndIM(t *testing.T) {
	doc := minimalDoc()
	doc.IBSCBS = &nfse.IBSCBS{
		CIndOp: "020101", IndDest: 1,
		Dest:    &nfse.Pessoa{CPF: "12345678909", XNome: "Dest Teste", CAEPF: "12345678000190"},
		Valores: nfse.IBSCBSValores{Trib: nfse.TribIBSCBS{CST: "000", CClassTrib: "000001"}},
	}
	if _, _, err := BuildDPS(doc, time.Now()); err == nil {
		t.Fatal("esperado erro: TCRTCInfoDest não suporta CAEPF")
	}
}
