package nfses

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// docPunct remove a pontuação do CPF/CNPJ gravado no cadastro — o DPS leva só
// os dígitos (mesma normalização de services.BuildPersonSK).
var docPunct = strings.NewReplacer(".", "", "-", "", "/", "")

// Códigos do leiaute (tiposSimples_v1.01.xsd) usados nas validações abaixo.
const (
	tpEmitPrestador   = 1
	opSimpNacApuracao = 3 // exige regApTribSN
	appVersion        = "ctech-dfe-1.0"
)

// documentInput reúne tudo que a montagem precisa. Os map[string]AttributeValue
// vêm dos repositórios sem conversão intermediária, seguindo o padrão de
// NfeService.Emit.
type documentInput struct {
	Org           map[string]types.AttributeValue
	Config        map[string]types.AttributeValue
	Prestador     map[string]types.AttributeValue
	Tomador       map[string]types.AttributeValue
	Intermediario map[string]types.AttributeValue
	Service       map[string]types.AttributeValue
	Body          NfseEmitBody
	Serie         string
	Numero        int
	Environment   int
}

// buildDocument converte cadastro + catálogo + body no modelo neutro do
// go-dfe. Toda regra condicional que depende de contexto de emissão mora
// aqui — o go-dfe só valida estrutura.
func buildDocument(in documentInput) (nfse.Document, error) {
	if in.Body.TpEmit != tpEmitPrestador && in.Body.MotivoEmisTI == 0 {
		return nfse.Document{}, problem.BadRequest(
			"motivo_emis_ti é obrigatório quando a emissão não é do prestador")
	}

	prest, err := buildPrestador(in.Prestador)
	if err != nil {
		return nfse.Document{}, err
	}

	cLocEmi := strAttr(in.Config, "c_loc_emi")

	doc := nfse.Document{
		Ambiente: in.Environment, VerAplic: appVersion,
		TpEmit: in.Body.TpEmit, MotivoEmisTI: in.Body.MotivoEmisTI,
		ChNFSeRej:   in.Body.ChNFSeRej,
		Competencia: in.Body.Competence,
		Serie:       in.Serie, Numero: in.Numero,
		CLocEmi:       cLocEmi,
		Prestador:     prest,
		Tomador:       buildPessoa(in.Tomador),
		Intermediario: buildPessoa(in.Intermediario),
		Servico:       buildServico(in.Service, in.Body),
		Valores:       buildValores(in.Service, in.Body),
		IBSCBS:        buildIBSCBS(in.Service),
	}
	// LocPrest.CLocPrestacao é o município de prestação do serviço; por
	// padrão é o mesmo município de emissão (config.c_loc_emi) — buildServico
	// não recebe a config, então o valor é aplicado aqui.
	doc.Servico.LocPrest.CLocPrestacao = cLocEmi

	if in.Body.AdditionalInfo != nil {
		doc.Servico.InfoCompl = &nfse.InfoCompl{XInfComp: *in.Body.AdditionalInfo}
	}

	if in.Body.SubstitutesAccessKey != nil {
		motivo := ""
		if in.Body.SubstitutesReason != nil {
			motivo = *in.Body.SubstitutesReason
		}
		doc.Substituicao = &nfse.Substituicao{
			ChSubstda: *in.Body.SubstitutesAccessKey, CMotivo: motivo,
		}
	}
	return doc, nil
}

// buildPrestador extrai identidade e regime tributário do item de cadastro.
// O grupo nfse é o mesmo em organizations e organization_persons (spec §3.2),
// então esta função serve aos dois casos sem ramificação.
func buildPrestador(item map[string]types.AttributeValue) (nfse.Prestador, error) {
	if item == nil {
		return nfse.Prestador{}, problem.BadRequest("prestador não encontrado")
	}
	grupo := nfseGroup(item)
	regTribItem := mapAttr(grupo, "reg_trib")
	if regTribItem == nil {
		return nfse.Prestador{}, problem.BadRequest(
			"o prestador não tem regime tributário de NFS-e cadastrado (grupo nfse.reg_trib)")
	}

	reg := nfse.RegTrib{
		OpSimpNac:   intAttr(regTribItem, "op_simp_nac", 0),
		RegApTribSN: intAttr(regTribItem, "reg_ap_trib_sn", 0),
		RegEspTrib:  intAttr(regTribItem, "reg_esp_trib", 0),
	}
	if reg.OpSimpNac == opSimpNacApuracao && reg.RegApTribSN == 0 {
		return nfse.Prestador{}, problem.BadRequest(
			"reg_ap_trib_sn é obrigatório quando op_simp_nac = 3")
	}

	p := basePessoa(item, grupo)
	return nfse.Prestador{Pessoa: p, RegTrib: reg}, nil
}

func buildPessoa(item map[string]types.AttributeValue) *nfse.Pessoa {
	if item == nil {
		return nil
	}
	return new(basePessoa(item, nfseGroup(item)))
}

