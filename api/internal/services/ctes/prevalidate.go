package ctes

// prevalidate.go: o que a emissão do CT-e recusa localmente (422) em vez de deixar a SEFAZ rejeitar. As regras
// vêm do catálogo de go-dfe/validate (uma implementação só, a mesma que o shadow registra); aqui só se escolhe
// quais delas recusam. As demais continuam só no log do shadow.

import (
	"fmt"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// blockRules são as regras "Obrig." do MOC do CT-e que a emissão recusa antes da SEFAZ, por id do catálogo.
// Só entram as determinísticas, provadas só com o documento. Ficam só no shadow: tabelas e cadastros externos
// (município x UF, IE, CFOP x UF), relógio da SEFAZ e o que depende de configuração.
var blockRules = validate.Modes{
	// Chave de acesso: DV (G015), UF (G007), modelo (G013) e Id = concatenação da chave (G014).
	"G015": validate.ModeBlock, "G007": validate.ModeBlock, "G013": validate.ModeBlock, "G014": validate.ModeBlock,
	// Grupo do tipo de CT-e: normal exige infCTeNorm (G016).
	"G016": validate.ModeBlock,
	// CNPJ e CPF com dígito verificador (alfanumérico incluso): emitente, PAA, responsável técnico, autXML e atores.
	"G104": validate.ModeBlock, "PAA01": validate.ModeBlock, "G210": validate.ModeBlock,
	"G193": validate.ModeBlock, "G194": validate.ModeBlock,
	"G114": validate.ModeBlock, "G115": validate.ModeBlock, "G122": validate.ModeBlock, "G123": validate.ModeBlock,
	"G132": validate.ModeBlock, "G133": validate.ModeBlock, "G140": validate.ModeBlock, "G141": validate.ModeBlock,
	"G148": validate.ModeBlock, "G149": validate.ModeBlock,
}

// gateCTe recusa o CT-e montado com regra bloqueante ou tamanho fora do leiaute, nomeando cada achado.
func gateCTe(body map[string]any) error {
	found := dfecheck.GateCTe(body, blockRules)
	if len(found) == 0 {
		return nil
	}
	errs := make([]problem.FieldError, 0, len(found))
	for _, v := range found {
		if v.Rule != "" {
			errs = append(errs, problem.Field(v.Rule, dfecheck.FieldCodeFiscalRule,
				fmt.Sprintf("regra %s do MOC (rejeição %d): %s", v.Rule, v.CStat, v.Message),
				"rule", v.Rule, "cstat", v.CStat))
			continue
		}
		code, what := dfecheck.FieldCodeTooLong, "no máximo"
		if v.Code == validate.CodeMinLength {
			code, what = dfecheck.FieldCodeTooShort, "ao menos"
		}
		errs = append(errs, problem.Field(v.Path, code,
			fmt.Sprintf("%s tem %d caracteres; o leiaute aceita %s %d", v.Path, v.Got, what, v.Limit),
			"xml_path", v.Path, "limit", v.Limit, "got", v.Got))
	}
	return problem.UnprocessableEntityFields(
		"o CT-e não passa nas validações do leiaute e do MOC; corrija os itens apontados antes de emitir", errs...,
	).WithCode(dfecheck.CodeFiscalPrevalidation)
}
