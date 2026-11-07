package dfecheck

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// O portão só recusa as regras promovidas a block e os achados de tamanho; o resto continua só no shadow.
func TestGateOnlyBlocksPromotedRulesAndSizes(t *testing.T) {
	nfe := map[string]any{"infNFe": map[string]any{
		"ide":  map[string]any{"mod": "55", "finNFe": "1", "tpNFDebito": "01", "tpNF": "1"},
		"dest": map[string]any{"xNome": strings.Repeat("a", 61)},
	}}
	body := map[string]any{"enviNFe": map[string]any{"NFe": nfe}}

	found := GateEnviNFe(body, validate.Modes{"B25.1-10": validate.ModeBlock})
	var rule, size bool
	for _, v := range found {
		switch {
		case v.Rule == "B25.1-10":
			rule = true
		case v.Code == validate.CodeMaxLength && v.Path == "NFe/infNFe/dest/xNome":
			size = true
		case v.Rule != "":
			t.Errorf("regra não promovida recusou: %s", v.Rule)
		case v.Code != validate.CodeMinLength && v.Code != validate.CodeMaxLength:
			t.Errorf("achado de schema que não é de tamanho recusou: %s %s", v.Path, v.Code)
		}
	}
	if !rule || !size {
		t.Fatalf("esperado B25.1-10 e o tamanho de dest/xNome; achados: %+v", found)
	}
	if got := GateEnviNFe(map[string]any{}, nil); got != nil {
		t.Errorf("corpo sem NFe deveria passar: %v", got)
	}
}
