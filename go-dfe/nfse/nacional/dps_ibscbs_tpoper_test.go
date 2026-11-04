package nacional

import (
	"testing"

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
