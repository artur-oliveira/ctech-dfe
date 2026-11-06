// Package dfecheck liga go-dfe/validate aos fluxos de emissão em modo shadow: valida o documento que o
// builder produziu (schema do XSD + regras do MOC) e só registra no log. Nunca devolve erro nem bloqueia,
// e um defeito no validador não derruba a emissão. A passagem a warn/block é decisão por regra (validate.Modes).
package dfecheck

import (
	"log/slog"

	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// Esquemas e famílias de regra por documento.
const (
	SpecNFe  = "nfe"
	SpecMDFe = "mdfe"
	SpecCTe  = "cte"
)

// signaturePath é preenchida depois, pelo egress.
const signaturePath = "Signature"

// Shadow valida doc (conteúdo do elemento raiz do esquema) e registra cada achado. Devolve os achados
// para teste; o fluxo de emissão os ignora.
func Shadow(spec, docKind, accessKey string, doc map[string]any) (found []validate.Violation) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dfecheck: validador falhou", "spec", spec, "panic", r)
			found = nil
		}
	}()
	s, err := validate.Load(spec)
	if err != nil {
		slog.Error("dfecheck: esquema indisponível", "spec", spec, "err", err)
		return nil
	}
	found = s.Validate(doc, validate.Options{Skip: []string{signaturePath}})
	rules, _ := validate.Apply(docKind, doc, nil)
	found = append(found, rules...)
	for _, v := range found {
		// A mensagem de schema cita o valor recusado (pode ser CPF, e-mail, nome): fica fora do log.
		// A de regra só traz valores monetários e contagens.
		msg := ""
		if v.Code == "rule" {
			msg = v.Message
		}
		slog.Warn("dfecheck: achado pré-SEFAZ", "doc", docKind, "access_key", accessKey,
			"path", v.Path, "code", v.Code, "rule", v.Rule, "cstat", v.CStat, "msg", msg)
	}
	return found
}

// ShadowEnviNFe valida o NFe de um enviNFe (NF-e e NFC-e).
func ShadowEnviNFe(accessKey string, body map[string]any) {
	env, _ := body["enviNFe"].(map[string]any)
	if nfe, ok := env["NFe"].(map[string]any); ok {
		Shadow(SpecNFe, "nfe", accessKey, nfe)
	}
}

// ShadowMDFe valida o MDFe do corpo do builder.
func ShadowMDFe(accessKey string, body map[string]any) {
	mdfe, ok := body["MDFe"].(map[string]any)
	if !ok {
		return
	}
	Shadow(SpecMDFe, "mdfe", accessKey, mdfe)
	// infModal é xs:any no XSD do MDF-e: o conteúdo do modal tem esquema próprio.
	inf, _ := mdfe["infMDFe"].(map[string]any)
	modal, _ := inf["infModal"].(map[string]any)
	for _, m := range []string{"rodo", "aereo", "aquav", "ferrov"} {
		if node, ok := modal[m].(map[string]any); ok {
			Shadow(SpecMDFe+"_"+m, "mdfe_"+m, accessKey, node)
		}
	}
}
