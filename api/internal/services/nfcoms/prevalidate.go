package nfcoms

// prevalidate.go: o que a emissão da NFCom recusa localmente (422) em vez de deixar a SEFAZ rejeitar. As regras vêm
// do catálogo de go-dfe/validate (uma implementação só, a mesma que o shadow registra); aqui só se escolhe quais
// delas recusam. As demais ficam no log do shadow.

import (
	"fmt"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// blockRules são as regras do MOC da NFCom cuja rejeição é certa e provada só com o documento: campos de contingência
// (G04, G05), composição do Id e DV da chave (G09, G11) e dígitos verificadores de CNPJ e CPF (G12, G22, G23).
var blockRules = validate.Modes{
	"G04": validate.ModeBlock, "G05": validate.ModeBlock,
	"G09": validate.ModeBlock, "G11": validate.ModeBlock,
	"G12": validate.ModeBlock, "G22": validate.ModeBlock, "G23": validate.ModeBlock,
}

// gateNFCom recusa a NFCom montada com regra bloqueante ou schema fora do leiaute, nomeando cada achado.
func gateNFCom(body map[string]any) error {
	found := dfecheck.GateNFCom(body, blockRules)
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
		"a NFCom não passa nas validações do leiaute e do MOC; corrija os itens apontados antes de emitir", errs...,
	).WithCode(dfecheck.CodeFiscalPrevalidation)
}
