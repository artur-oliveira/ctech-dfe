// Package dfecheck liga go-dfe/validate aos fluxos de emissão. Shadow valida o documento que o builder produziu
// (schema do XSD + regras do MOC) e só registra no log: nunca devolve erro nem bloqueia. Gate é o portão de
// recusa: devolve só os achados que o chamador promoveu a block (regras escolhidas por id e os achados
// determinísticos do schema, por tipo de documento: SchemaMode). Nos dois, um defeito no validador falha aberto e não derruba a emissão.
package dfecheck

import (
	"log/slog"
	"os"
	"strings"

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
	// FieldCodeSchema: achado de schema que não é de tamanho (padrão, enum, obrigatório, choice, ocorrência);
	// params.reason traz o código do validador.
	FieldCodeSchema = "schema_invalid"
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

// Kind é a granularidade da promoção do schema a block: cada documento e cada família de evento tem o seu modo,
// para recuar um só sem derrubar os outros.
type Kind string

const (
	KindNFe       Kind = "nfe"
	KindNFCe      Kind = "nfce"
	KindCTe       Kind = "cte"
	KindCTeOS     Kind = "cteos"
	KindMDFe      Kind = "mdfe"
	KindNFeEvent  Kind = "nfe_event"
	KindCTeEvent  Kind = "cte_event"
	KindMDFeEvent Kind = "mdfe_event"
)

// schemaDefaults é a tabela de promoção: o modo de schema de cada tipo quando nenhuma variável de ambiente o
// muda. Decisão do dono (2026-11-09): todos em shadow no primeiro rollout (os achados vão só para o log); um tipo
// sobe para block pela variável de ambiente, ou aqui, depois de observar o log em homologação.
var schemaDefaults = map[Kind]validate.Mode{
	KindNFe: validate.ModeShadow, KindNFCe: validate.ModeShadow, KindCTe: validate.ModeShadow,
	KindCTeOS: validate.ModeShadow, KindMDFe: validate.ModeShadow,
	KindNFeEvent: validate.ModeShadow, KindCTeEvent: validate.ModeShadow, KindMDFeEvent: validate.ModeShadow,
}

// Variáveis de ambiente do interruptor: DFE_GATE_SCHEMA_<TIPO> (por exemplo DFE_GATE_SCHEMA_NFCE) vale para um
// tipo; DFE_GATE_SCHEMA vale para os que não têm a própria. Valores: off, shadow, warn, block.
const envSchemaPrefix = "DFE_GATE_SCHEMA"

// SchemaMode devolve o modo de schema do Gate para k: a variável do tipo, a geral ou o padrão da tabela. Valor
// desconhecido é ignorado (cai no padrão): um erro de digitação não desliga nem liga o portão por acidente.
func SchemaMode(k Kind) validate.Mode {
	for _, name := range []string{envSchemaPrefix + "_" + strings.ToUpper(string(k)), envSchemaPrefix} {
		switch m := validate.Mode(strings.ToLower(strings.TrimSpace(os.Getenv(name)))); m {
		case validate.ModeOff, validate.ModeShadow, validate.ModeWarn, validate.ModeBlock:
			return m
		}
	}
	if m, ok := schemaDefaults[k]; ok {
		return m
	}
	return validate.ModeShadow
}

// sizeCodes são os achados de tamanho que o modo warn recusa (a política anterior ao schema em block): tamanho de
// texto fora do leiaute vira rejeição 225 na SEFAZ, e o valor nunca é truncado em silêncio (CONDUCT.md).
var sizeCodes = map[string]bool{validate.CodeMinLength: true, validate.CodeMaxLength: true}

// blockCodes são os achados de schema determinísticos que o modo block recusa: a SEFAZ os rejeita com 225 (schema
// XML inválido) sem depender de tabela, relógio ou cadastro. Ficam de fora unknown_tag (aviso; o overlay das NTs
// pode estar atrás do XSD), attr_required, total_digits, fraction_digits, min/max_inclusive (os padrões TDec do
// XSD já cobrem casas e faixa) e todas as regras de negócio, que têm o seu próprio modo por id.
var blockCodes = map[string]bool{
	validate.CodeRequired: true, validate.CodeChoiceConflict: true, validate.CodeChoiceMissing: true,
	validate.CodeMaxOccurs: true, validate.CodePattern: true, validate.CodeEnum: true, validate.CodeLength: true,
	validate.CodeMinLength: true, validate.CodeMaxLength: true,
}

// blocks diz se o achado de schema code recusa a emissão no modo mode.
func blocks(mode validate.Mode, code string) bool {
	switch mode {
	case validate.ModeBlock:
		return blockCodes[code]
	case validate.ModeWarn, validate.ModeShadow:
		return sizeCodes[code] // shadow e warn mantêm só a recusa de tamanho que já existia; o resto vai ao log
	}
	return false // off: o portão de schema não recusa nada
}

// Gate devolve os achados que recusam a emissão: as regras que block promove a validate.ModeBlock (só elas; o
// modo padrão do catálogo não recusa nada aqui) e os achados de schema que o modo schema recusa (blocks). Falha
// aberta: esquema indisponível ou pânico no validador registram o erro e não recusam nada.
func Gate(spec, docKind string, doc map[string]any, block validate.Modes, schema validate.Mode) (found []validate.Violation) {
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
		if blocks(schema, v.Code) {
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
	return gateEnviNFe(body, block, KindNFe)
}

// GateEnviNFCe é o Gate do NFe de um enviNFe da NFC-e (modelo 65): o mesmo leiaute, com o modo de schema próprio.
func GateEnviNFCe(body map[string]any, block validate.Modes) []validate.Violation {
	return gateEnviNFe(body, block, KindNFCe)
}

func gateEnviNFe(body map[string]any, block validate.Modes, kind Kind) []validate.Violation {
	env, _ := body["enviNFe"].(map[string]any)
	nfe, ok := env["NFe"].(map[string]any)
	if !ok {
		return nil
	}
	return Gate(SpecNFe, "nfe", nfe, block, SchemaMode(kind))
}

// GateMDFe é o Gate do MDFe do corpo do builder: o documento (regras do MOC e tamanhos) e o conteúdo do modal
// (infModal é xs:any no XSD; só os tamanhos, pois as regras do MDF-e já leem o modal pelo documento).
func GateMDFe(body map[string]any, block validate.Modes) []validate.Violation {
	mdfe, ok := body["MDFe"].(map[string]any)
	if !ok {
		return nil
	}
	mode := SchemaMode(KindMDFe)
	found := Gate(SpecMDFe, "mdfe", mdfe, block, mode)
	inf, _ := mdfe["infMDFe"].(map[string]any)
	modal, _ := inf["infModal"].(map[string]any)
	for _, m := range []string{"rodo", "aereo", "aquav", "ferrov"} {
		if node, ok := modal[m].(map[string]any); ok {
			found = append(found, Gate(SpecMDFe+"_"+m, "mdfe_"+m, node, nil, mode)...)
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
	mode := SchemaMode(KindCTe)
	found := Gate(SpecCTe, "cte", cte, block, mode)
	inf, _ := cte["infCte"].(map[string]any)
	norm, _ := inf["infCTeNorm"].(map[string]any)
	modal, _ := norm["infModal"].(map[string]any)
	if rodo, ok := modal["rodo"].(map[string]any); ok {
		found = append(found, Gate(SpecCTe+"_rodo", "cte_rodo", rodo, nil, mode)...)
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

// Esquemas do CT-e OS (modelo 67) e do seu modal rodoviário. Não há catálogo de regras do MOC para o CT-e OS
// (H001 a H133 não estão em go-dfe/validate): o shadow e o portão valem só o esquema (cteOS_v4.00.xsd).
const (
	SpecCTeOS     = "cteos"
	SpecCTeRodoOS = "cte_rodoos"
)

// CTeOSModal separa o CTeOS e o rodoOS do corpo do builder.
func CTeOSModal(body map[string]any) (cte, rodoOS map[string]any) {
	cte, _ = body["CTeOS"].(map[string]any)
	inf, _ := cte["infCte"].(map[string]any)
	norm, _ := inf["infCTeNorm"].(map[string]any)
	modal, _ := norm["infModal"].(map[string]any)
	rodoOS, _ = modal["rodoOS"].(map[string]any)
	return cte, rodoOS
}

// ShadowCTeOS valida o CTeOS do corpo do builder e o rodoOS (infModal é xs:any no XSD).
func ShadowCTeOS(accessKey string, body map[string]any) {
	cte, rodo := CTeOSModal(body)
	if cte == nil {
		return
	}
	Shadow(SpecCTeOS, "cteos", accessKey, cte)
	if rodo != nil {
		Shadow(SpecCTeRodoOS, "cte_rodoos", accessKey, rodo)
	}
}

// GateCTeOS é o Gate do CTeOS: só os tamanhos do leiaute (sem regras block, ver acima).
func GateCTeOS(body map[string]any) []validate.Violation {
	cte, rodo := CTeOSModal(body)
	if cte == nil {
		return nil
	}
	mode := SchemaMode(KindCTeOS)
	found := Gate(SpecCTeOS, "cteos", cte, nil, mode)
	if rodo != nil {
		found = append(found, Gate(SpecCTeRodoOS, "cte_rodoos", rodo, nil, mode)...)
	}
	return found
}
