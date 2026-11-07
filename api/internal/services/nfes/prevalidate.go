package nfes

// prevalidate.go: o que a emissão de NF-e e NFC-e recusa localmente (422) em vez de deixar a SEFAZ rejeitar.
// Três famílias: cadastro incompleto (município, UF, país), regras do MOC promovidas a block no catálogo de
// go-dfe/validate (uma implementação só da regra, a mesma que o shadow registra) e tamanho de texto fora do
// leiaute (o valor nunca é truncado em silêncio).

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// Codes (problem.code) e códigos de campo (errors[].code) que a UI mapeia.
const (
	// CodeRegistrationIncomplete: cadastro sem um dado que o leiaute exige (município IBGE, UF, país do
	// endereço no exterior, município do ISSQN). errors[] nomeia a entidade e o campo.
	CodeRegistrationIncomplete = "registration_incomplete"
	// CodeFiscalPrevalidation: regra do MOC ou tamanho do leiaute violado no documento montado.
	CodeFiscalPrevalidation = "fiscal_prevalidation"

	fieldCodeRequired   = "required"
	fieldCodeTooLong    = "too_long"
	fieldCodeTooShort   = "too_short"
	fieldCodeFiscalRule = "fiscal_rule"

	// Entidades nomeadas nos params dos erros (errors[].params.entity).
	entityEmitente     = "emitente"
	entityDestinatario = "destinatario"
	entityProduto      = "produto"
	entityRetirada     = "retirada"
	entityEntrega      = "entrega"
	entityTransporta   = "transportadora"
	entityDocumento    = "documento"
)

// blockRules são as regras do MOC que a emissão recusa antes da SEFAZ, por id do catálogo de go-dfe/validate
// (o modelo de cada regra já está na própria regra). Só regras "Obrig." da SEFAZ entram: uma regra facultativa
// (a critério da UF, como E14-10, E14-20 e I08-60) continua só no shadow.
var blockRules = validate.Modes{
	// Destinatário no exterior (E03a, E10, E12, E14, E16a, E17) e coerência do escopo do CFOP com o idDest
	// (I08-30, I08-40 e I08-50: CFOP 7xxx só com idDest 3, 6xxx só com 2, 5xxx só com 1).
	"E03a-10": validate.ModeBlock, "E03a-20": validate.ModeBlock, "E03a-30": validate.ModeBlock,
	"E03a-60": validate.ModeBlock, "E10-30": validate.ModeBlock, "E12-10": validate.ModeBlock,
	"E14-30": validate.ModeBlock, "E16a-20": validate.ModeBlock, "E17-40": validate.ModeBlock,
	"I08-30": validate.ModeBlock, "I08-40": validate.ModeBlock, "I08-50": validate.ModeBlock,
	// Locais de retirada e entrega no exterior.
	"F07-10": validate.ModeBlock, "G07-10": validate.ModeBlock,
	// Indicador de IE do destinatário (isento).
	"E16a-30": validate.ModeBlock, "E17-30": validate.ModeBlock,
	// Nota de crédito (finNFe 5) e de débito (finNFe 6): finalidade x tipo x referências x grupos de imposto.
	"B25-30": validate.ModeBlock, "B25-65": validate.ModeBlock, "B25-80": validate.ModeBlock,
	"B25-100": validate.ModeBlock, "B25-110": validate.ModeBlock, "B25-120": validate.ModeBlock,
	"B25.1-10": validate.ModeBlock, "B25.1-20": validate.ModeBlock, "B25.2-10": validate.ModeBlock,
	"B25.2-20": validate.ModeBlock, "B25.2-30": validate.ModeBlock, "B25.2-40": validate.ModeBlock,
	// Pagamento x total: pago a mais sem troco (866), troco errado (869), tPag 90/91 com valor (904).
	"YA03-20": validate.ModeBlock, "YA09-10": validate.ModeBlock, "YA03-30": validate.ModeBlock,
}

// blockRulesNFCe acrescenta o pago a menos (YA03-10, 865), que o MOC marca como "implementação futura para o
// modelo 55": na NF-e ele fica só no shadow para não recusar o que a SEFAZ ainda aceita.
var blockRulesNFCe = func() validate.Modes {
	m := validate.Modes{"YA03-10": validate.ModeBlock}
	for k, v := range blockRules {
		m[k] = v
	}
	return m
}()

