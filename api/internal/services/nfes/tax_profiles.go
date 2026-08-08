package nfes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// loadTaxProfiles carrega num único BatchGetItem todos os perfis referenciados
// pelos produtos de uma emissão. Um id referenciado que não existe é erro de
// cadastro, não silêncio: emitir com a tributação faltando produziria um XML
// errado que só a SEFAZ recusaria.
func loadTaxProfiles(
	ctx context.Context, repo *repositories.TaxProfileRepository, orgPK string, ids []string,
) (map[string]map[string]any, error) {
	out := map[string]map[string]any{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := repo.BatchGet(ctx, orgPK, ids)
	if err != nil {
		return nil, err
	}
	for id, row := range rows {
		var m map[string]any
		if err := attributevalue.UnmarshalMap(row, &m); err != nil {
			return nil, problem.InternalServer("failed to decode tax profile " + id)
		}
		out[id] = m
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			return nil, problem.BadRequest("perfil fiscal não encontrado: " + id)
		}
	}
	return out, nil
}

// Chaves do vínculo produto → perfil fiscal (ProductTaxProfileRef no dto).
const (
	// productTaxProfilesField é a lista de vínculos no item do produto.
	productTaxProfilesField = "tax_profiles"
	// taxProfileIDField é o id do perfil dentro de um vínculo.
	taxProfileIDField = "tax_profile_id"
	// taxProfileOverridesField são os campos que o produto sobrescreve do perfil.
	taxProfileOverridesField = "overrides"
	// profileCfopsField é a lista de CFOPs cobertos por um perfil.
	profileCfopsField = "cfops"
	// cfopField é o CFOP dentro de uma entrada de cfop_config.
	cfopField = "cfop"
	// cfopConfigField é a lista de tributação por CFOP no item do produto.
	cfopConfigField = "cfop_config"
)

// profileRefs devolve os ids dos perfis referenciados por um produto, na ordem
// em que foram declarados — a ordem é o critério de desempate quando mais de um
// perfil cobre o mesmo CFOP.
func profileRefs(product map[string]any) []string {
	refs, ok := product[productTaxProfilesField].([]any)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := m[taxProfileIDField].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// productCFOPs devolve o conjunto de CFOPs válidos para o produto: a união dos
// que estão em cfop_config[] com os cobertos pelos perfis referenciados.
func productCFOPs(product map[string]any, profiles map[string]map[string]any) []string {
	set := map[string]struct{}{}
	if entries, ok := product[cfopConfigField].([]any); ok {
		for _, e := range entries {
			if m, ok := e.(map[string]any); ok {
				if c, ok := m[cfopField].(string); ok {
					set[c] = struct{}{}
				}
			}
		}
	}
	for _, id := range profileRefs(product) {
		for _, c := range profileCFOPs(profiles[id]) {
			set[c] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func profileCFOPs(profile map[string]any) []string {
	raw, ok := profile[profileCfopsField].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if c, ok := v.(string); ok {
			out = append(out, c)
		}
	}
	return out
}

// resolveCfopTax devolve o tratamento tributário efetivo de um produto para um
// CFOP, na ordem de precedência da spec §3.2 — da menor para a maior:
//
//	perfil  →  overrides do produto sobre o perfil  →  cfop_config[cfop]
//
// Overrides de nível de produto (icms_aliq_override, fcp_aliq_override) e a
// tabela da UF ficam fora daqui: são campos do próprio item, aplicados adiante
// por resolveICMSAliq/resolveFCPAliq.
//
// Produto legado, sem perfil nenhum: o resultado é exatamente a entrada de
// cfop_config que já era usada hoje. É essa propriedade que garante que nenhuma
// emissão existente muda de comportamento.
func resolveCfopTax(product map[string]any, profiles map[string]map[string]any, cfop string) (map[string]any, error) {
	resolved := map[string]any{}

	// 1. Perfil cujo conjunto de CFOPs cobre este CFOP. O primeiro declarado
	//    vence — dois perfis cobrindo o mesmo CFOP é configuração ambígua, e a
	//    ordem de declaração é o desempate previsível.
	for _, id := range profileRefs(product) {
		profile, ok := profiles[id]
		if !ok {
			return nil, fmt.Errorf("perfil fiscal não encontrado: %s", id)
		}
		if !containsCFOP(profileCFOPs(profile), cfop) {
			continue
		}
		mergeTaxFields(resolved, profile)
		// 2. Override do produto sobre esse perfil.
		mergeTaxFields(resolved, profileOverrides(product, id))
		break
	}

	// 3. cfop_config explícito no produto — vence tudo.
	if entries, ok := product[cfopConfigField].([]any); ok {
		for _, e := range entries {
			m, ok := e.(map[string]any)
			if !ok || m[cfopField] != cfop {
				continue
			}
			mergeTaxFields(resolved, m)
			break
		}
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("CFOP %s sem tributação configurada", cfop)
	}
	resolved[cfopField] = cfop
	return resolved, nil
}

// profileOverrides devolve o bloco `overrides` que o produto declarou para um
// perfil específico.
func profileOverrides(product map[string]any, profileID string) map[string]any {
	refs, ok := product[productTaxProfilesField].([]any)
	if !ok {
		return nil
	}
	for _, r := range refs {
		m, ok := r.(map[string]any)
		if !ok || m[taxProfileIDField] != profileID {
			continue
		}
		if ov, ok := m[taxProfileOverridesField].(map[string]any); ok {
			return ov
		}
		return nil
	}
	return nil
}

// mergeTaxFields aplica sobre dst as chaves de tributação de src que carregam
// valor. Chave ausente, nula ou string vazia não sobrescreve — é assim que um
// override parcial altera só o que nomeia, em vez de zerar o resto.
//
// Chaves de identidade do cadastro (pk/sk/name/…) nunca entram no resultado: o
// que sai daqui alimenta o XML, não a tela do cadastro.
func mergeTaxFields(dst, src map[string]any) {
	for k, v := range src {
		if nonTaxField(k) || v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		dst[k] = v
	}
}

// nonTaxFields são atributos de cadastro que não são tributação e por isso não
// podem vazar do perfil para o item da emissão.
var nonTaxFields = map[string]struct{}{
	"pk": {}, "sk": {}, "name": {}, "description": {},
	profileCfopsField: {}, cfopField: {},
	"created_at": {}, "updated_at": {},
}

func nonTaxField(k string) bool {
	_, ok := nonTaxFields[k]
	return ok
}

func containsCFOP(list []string, cfop string) bool {
	for _, c := range list {
		if c == cfop {
			return true
		}
	}
	return false
}

// cfopNotConfiguredError monta a mensagem de CFOP inválido dizendo onde
// configurar — a mensagem antiga só dizia que estava faltando.
func cfopNotConfiguredError(cfop, productCode string, valid []string) string {
	msg := fmt.Sprintf("CFOP %s não configurado para o produto %s", cfop, productCode)
	if len(valid) == 0 {
		return msg + " — configure a tributação no produto (cfop_config) ou vincule um perfil fiscal"
	}
	return msg + fmt.Sprintf(" — configurados: %s. Adicione o CFOP ao cfop_config do produto ou a um perfil fiscal vinculado",
		strings.Join(valid, ", "))
}
