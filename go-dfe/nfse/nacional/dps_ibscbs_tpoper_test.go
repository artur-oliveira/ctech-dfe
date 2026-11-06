package nacional

import (
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func TestToXMLIBSCBS_TpOperE0904(t *testing.T) {
	cases := []struct {
		name, cTribNac string
		enteGov, want  int
	}{
		{"servico comum descarta", "010101", 0, 0},
		{"item 25.05 mantem", "250501", 0, 2},
		{"item 10.05 mantem", "100501", 0, 2},
		{"compra governamental mantem", "010101", 1, 2},
	}
	for _, c := range cases {
		out, err := toXMLIBSCBS(&nfse.IBSCBS{TpOper: 2, TpEnteGov: c.enteGov}, c.cTribNac)
		if err != nil {
			t.Fatal(err)
		}
		if out.TpOper != c.want {
			t.Errorf("%s: tpOper=%d, want %d", c.name, out.TpOper, c.want)
		}
	}
}

// Ponta a ponta no XML do DPS: serviço comum (cTribNac 010101) com tpOper no documento não pode levar a tag,
// que o Sefin rejeita com E0904; compra governamental e os itens liberados continuam levando.
func TestBuildDPS_TpOperForaDoXML(t *testing.T) {
	cases := []struct {
		name, cTribNac string
		enteGov        int
		want           bool
	}{
		{"servico comum", "010101", 0, false},
		{"compra governamental", "010101", 1, true},
		{"item 25.05", "250501", 0, true},
	}
	for _, c := range cases {
		doc := minimalDoc()
		doc.Servico.CServ.CTribNac = c.cTribNac
		doc.IBSCBS = &nfse.IBSCBS{FinNFSe: 0, CIndOp: "100301", TpOper: 1, TpEnteGov: c.enteGov,
			Valores: nfse.IBSCBSValores{Trib: nfse.TribIBSCBS{CST: "000", CClassTrib: "000001"}}}
		out, _, err := BuildDPS(doc, time.Now())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := strings.Contains(string(out), "<tpOper>"); got != c.want {
			t.Errorf("%s: tpOper no XML = %v, want %v", c.name, got, c.want)
		}
	}
}