// gateEnviNFe recusa o enviNFe com regra bloqueante ou tamanho fora do leiaute, nomeando cada achado.
func gateEnviNFe(enviNFe map[string]any, model string) error {
	block := blockRules
	if model == nfModel65 {
		block = blockRulesNFCe
	}
	found := dfecheck.GateEnviNFe(enviNFe, block)
	if len(found) == 0 {
		return nil
	}
	errs := make([]problem.FieldError, 0, len(found))
	for _, v := range found {
		if v.Rule != "" {
			errs = append(errs, problem.Field(v.Rule, fieldCodeFiscalRule,
				fmt.Sprintf("regra %s do MOC (rejeição %d): %s", v.Rule, v.CStat, v.Message),
				"rule", v.Rule, "cstat", v.CStat))
			continue
		}
		entity, field, item := lengthTarget(v.Path)
		code, what := fieldCodeTooLong, "no máximo"
		if v.Code == validate.CodeMinLength {
			code, what = fieldCodeTooShort, "ao menos"
		}
		label := entity
		if item > 0 {
			label = fmt.Sprintf("%s do item %d", entity, item)
		}
		params := []any{"entity", entity, "xml_path", v.Path, "limit", v.Limit, "got", v.Got}
		if item > 0 {
			params = append(params, "item", item)
		}
		errs = append(errs, problem.Field(field, code,
			fmt.Sprintf("%s: %s tem %d caracteres; o leiaute aceita %s %d", label, field, v.Got, what, v.Limit),
			params...))
	}
	return problem.UnprocessableEntityFields(
		"a nota não passa nas validações do leiaute e do MOC; corrija os itens apontados antes de emitir", errs...,
	).WithCode(CodeFiscalPrevalidation)
}

var (
	xmlIndex = regexp.MustCompile(`\[(\d+)\]`)
	// enderFields traduz a tag de TEndereco/TLocal para o campo do endereço no cadastro.
	enderFields = map[string]string{
		"xLgr": "street", "nro": "number", "xCpl": "complement", "xBairro": "neighborhood",
		"xMun": "city", "fone": "contacts.phones", "xPais": addrCountryName,
	}
	// partyFields traduz as tags diretas de emit/dest/transporta para o campo do cadastro.
	partyFields = map[string]string{
		"xNome": "name", "xFant": "person.fantasy_name", "IE": "person.state_registrations.state_registration",
		"email": "person.contacts.emails",
	}
	// localFields traduz as tags de retirada/entrega para o campo do pedido (NfeLocalBody).
	localFields = map[string]string{
		"xNome": "x_nome", "xLgr": "x_lgr", "nro": "nro", "xCpl": "x_cpl", "xBairro": "x_bairro",
		"xMun": "x_mun", "xPais": "x_pais", "email": "email", "IE": "ie",
	}
	prodFields = map[string]string{"xProd": "description", "cProd": "code", "infAdProd": "inf_ad_prod"}
)

// lengthTarget traduz o caminho do achado (por exemplo "NFe/infNFe/det[2]/prod/xProd") na entidade, no campo do
// cadastro ou do pedido a corrigir e no número do item (base 1; 0 fora de det).
func lengthTarget(path string) (entity, field string, item int) {
	p := strings.TrimPrefix(path, validateRoot)
	if m := xmlIndex.FindStringSubmatch(p); m != nil && strings.HasPrefix(p, "infNFe/det[") {
		n, _ := strconv.Atoi(m[1])
		item = n + 1
	} else if strings.HasPrefix(p, "infNFe/det/") {
		item = 1
	}
	p = xmlIndex.ReplaceAllString(p, "")
	parts := strings.Split(p, "/")
	tag := parts[len(parts)-1]
	switch {
	case len(parts) >= 3 && parts[1] == "emit":
		return entityEmitente, partyOrEnder(parts, tag), 0
	case len(parts) >= 3 && parts[1] == "dest":
		return entityDestinatario, partyOrEnder(parts, tag), 0
	case len(parts) >= 3 && parts[1] == "transp" && parts[2] == "transporta":
		return entityTransporta, partyOrEnder(parts, tag), 0
	case len(parts) == 3 && (parts[1] == "retirada" || parts[1] == "entrega"):
		if f, ok := localFields[tag]; ok {
			return parts[1], parts[1] + "." + f, 0
		}
	case len(parts) >= 3 && parts[1] == "det":
		if f, ok := prodFields[tag]; ok {
			return entityProduto, f, item
		}
	}
	return entityDocumento, p, item
}

