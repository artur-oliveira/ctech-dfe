package bpes

// prevalidate.go: o que a emissão do BP-e recusa localmente (422) em vez de deixar a SEFAZ rejeitar. As regras vêm
// do catálogo de go-dfe/validate (uma implementação só, a mesma que o shadow registra); aqui só se escolhe quais
// delas recusam. As demais ficam no log do shadow.

import (
	"fmt"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// blockRules são as regras do MOC do BP-e cuja rejeição é certa e provada só com o documento: campos de contingência
// (F05, F06), composição do Id e DV da chave (F09, F11) e dígitos verificadores de CNPJ e CPF (F18, F24, F34, F35).
var blockRules = validate.Modes{
	"F05": validate.ModeBlock, "F06": validate.ModeBlock,
	"F09": validate.ModeBlock, "F11": validate.ModeBlock,
	"F18": validate.ModeBlock, "F24": validate.ModeBlock, "F34": validate.ModeBlock, "F35": validate.ModeBlock,
}

// specBPe e specBPeTM são as famílias de esquema e regras do BP-e comum e do TM e do TA.
const (
	specBPe   = dfecheck.SpecBPe
	specBPeTM = dfecheck.SpecBPeTM
	specBPeTA = dfecheck.SpecBPeTA
)

// gateBPe recusa o BP-e montado com regra bloqueante ou schema fora do leiaute, nomeando cada achado.
func gateBPe(variant string, body map[string]any) error {
	found := dfecheck.GateBPe(variant, body, blockRules)
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
		"o BP-e não passa nas validações do leiaute e do MOC; corrija os itens apontados antes de emitir", errs...,
	).WithCode(dfecheck.CodeFiscalPrevalidation)
}
