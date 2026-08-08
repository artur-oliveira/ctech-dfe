package services

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Placeholders aceitos em inf_ad_fisco / inf_cpl de uma natureza de operação.
// O mapa é **fechado**: uma chave desconhecida é erro de validação no cadastro
// da operação, nunca uma interpolação silenciosa em branco no XML — um texto
// fiscal com um buraco no meio é rejeição da SEFAZ ou, pior, uma nota emitida
// com informação faltando.
const (
	PlaceholderVNF        = "v_nf"
	PlaceholderVICMSST    = "v_icms_st"
	PlaceholderCliente    = "cliente"
	PlaceholderNatOp      = "nat_op"
	PlaceholderCompetenci = "competencia"
)

// AllPlaceholders é a fonte única das chaves aceitas — validação, documentação
// e UI derivam daqui.
var AllPlaceholders = []string{
	PlaceholderVNF,
	PlaceholderVICMSST,
	PlaceholderCliente,
	PlaceholderNatOp,
	PlaceholderCompetenci,
}

// placeholderRe casa `{{chave}}`, com espaços opcionais em volta da chave.
var placeholderRe = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

// ValidatePlaceholders reporta a primeira chave desconhecida do template.
// Chamada no cadastro da operação, não na emissão: o erro tem que aparecer para
// quem escreveu o texto, não para quem emitiu a nota três semanas depois.
func ValidatePlaceholders(template string) error {
	known := make(map[string]struct{}, len(AllPlaceholders))
	for _, k := range AllPlaceholders {
		known[k] = struct{}{}
	}
	for _, m := range placeholderRe.FindAllStringSubmatch(template, -1) {
		if _, ok := known[m[1]]; !ok {
			valid := append([]string(nil), AllPlaceholders...)
			sort.Strings(valid)
			return problem.BadRequest(fmt.Sprintf(
				"placeholder desconhecido: {{%s}} — disponíveis: {{%s}}",
				m[1], strings.Join(valid, "}}, {{")))
		}
	}
	return nil
}

// Interpolate substitui os placeholders de template pelos valores de vars.
// Uma chave conhecida sem valor em vars vira string vazia: o cadastro já
// garantiu que a chave existe, e um valor ausente na emissão (ICMS ST zerado,
// por exemplo) é ausência legítima, não erro.
func Interpolate(template string, vars map[string]string) (string, error) {
	if err := ValidatePlaceholders(template); err != nil {
		return "", err
	}
	return placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
		key := placeholderRe.FindStringSubmatch(match)[1]
		return vars[key]
	}), nil
}