// validateRoot é a raiz dos caminhos do validador da NF-e.
const validateRoot = "NFe/"

func partyOrEnder(parts []string, tag string) string {
	if len(parts) == 4 { // emit/enderEmit/<tag>, dest/enderDest/<tag>
		if f, ok := enderFields[tag]; ok {
			return "person.addresses[0]." + f
		}
	}
	if f, ok := partyFields[tag]; ok {
		return f
	}
	return strings.Join(parts[1:], "/")
}

// requireAddress recusa o cadastro cujo endereço o leiaute não aceita: sem UF, sem município IBGE no Brasil
// (rejeições 272 e 274 da SEFAZ, antes vindas de um "0000000" inventado) ou, no exterior, sem o país.
func requireAddress(entity string, person map[string]any) error {
	addr := services.FirstAddress(person)
	const prefix = "person.addresses[0]."
	var errs []problem.FieldError
	missing := func(field, what string) {
		errs = append(errs, problem.Field(prefix+field, fieldCodeRequired,
			fmt.Sprintf("%s sem %s no endereço", entity, what), "entity", entity))
	}
	switch uf := anyStr(addr, "state_federation", ""); {
	case uf == "":
		missing("state_federation", "UF")
	case uf == ufExterior:
		if anyStr(addr, addrCountryCode, "") == "" {
			missing(addrCountryCode, "código do país (tabela BACEN)")
		}
		if anyStr(addr, addrCountryName, "") == "" {
			missing(addrCountryName, "nome do país")
		}
	case anyStr(addr, "city_ibge_code", "") == "":
		missing("city_ibge_code", "código IBGE do município")
	}
	if len(errs) == 0 {
		return nil
	}
	return problem.UnprocessableEntityFields(
		fmt.Sprintf("complete o cadastro do %s antes de emitir", entity), errs...,
	).WithCode(CodeRegistrationIncomplete)
}

// requireLocalCountry recusa o local de retirada/entrega no exterior sem país (F/G: cPais e xPais).
func requireLocalCountry(entity string, l *NfeLocalBody) error {
	if l == nil || l.UF != ufExterior {
		return nil
	}
	var errs []problem.FieldError
	if ptrStr(l.CPais) == "" {
		errs = append(errs, problem.Field(entity+".c_pais", fieldCodeRequired, "local no exterior sem código do país", "entity", entity))
	}
	if ptrStr(l.XPais) == "" {
		errs = append(errs, problem.Field(entity+".x_pais", fieldCodeRequired, "local no exterior sem nome do país", "entity", entity))
	}
	if len(errs) == 0 {
		return nil
	}
	return problem.UnprocessableEntityFields("informe o país do local de "+entity+" no exterior", errs...).
		WithCode(CodeRegistrationIncomplete)
}

// requireISSQNMunFG recusa o item de serviço sem o município do fato gerador do ISSQN (U14, obrigatório), antes
// preenchido com um "0000000" que a SEFAZ rejeita.
func requireISSQNMunFG(productItems []map[string]any) error {
	var errs []problem.FieldError
	for i, item := range productItems {
		cfg := findCFOPEntry(getCFOPConfig(item), anyStr(item, "cfop", ""))
		if cfgStr(cfg, "issqn_ind_iss", "") != "" && cfgStr(cfg, "issqn_c_mun_fg", "") == "" {
			errs = append(errs, problem.Field("cfop_config.issqn_c_mun_fg", fieldCodeRequired,
				fmt.Sprintf("item %d (%s): serviço sem o município do fato gerador do ISSQN", i+1, anyStr(item, "product_code", "")),
				"entity", entityProduto, "item", i+1))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return problem.UnprocessableEntityFields("informe o município do fato gerador do ISSQN na tributação do produto", errs...).
		WithCode(CodeRegistrationIncomplete)
}