// nfseGroup devolve o grupo `nfse` do cadastro. Organizations e
// organization_persons gravam o DTO como veio da API, então identidade,
// endereços e o grupo nfse ficam aninhados em `person` (PersonObjectBody em
// internal/api/v1/dto.go) — nunca na raiz do item.
func nfseGroup(item map[string]types.AttributeValue) map[string]types.AttributeValue {
	return mapAttr(mapAttr(item, "person"), "nfse")
}

// basePessoa mapeia identidade + endereço. Os campos de NFS-e (IM, CAEPF,
// NIF, cNaoNIF, endereço no exterior) vêm do grupo nfse adicionado na F1;
// nome, documento e endereço nacional vêm dos campos já existentes.
func basePessoa(item, grupo map[string]types.AttributeValue) nfse.Pessoa {
	person := mapAttr(item, "person")
	doc := personDoc(item)
	p := nfse.Pessoa{
		XNome:   strAttr(item, "name"),
		IM:      strAttr(grupo, "im"),
		CAEPF:   strAttr(grupo, "caepf"),
		NIF:     strAttr(grupo, "nif"),
		CNaoNIF: intPtrAttr(grupo, "c_nao_nif"),
		Fone:    firstContact(person, "phones"),
		Email:   firstContact(person, "emails"),
	}
	if len(doc) == lenCNPJ {
		p.CNPJ = doc
	} else if len(doc) == lenCPF {
		p.CPF = doc
	}
	p.End = buildEndereco(item, grupo)
	return p
}

// buildEndereco prefere o endereço no exterior (grupo nfse.foreign_address)
// quando presente; caso contrário usa o primeiro item de addresses do
// cadastro (endereço nacional).
func buildEndereco(item, grupo map[string]types.AttributeValue) *nfse.Endereco {
	if foreign := mapAttr(grupo, "foreign_address"); foreign != nil {
		return &nfse.Endereco{
			CPais:       strAttr(foreign, "c_pais"),
			CEndPost:    strAttr(foreign, "c_end_post"),
			XCidade:     strAttr(foreign, "x_cidade"),
			XEstadoProv: strAttr(foreign, "x_estado_prov"),
			XLgr:        strAttr(foreign, "x_lgr"),
			Nro:         strAttr(foreign, "nro"),
			XCpl:        strAttr(foreign, "x_cpl"),
			XBairro:     strAttr(foreign, "x_bairro"),
		}
	}

	addresses := listAttr(mapAttr(item, "person"), "addresses")
	if len(addresses) == 0 {
		return nil
	}
	addr, ok := addresses[0].(*types.AttributeValueMemberM)
	if !ok {
		return nil
	}

	return &nfse.Endereco{
		CMun:    strAttr(addr.Value, "city_ibge_code"),
		CEP:     strAttr(addr.Value, "postal_code"),
		XLgr:    strAttr(addr.Value, "street"),
		Nro:     strAttr(addr.Value, "number"),
		XCpl:    strAttr(addr.Value, "complement"),
		XBairro: strAttr(addr.Value, "neighborhood"),
	}
}

// buildServico lê os defaults do catálogo (trib_nacional_code, nbs_code,
// code, description, trib_municipal_code) e aplica os overrides do item de
// emissão (description, c_trib_mun). LocPrest.CLocPrestacao é preenchido por
// buildDocument, que tem acesso à config.
func buildServico(service map[string]types.AttributeValue, body NfseEmitBody) nfse.Servico {
	cServ := nfse.CServ{
		CTribNac:    strAttr(service, "trib_nacional_code"),
		CTribMun:    strAttr(service, "trib_municipal_code"),
		XDescServ:   strAttr(service, "description"),
		CNBS:        strAttr(service, "nbs_code"),
		CIntContrib: strAttr(service, "code"),
	}
	if body.Service.Description != nil {
		cServ.XDescServ = *body.Service.Description
	}
	if body.Service.CTribMun != nil {
		cServ.CTribMun = *body.Service.CTribMun
	}

	return nfse.Servico{CServ: cServ}
}

