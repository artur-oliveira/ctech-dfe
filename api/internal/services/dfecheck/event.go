package dfecheck

import (
	"fmt"
	"log/slog"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// Specs dos eventos (go-dfe/validate/specs/evento_*.json): o envelope de cada documento e, por evento, o
// conteúdo de detEvento (NF-e, pelo tpEvento) ou o grupo evXxx dentro dele (CT-e e MDF-e, pelo nome do grupo).
const (
	specEventNFePrefix  = "evento_nfe_"
	specEventNFeEnv     = "evento_nfe_envEvento"
	specEventCTeEnv     = "evento_cte_eventoCTe"
	specEventMDFeEnv    = "evento_mdfe_eventoMDFe"
	specEventCTePrefix  = "evento_cte_"
	specEventMDFePrefix = "evento_mdfe_"
	// Caminhos preenchidos depois, pelo egress (assinatura do evento).
	signaturePathNFeEvent = "evento/Signature"
)

// CheckEvent valida doc, o conteúdo do elemento raiz do spec de evento, sem registrar nada. Spec que não
// existe (evento sem XSD vendorizado) devolve nil.
func CheckEvent(spec string, doc map[string]any, skip ...string) []validate.Violation {
	s, err := validate.Load(spec)
	if err != nil {
		return nil
	}
	return s.Validate(doc, validate.Options{Skip: append([]string{signaturePath}, skip...)})
}

// EventFindings valida o corpo de evento que o builder produziu (envelope + detEvento). doc é "nfe", "cte" ou
// "mdfe". Usado pelos testes de paridade e por ShadowEvent.
func EventFindings(doc string, body map[string]any) []validate.Violation {
	var found []validate.Violation
	switch doc {
	case "nfe":
		env, _ := body["envEvento"].(map[string]any)
		found = CheckEvent(specEventNFeEnv, env, signaturePathNFeEvent)
		evento, _ := env["evento"].(map[string]any)
		inf, _ := evento["infEvento"].(map[string]any)
		det, _ := inf["detEvento"].(map[string]any)
		tp, _ := inf["tpEvento"].(string)
		found = append(found, CheckEvent(specEventNFePrefix+tp, det)...)
	case "cte", "mdfe":
		envSpec, prefix, rootKey := specEventCTeEnv, specEventCTePrefix, "eventoCTe"
		if doc == "mdfe" {
			envSpec, prefix, rootKey = specEventMDFeEnv, specEventMDFePrefix, "eventoMDFe"
		}
		env, _ := body[rootKey].(map[string]any)
		found = CheckEvent(envSpec, env)
		inf, _ := env["infEvento"].(map[string]any)
		det, _ := inf["detEvento"].(map[string]any)
		for k, v := range det {
			if g, ok := v.(map[string]any); ok && !strings.HasPrefix(k, "@") {
				found = append(found, CheckEvent(prefix+k, g)...)
			}
		}
	}
	return found
}

// ShadowEvent valida o corpo do evento e só registra: nunca devolve erro nem bloqueia (falha aberta).
func ShadowEvent(doc, accessKey string, body map[string]any) (found []validate.Violation) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dfecheck: validador de evento falhou", "doc", doc, "panic", r)
			found = nil
		}
	}()
	found = EventFindings(doc, body)
	for _, v := range found {
		slog.Warn("dfecheck: achado pré-SEFAZ de evento", "doc", doc, "access_key", accessKey,
			"path", v.Path, "code", v.Code)
	}
	return found
}

// GateEvent é o portão do corpo de evento (NF-e, CT-e, MDF-e): os achados de schema que o modo de KindXEvent
// recusa (blocks). Os eventos não têm regras de catálogo. Falha aberta como Gate.
func GateEvent(doc string, body map[string]any) (found []validate.Violation) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("dfecheck: portão de evento falhou", "doc", doc, "panic", r)
			found = nil
		}
	}()
	mode := SchemaMode(Kind(doc + "_event"))
	for _, v := range EventFindings(doc, body) {
		if blocks(mode, v.Code) {
			found = append(found, v)
		}
	}
	return found
}

// EventProblem monta o 422 fiscal_prevalidation dos achados do portão de evento, ou nil sem achados. label é o
// documento em pt-BR ("a NF-e"); só caminho e motivo vão na mensagem, nunca o valor recusado.
func EventProblem(label string, found []validate.Violation) error {
	if len(found) == 0 {
		return nil
	}
	return problem.UnprocessableEntityFields(
		"o evento de "+label+" não passa nas validações do leiaute; corrija os itens apontados antes de enviar",
		SchemaFieldErrors(found)...,
	).WithCode(CodeFiscalPrevalidation)
}

// SchemaFieldErrors converte achados de schema em errors[] (field = caminho XML, code = código do achado do
// validador ou too_long/too_short, params xml_path, mais limit/got nos de tamanho). Mensagem em pt-BR sem o valor.
func SchemaFieldErrors(found []validate.Violation) []problem.FieldError {
	errs := make([]problem.FieldError, 0, len(found))
	for _, v := range found {
		errs = append(errs, SchemaFieldError(v))
	}
	return errs
}

// SchemaFieldError traduz um achado de schema (nunca de regra) em FieldError.
func SchemaFieldError(v validate.Violation) problem.FieldError {
	switch v.Code {
	case validate.CodeMaxLength:
		return problem.Field(v.Path, FieldCodeTooLong,
			fmt.Sprintf("%s tem %d caracteres; o leiaute aceita no máximo %d", v.Path, v.Got, v.Limit),
			"xml_path", v.Path, "limit", v.Limit, "got", v.Got)
	case validate.CodeMinLength:
		return problem.Field(v.Path, FieldCodeTooShort,
			fmt.Sprintf("%s tem %d caracteres; o leiaute aceita ao menos %d", v.Path, v.Got, v.Limit),
			"xml_path", v.Path, "limit", v.Limit, "got", v.Got)
	}
	reasons := map[string]string{
		validate.CodeRequired:       "campo obrigatório ausente no leiaute",
		validate.CodeChoiceConflict: "só uma das alternativas do leiaute pode ser informada",
		validate.CodeChoiceMissing:  "falta informar uma das alternativas do leiaute",
		validate.CodeMaxOccurs:      "repetido mais vezes do que o leiaute permite",
		validate.CodePattern:        "valor fora do formato do leiaute",
		validate.CodeEnum:           "valor fora da lista permitida pelo leiaute",
		validate.CodeLength:         "tamanho diferente do exigido pelo leiaute",
	}
	reason := reasons[v.Code]
	if reason == "" {
		reason = "fora do leiaute"
	}
	return problem.Field(v.Path, FieldCodeSchema, fmt.Sprintf("%s: %s", v.Path, reason),
		"xml_path", v.Path, "reason", v.Code, "limit", v.Limit, "got", v.Got)
}
