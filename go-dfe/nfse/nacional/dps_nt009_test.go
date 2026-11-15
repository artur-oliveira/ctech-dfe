package nacional

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

var nt009Now = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

func ibscbsDoc(ambiente int) nfse.Document {
	doc := minimalDoc()
	doc.Ambiente = ambiente
	doc.IBSCBS = &nfse.IBSCBS{
		FinNFSe: 0, CIndOp: "020101", IndDest: 0,
		Valores: nfse.IBSCBSValores{Trib: nfse.TribIBSCBS{CST: "000", CClassTrib: "000001"}},
	}
	return doc
}

// validaXSD valida o XML contra o DPS_v1.01.xsd do diretório dado. Sem xmllint no PATH o teste é pulado.
func validaXSD(t *testing.T, dir string, xml []byte) {
	t.Helper()
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint não está no PATH")
	}
	f := filepath.Join(t.TempDir(), "dps.xml")
	if err := os.WriteFile(f, xml, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("xmllint", "--noout", "--schema", filepath.Join("testdata", dir, "DPS_v1.01.xsd"), f).CombinedOutput()
	if err != nil {
		t.Fatalf("XML não valida em %s: %v\n%s", dir, err, out)
	}
}

// Produção restrita (tpAmb 2) usa o leiaute da NT 009: finNFSe em infDPS (regressão do E1235 "invalid child
// element 'cLocEmi'... expected 'cMotivoEmisTI, finNFSe'") e CST/cClassTrib direto em trib.
func TestBuildDPS_NT009LayoutInHomologation(t *testing.T) {
	doc := ibscbsDoc(2)
	doc.Servico.CServ.CNBS = "101011100"
	doc.Servico.CServ.CIntContrib = "SRV1"
	doc.IBSCBS.Valores.Trib.CCredPres = "01"
	doc.IBSCBS.Valores.Trib.Dif = &nfse.DifIBSCBS{PDifUF: "10.00", PDifMun: "10.00", PDifCBS: "10.00"}
	raw, _, err := BuildDPS(doc, nt009Now)
	if err != nil {
		t.Fatal(err)
	}
	x := string(raw)
	for _, want := range []string{
		"<tpEmit>1</tpEmit><finNFSe>0</finNFSe><cLocEmi>",
		"<IBSCBS><cIndOp>",
		// regressão do E1235 "cServ has invalid child element 'cNBS'": xDescServ vai por último na NT 009.
		"<cTribNac>010101</cTribNac><cNBS>101011100</cNBS><cIntContrib>SRV1</cIntContrib><xDescServ>",
		"<trib><CST>000</CST><cClassTrib>000001</cClassTrib><gIBSCBS><cCredPres>01</cCredPres><gDif>",
	} {
		if !strings.Contains(x, want) {
			t.Errorf("XML não contém %q\n%s", want, x)
		}
	}
	validaXSD(t, "xsd_nt009", raw)
}

func TestBuildDPS_NT009LayoutWithoutOptionalGroup(t *testing.T) {
	raw, _, err := BuildDPS(ibscbsDoc(2), nt009Now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "<trib><CST>000</CST><cClassTrib>000001</cClassTrib></trib>") {
		t.Errorf("gIBSCBS vazio deveria ser omitido: %s", raw)
	}
	validaXSD(t, "xsd_nt009", raw)
}

// Sem grupo IBSCBS no documento o infDPS da NT 009 ainda exige finNFSe.
func TestBuildDPS_NT009FinNFSeWithoutIBSCBS(t *testing.T) {
	doc := minimalDoc()
	doc.Ambiente = 2
	raw, _, err := BuildDPS(doc, nt009Now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "<finNFSe>0</finNFSe>") {
		t.Errorf("faltou finNFSe em infDPS: %s", raw)
	}
}

// Produção (tpAmb 1) continua no leiaute de 20260209 até a Sefin publicar a virada.
func TestBuildDPS_ProductionKeepsLegacyLayout(t *testing.T) {
	raw, _, err := BuildDPS(ibscbsDoc(1), nt009Now)
	if err != nil {
		t.Fatal(err)
	}
	x := string(raw)
	if strings.Contains(x, "<tpEmit>1</tpEmit><finNFSe>") {
		t.Errorf("produção não deve ter finNFSe em infDPS: %s", x)
	}
	if !strings.Contains(x, "<IBSCBS><finNFSe>0</finNFSe><cIndOp>") ||
		!strings.Contains(x, "<trib><gIBSCBS><CST>000</CST><cClassTrib>000001</cClassTrib></gIBSCBS></trib>") {
		t.Errorf("produção perdeu o leiaute antigo: %s", x)
	}
	// Sem xmllint aqui: o DPS_v1.01.xsd de 20260209 declara a série com ^ e $ literais, que o libxml2 não
	// aceita. A conformidade de produção é coberta por TestDPSCoverageMatchesSchema.
}

func TestBuildDPS_NT009RejectsRemovedGroups(t *testing.T) {
	ded := minimalDoc()
	ded.Ambiente = 2
	ded.Valores.VDedRed = &nfse.DedRed{PDR: "10.00"}
	if _, _, err := BuildDPS(ded, nt009Now); err == nil || !strings.Contains(err.Error(), "vAjusteBC") {
		t.Errorf("vDedRed deveria ser recusado na NT 009: %v", err)
	}

	ree := ibscbsDoc(2)
	ree.IBSCBS.Valores.ReeRepRes = []nfse.ReeRepResDoc{{
		DtEmiDoc: "2026-08-01", DtCompDoc: "2026-08-01", TpReeRepRes: "1", VlrReeRepRes: "1.00",
		DocFiscalOutro: &nfse.ReeRepResDocFiscal{CMunDocFiscal: "2211001", NDocFiscal: "1", XDocFiscal: "x"},
	}}
	if _, _, err := BuildDPS(ree, nt009Now); err == nil || !strings.Contains(err.Error(), "vAjusteBC") {
		t.Errorf("gReeRepRes deveria ser recusado na NT 009: %v", err)
	}

	imo := ibscbsDoc(2)
	imo.IBSCBS.Imovel = &nfse.Imovel{InscImobFisc: "999", CIB: "CIB002"}
	if _, _, err := BuildDPS(imo, nt009Now); err == nil || !strings.Contains(err.Error(), "imovel") {
		t.Errorf("imovel deveria ser recusado na NT 009: %v", err)
	}
}

// E0121/E0128/E0225/E0236: o emitente não leva nome nem endereço.
func TestDPSEmitenteSemNomeEEndereco(t *testing.T) {
	end := &nfse.Endereco{XLgr: "Rua A", Nro: "1", XBairro: "Centro",
		CMun: "2211001", CEP: "64000000"}
	doc := minimalDoc()
	doc.Prestador.XNome, doc.Prestador.End = "Prestador Teste", end
	doc.Tomador.End = end
	for _, tc := range []struct {
		tpEmit            int
		prestTem, tomaTem bool
	}{{1, false, true}, {2, true, false}} {
		doc.TpEmit = tc.tpEmit
		p := toXMLPrestador(doc.Prestador, doc.TpEmit)
		if got := p.XNome != "" && p.End != nil; got != tc.prestTem {
			t.Errorf("tpEmit=%d: prestador com nome e endereço=%v, quer %v", tc.tpEmit, got, tc.prestTem)
		}
		x := toXMLTomador(doc.Tomador, doc.TpEmit)
		if got := x.XNome != "" || x.End != nil; got != tc.tomaTem {
			t.Errorf("tpEmit=%d: tomador com nome ou endereço=%v, quer %v", tc.tpEmit, got, tc.tomaTem)
		}
	}
}
