package mdfes

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

// Paridade builder x XSD do MDF-e: nenhuma tag fora do leiaute e valores dentro de tipo, tamanho e
// casas decimais. Obrigatórios ausentes não contam: a fixture mínima não preenche o documento inteiro.
func TestBuildMDFe_ParidadeComXSD(t *testing.T) {
	out := BuildMDFe(baseParams(nil))
	body := out["MDFe"].(map[string]any)
	findings := dfecheck.Shadow(dfecheck.SpecMDFe, "mdfe", "teste", body)
	modal := body["infMDFe"].(map[string]any)["infModal"].(map[string]any)
	findings = append(findings, dfecheck.Shadow("mdfe_rodo", "mdfe_rodo", "teste", modal["rodo"].(map[string]any))...)
	for _, v := range findings {
		// chNFe da fixture de carga é fictícia ("111"); só o formato dela destoa.
		if v.Code == "pattern" && strings.HasSuffix(v.Path, "/chNFe") {
			continue
		}
		switch v.Code {
		case "required", "choice_missing", "rule":
			continue
		}
		t.Errorf("%s %s: %s", v.Path, v.Code, v.Message)
	}
}
