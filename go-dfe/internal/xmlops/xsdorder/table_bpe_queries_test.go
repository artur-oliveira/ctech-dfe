package xsdorder

import (
	"fmt"
	"testing"
)

// As consultas do BP-e saem na ordem do XSD (a reconciliação do 204 usa consSitBPe).
func TestResolveBPeQueries(t *testing.T) {
	want := map[string][]string{
		"consSitBPe":      {"tpAmb", "xServ", "chBPe"},
		"consStatServBPe": {"tpAmb", "xServ"},
	}
	for tag, order := range want {
		got, ok := Resolve("", tag)
		if !ok || fmt.Sprint(got) != fmt.Sprint(order) {
			t.Errorf("%s: ok=%v order=%v, want %v", tag, ok, got, order)
		}
	}
}
