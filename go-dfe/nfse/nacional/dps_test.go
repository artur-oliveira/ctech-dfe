package nacional

import (
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
