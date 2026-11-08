package dfecheck

import (
	"log/slog"
	"strings"

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
