package mdfes

// prevalidate.go: o que a emissão do MDF-e recusa localmente (422) em vez de deixar a SEFAZ rejeitar. As regras
// vêm do catálogo de go-dfe/validate (uma implementação só, a mesma que o shadow registra); aqui só se escolhe
// quais delas recusam. As demais continuam só no log do shadow.

import (
	"fmt"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// blockRules são as regras "Obrig." do MOC do MDF-e que a emissão recusa antes da SEFAZ, por id do catálogo.
// Só entram as determinísticas, provadas só com o documento e com a condição que o MOC escreve por inteiro.
// Ficam só no shadow: datas comparadas com o relógio da SEFAZ (F30a, F37a, F45a, F36b, F79, F80), tabelas que o
// leiaute não traz (F08, F11, F89, F90), cadastros externos (F113, ANTT) e o que depende de configuração (F72).
var blockRules = validate.Modes{
	// Chave de acesso: DV e composição (CNPJ/CPF, ano, mês, modelo, número, tpEmis, UF).
	"F05": validate.ModeBlock, "F30": validate.ModeBlock, "F36d": validate.ModeBlock,
	"F37": validate.ModeBlock, "F45": validate.ModeBlock,
	// CPF e CNPJ com dígito verificador.
	"F56": validate.ModeBlock, "F57": validate.ModeBlock, "F67": validate.ModeBlock, "F68": validate.ModeBlock,
	"F96": validate.ModeBlock, "F98": validate.ModeBlock, "F100": validate.ModeBlock, "F101": validate.ModeBlock,
	"F102": validate.ModeBlock, "F103": validate.ModeBlock, "F104": validate.ModeBlock, "F105": validate.ModeBlock,
	"F106": validate.ModeBlock, "F121": validate.ModeBlock,
	// Contagens e duplicidades: quantidade de documentos (F51), chave repetida (F28, F29, F36c), município
	// repetido (F10, F13), condutor (F99), contratante (F66) e autorizado ao XML (F107) repetidos.
	"F51": validate.ModeBlock, "F28": validate.ModeBlock, "F29": validate.ModeBlock, "F36c": validate.ModeBlock,
	"F10": validate.ModeBlock, "F13": validate.ModeBlock, "F99": validate.ModeBlock, "F66": validate.ModeBlock,
	"F107": validate.ModeBlock,
	// Totais do pagamento: componentes = contrato (F58), parcelas + adiantamento = contrato (F62), R$ 0,01 de tolerância.
	"F58": validate.ModeBlock, "F62": validate.ModeBlock,
	// Tipo de emitente x documentos (F14 a F17) e carregamento posterior (F21 a F24, F26, F27).
	"F14": validate.ModeBlock, "F15": validate.ModeBlock, "F16": validate.ModeBlock, "F17": validate.ModeBlock,
	"F21": validate.ModeBlock, "F22": validate.ModeBlock, "F23": validate.ModeBlock, "F24": validate.ModeBlock,
	"F26": validate.ModeBlock, "F27": validate.ModeBlock,
}

// gateMDFe recusa o MDF-e montado com regra bloqueante ou tamanho fora do leiaute, nomeando cada achado.
// errors[]: regra do MOC com field = id da regra, code fiscal_rule e params rule e cstat; tamanho com
// field = caminho XML, code too_long ou too_short e params xml_path, limit e got.
func gateMDFe(body map[string]any) error {
	found := dfecheck.GateMDFe(body, blockRules)
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
		errs = append(errs, dfecheck.SchemaFieldError(v))
	}
	return problem.UnprocessableEntityFields(
		"o MDF-e não passa nas validações do leiaute e do MOC; corrija os itens apontados antes de emitir", errs...,
	).WithCode(dfecheck.CodeFiscalPrevalidation)
}
