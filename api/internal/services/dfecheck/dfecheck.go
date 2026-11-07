// Package dfecheck liga go-dfe/validate aos fluxos de emissão. Shadow valida o documento que o builder produziu
// (schema do XSD + regras do MOC) e só registra no log: nunca devolve erro nem bloqueia. Gate é o portão de
// recusa: devolve só os achados que o chamador promoveu a block (regras escolhidas por id e os achados de
// tamanho do schema). Nos dois, um defeito no validador falha aberto e não derruba a emissão.
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

// Códigos dos 422 do Gate que a UI mapeia, os mesmos em todo documento: problem.code e errors[].code.
const (
	CodeFiscalPrevalidation = "fiscal_prevalidation"
	FieldCodeFiscalRule     = "fiscal_rule"
	FieldCodeTooLong        = "too_long"
	FieldCodeTooShort       = "too_short"
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

// sizeCodes são os achados de schema que o Gate recusa: tamanho de texto fora do leiaute vira rejeição 225 na
// SEFAZ, e o valor nunca é truncado em silêncio (CONDUCT.md, política de não truncar).
var sizeCodes = map[string]bool{validate.CodeMinLength: true, validate.CodeMaxLength: true}

// Gate devolve os achados que recusam a emissão: as regras que block promove a validate.ModeBlock (só elas; o
// modo padrão do catálogo não recusa nada aqui) e os achados de tamanho do schema. Falha aberta: esquema indisponível ou
// pânico no validador registram o erro e não recusam nada.
func Gate(spec, docKind string, doc map[string]any, block validate.Modes) (found []validate.Violation) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dfecheck: portão falhou", "spec", spec, "panic", r)
			found = nil
		}
	}()
	s, err := validate.Load(spec)
	if err != nil {
		slog.Error("dfecheck: esquema indisponível", "spec", spec, "err", err)
		return nil
	}
	for _, v := range s.Validate(doc, validate.Options{Skip: []string{signaturePath}}) {
		if sizeCodes[v.Code] {
			found = append(found, v)
		}
	}
	rules, _ := validate.Apply(docKind, doc, block)
	for _, v := range rules {
		if block[v.Rule] == validate.ModeBlock {
			found = append(found, v)
		}
	}
	return found
}

// GateEnviNFe é o Gate do NFe de um enviNFe (NF-e e NFC-e).
func GateEnviNFe(body map[string]any, block validate.Modes) []validate.Violation {
	env, _ := body["enviNFe"].(map[string]any)
	nfe, ok := env["NFe"].(map[string]any)
	if !ok {
		return nil
	}
	return Gate(SpecNFe, "nfe", nfe, block)
}

// GateMDFe é o Gate do MDFe do corpo do builder: o documento (regras do MOC e tamanhos) e o conteúdo do modal
// (infModal é xs:any no XSD; só os tamanhos, pois as regras do MDF-e já leem o modal pelo documento).
func GateMDFe(body map[string]any, block validate.Modes) []validate.Violation {
	mdfe, ok := body["MDFe"].(map[string]any)
	if !ok {
		return nil
	}
	found := Gate(SpecMDFe, "mdfe", mdfe, block)
	inf, _ := mdfe["infMDFe"].(map[string]any)
	modal, _ := inf["infModal"].(map[string]any)
	for _, m := range []string{"rodo", "aereo", "aquav", "ferrov"} {
		if node, ok := modal[m].(map[string]any); ok {
			found = append(found, Gate(SpecMDFe+"_"+m, "mdfe_"+m, node, nil)...)
		}
	}
	return found
}

// GateCTe é o Gate do CTe do corpo do builder: o documento e o conteúdo do modal rodoviário (infModal é xs:any).
func GateCTe(body map[string]any, block validate.Modes) []validate.Violation {
	cte, ok := body["CTe"].(map[string]any)
	if !ok {
		return nil
	}
	found := Gate(SpecCTe, "cte", cte, block)
	inf, _ := cte["infCte"].(map[string]any)
	norm, _ := inf["infCTeNorm"].(map[string]any)
	modal, _ := norm["infModal"].(map[string]any)
	if rodo, ok := modal["rodo"].(map[string]any); ok {
		found = append(found, Gate(SpecCTe+"_rodo", "cte_rodo", rodo, nil)...)
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

// ShadowCTe valida o CTe do corpo do builder. infModal é xs:any no XSD do CT-e: o rodo tem esquema próprio.
func ShadowCTe(accessKey string, body map[string]any) {
	cte, ok := body["CTe"].(map[string]any)
	if !ok {
		return
	}
	Shadow(SpecCTe, "cte", accessKey, cte)
	inf, _ := cte["infCte"].(map[string]any)
	norm, _ := inf["infCTeNorm"].(map[string]any)
	modal, _ := norm["infModal"].(map[string]any)
	if rodo, ok := modal["rodo"].(map[string]any); ok {
		Shadow(SpecCTe+"_rodo", "cte_rodo", accessKey, rodo)
	}
}