// buildValores lê os defaults tributários do catálogo (value, iss, federal,
// ibs_cbs/tot_trib são tratados à parte) e aplica os overrides de valor e
// alíquota do item de emissão.
func buildValores(service map[string]types.AttributeValue, body NfseEmitBody) nfse.Valores {
	vServ := strAttr(service, "value")
	if body.Service.Value != nil {
		vServ = *body.Service.Value
	}

	iss := mapAttr(service, "iss")
	pAliq := strAttr(iss, "tax_rate")
	if body.Service.TaxRate != nil {
		pAliq = *body.Service.TaxRate
	}

	tribMun := nfse.TribMunicipal{
		TribISSQN:  intAttr(iss, "trib_issqn", 0),
		TpRetISSQN: intAttr(iss, "tp_ret_issqn", 0),
		PAliq:      pAliq,
	}
	if exig := mapAttr(service, "exig_susp"); exig != nil {
		tribMun.ExigSusp = &nfse.ExigSusp{
			TpSusp:    intAttr(exig, "tp_susp", 0),
			NProcesso: strAttr(exig, "n_processo"),
		}
	}
	if bm := mapAttr(service, "bm"); bm != nil {
		tribMun.BM = &nfse.BenefMun{
			NBM:      strAttr(bm, "n_bm"),
			VRedBCBM: strAttr(bm, "v_red_bc_bm"),
			PRedBCBM: strAttr(bm, "p_red_bc_bm"),
		}
	}

	var tribFed *nfse.TribFederal
	if fed := mapAttr(service, "federal"); fed != nil {
		tribFed = &nfse.TribFederal{
			CST:            strAttr(fed, "cst_pis_cofins"),
			PAliqPis:       strAttr(fed, "aliq_pis"),
			PAliqCofins:    strAttr(fed, "aliq_cofins"),
			TpRetPisCofins: intPtrAttr(fed, "tp_ret_pis_cofins"),
			VRetCP:         strAttr(fed, "v_ret_cp"),
			VRetIRRF:       strAttr(fed, "v_ret_irrf"),
			VRetCSLL:       strAttr(fed, "v_ret_csll"),
		}
	}

	totTrib := nfse.TotTrib{}
	if tt := mapAttr(service, "tot_trib"); tt != nil {
		totTrib.IndTotTrib = intAttr(tt, "ind_tot_trib", 0)
		totTrib.PTotTribSN = strAttr(tt, "p_tot_trib_sn")
	}

	return nfse.Valores{
		VServPrest: nfse.VServPrest{VServ: vServ},
		Trib: nfse.Tributacao{
			TribMun: tribMun,
			TribFed: tribFed,
			TotTrib: totTrib,
		},
	}
}

// buildIBSCBS devolve nil quando o catálogo não tem o grupo ibs_cbs ou não
// tem c_ind_op — sem indicador de operação não há como montar o grupo (Anexo
// C da reforma tributária).
func buildIBSCBS(service map[string]types.AttributeValue) *nfse.IBSCBS {
	ibs := mapAttr(service, "ibs_cbs")
	cIndOp := strAttr(ibs, "c_ind_op")
	if ibs == nil || cIndOp == "" {
		return nil
	}

	return &nfse.IBSCBS{
		CIndOp:  cIndOp,
		TpOper:  intAttr(ibs, "tp_oper", 0),
		IndDest: intAttr(ibs, "ind_dest", 0),
		FinNFSe: intAttr(ibs, "fin_nfse", 0),
		Valores: nfse.IBSCBSValores{
			Trib: nfse.TribIBSCBS{
				CST:        strAttr(ibs, "cst"),
				CClassTrib: strAttr(ibs, "c_class_trib"),
			},
		},
	}
}

// strAttr extracts a string from a DynamoDB attribute map.
func strAttr(item map[string]types.AttributeValue, key string) string {
	v, ok := item[key].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

// intAttr extracts a number attribute as int, or def if absent/invalid.
func intAttr(item map[string]types.AttributeValue, key string, def int) int {
	v, ok := item[key].(*types.AttributeValueMemberN)
	if !ok {
		return def
	}
	var n int
	_, _ = fmt.Sscanf(v.Value, "%d", &n)
	return n
}

// intPtrAttr extracts a number attribute as *int, or nil if absent — used
// for fields where a legitimate domain value of 0 must be distinguished from
// "not informed" (e.g. Pessoa.CNaoNIF).
func intPtrAttr(item map[string]types.AttributeValue, key string) *int {
	v, ok := item[key].(*types.AttributeValueMemberN)
	if !ok {
		return nil
	}
	var n int
	_, _ = fmt.Sscanf(v.Value, "%d", &n)
	return &n
}

// mapAttr extracts a nested map attribute (M), or nil if absent.
func mapAttr(item map[string]types.AttributeValue, key string) map[string]types.AttributeValue {
	v, ok := item[key].(*types.AttributeValueMemberM)
	if !ok {
		return nil
	}
	return v.Value
}

// personDoc devolve o CPF/CNPJ só com dígitos do item de cadastro. pk
// (organizations) e sk (organization_persons) já são "CNPJ_…"/"CPF_…"
// normalizados; cpf_or_cnpj é o campo do DTO e pode vir formatado.
func personDoc(item map[string]types.AttributeValue) string {
	return docPunct.Replace(services.StripPKPrefix(firstNonEmpty(
		strAttr(item, "sk"), strAttr(item, "pk"), strAttr(item, "cpf_or_cnpj"))))
}

// firstNonEmpty devolve o primeiro valor não vazio.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// firstContact devolve o primeiro item de person.contacts[key] — o DPS aceita
// um único telefone/e-mail por pessoa (mesma regra de services.FirstPhone).
func firstContact(person map[string]types.AttributeValue, key string) string {
	list := listAttr(mapAttr(person, "contacts"), key)
	if len(list) == 0 {
		return ""
	}
	v, ok := list[0].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

// listAttr extracts a list attribute (L), or nil if absent.
func listAttr(item map[string]types.AttributeValue, key string) []types.AttributeValue {
	v, ok := item[key].(*types.AttributeValueMemberL)
	if !ok {
		return nil
	}
	return v.Value
}
