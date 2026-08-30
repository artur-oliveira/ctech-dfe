package v1

import (
	"strconv"

	"github.com/go-playground/validator/v10"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/validation"
)

// Request DTOs for all mutating endpoints. Each struct mirrors the payload the
// frontend sends (see ui/src/lib/types/api.ts and ui/src/lib/schemas/*) and
// carries go-playground/validator tags so the request body is validated
// rigidly at the route boundary before any persistence happens. Unknown JSON
// fields are rejected by bindJSON (DisallowUnknownFields); these tags enforce
// presence, format, and ranges.
//
// Optional/nullable fields are pointers with `omitempty` so a null/absent value
// skips validation, while a present value is still format-checked.

// ── Shared entity sub-objects (persons & organizations) ──────────────────────

// AddressBody is a postal address (endereço) for a person/organization.
type AddressBody struct {
	CityIBGECode    string  `json:"city_ibge_code" validate:"required,ibge"`
	Street          string  `json:"street" validate:"required,max=255"`
	Neighborhood    string  `json:"neighborhood" validate:"required,max=120"`
	Number          string  `json:"number" validate:"required,max=20"`
	City            string  `json:"city" validate:"required,max=120"`
	StateFederation string  `json:"state_federation" validate:"required,uf"`
	PostalCode      string  `json:"postal_code" validate:"required,cep"`
	Complement      *string `json:"complement" validate:"omitempty,max=120"`
}

// StateRegistrationBody is a state registration (inscrição estadual) entry.
type StateRegistrationBody struct {
	UF                string `json:"uf" validate:"required,uf"`
	StateRegistration string `json:"state_registration" validate:"required,max=20"`
	// IeSt é a inscrição de substituto tributário nesta UF (emit/IEST). Mora
	// aqui, e não numa lista à parte, porque é a mesma inscrição na mesma UF
	// com outro papel.
	IeSt *string `json:"ie_st" validate:"omitempty,max=20"`
}

// ContactsBody holds e-mail and phone contact lists.
type ContactsBody struct {
	Emails []string `json:"emails" validate:"omitempty,max=5,dive,email"`
	Phones []string `json:"phones" validate:"omitempty,max=5,dive,phonebr"`
}

// NfseRegTribBody é o regime tributário do prestador (TCRegTrib do DPS 1.01).
// Vive junto da identidade, e não na config da organização, porque quando a org
// emite como tomador ou intermediário (tpEmit 2 ou 3) o prestador é uma pessoa
// do cadastro e precisa do próprio regime — ver
// docs/specs/2026-08-04-nfse-design.md §3.2.
type NfseRegTribBody struct {
	// 1 não optante | 2 optante MEI | 3 optante ME/EPP
	OpSimpNac int `json:"op_simp_nac" validate:"required,oneof=1 2 3"`
	// Exigido apenas quando op_simp_nac = 3.
	// 1 federais e municipal pelo SN | 2 federais pelo SN, ISSQN por fora | 3 ambos por fora
	RegApTribSN *int `json:"reg_ap_trib_sn" validate:"omitempty,oneof=1 2 3"`
	// 0 nenhum | 1 ato cooperado | 2 estimativa | 3 microempresa municipal
	// 4 notário/registrador | 5 profissional autônomo | 6 sociedade de profissionais | 9 outros
	RegEspTrib int `json:"reg_esp_trib" validate:"oneof=0 1 2 3 4 5 6 9"`
}

// NfseForeignAddressBody é o endereço no exterior (TCEnderExt do DPS 1.01),
// usado quando a pessoa não tem endereço nacional.
type NfseForeignAddressBody struct {
	CPais       string  `json:"c_pais" validate:"required,len=2,alpha"`
	CEndPost    string  `json:"c_end_post" validate:"required,max=11"`
	XCidade     string  `json:"x_cidade" validate:"required,max=60"`
	XEstadoProv string  `json:"x_estado_prov" validate:"required,max=60"`
	XLgr        string  `json:"x_lgr" validate:"required,max=255"`
	Nro         string  `json:"nro" validate:"required,max=60"`
	XCpl        *string `json:"x_cpl" validate:"omitempty,max=156"`
	XBairro     string  `json:"x_bairro" validate:"required,max=60"`
}

// NfseInfoBody é o grupo de campos exigidos pela NFS-e que não existem no
// cadastro de NF-e. Fica em PersonObjectBody porque TCInfoPrestador e
// TCInfoPessoa têm os mesmos campos de identidade — assim organizations e
// organization_persons são estendidas por uma única mudança.
type NfseInfoBody struct {
	IM    *string `json:"im" validate:"omitempty,inscmun"`
	Caepf *string `json:"caepf" validate:"omitempty,caepf"`
	NIF   *string `json:"nif" validate:"omitempty,nif"`
	// 0 não informado | 1 dispensado | 2 não exigência
	CNaoNIF *int `json:"c_nao_nif" validate:"omitempty,oneof=0 1 2"`
	// Obrigatório apenas quando a pessoa for usada como prestador numa emissão;
	// a validação dessa obrigatoriedade ocorre na emissão, não no cadastro.
	RegTrib        *NfseRegTribBody        `json:"reg_trib" validate:"omitempty"`
	ForeignAddress *NfseForeignAddressBody `json:"foreign_address" validate:"omitempty"`
}

// PersonObjectBody is the nested `person` object shared by person and
// organization payloads. crt is sent as a number (1–4) or null.
type PersonObjectBody struct {
	FantasyName        *string                 `json:"fantasy_name" validate:"omitempty,max=255"`
	Crt                *int                    `json:"crt" validate:"omitempty,oneof=1 2 3 4"`
	StateRegistrations []StateRegistrationBody `json:"state_registrations" validate:"omitempty,dive"`
	Addresses          []AddressBody           `json:"addresses" validate:"required,min=1,dive"`
	Contacts           *ContactsBody           `json:"contacts" validate:"omitempty"`
	Nfse               *NfseInfoBody           `json:"nfse" validate:"omitempty"`
	// CNAE do emitente. Exigido pelo leiaute quando IM está presente (NF-e
	// mista mercadoria + serviço).
	Cnae *string `json:"cnae" validate:"omitempty,len=7,number"`
	// IDEstrangeiro repetido no objeto person para que o builder da NF-e o
	// encontre onde já lê o resto da identidade.
	IDEstrangeiro *string `json:"id_estrangeiro" validate:"omitempty,max=20"`
	// Inscrição Suframa do emitente (emit/ISUFEmit, reforma tributária).
	IsufEmit *string `json:"isuf_emit" validate:"omitempty,max=9,number"`
	// FreightRetention é o perfil de ICMS retido pelo remetente sobre o frete
	// (NF-e transp/retTransp). Fica na pessoa porque é da transportadora.
	FreightRetention *FreightRetentionBody `json:"freight_retention" validate:"omitempty"`
	// Bank são os dados de recebimento do condutor/TAC (MDF-e infANTT/infBanc).
	// Ficam na pessoa porque são invariantes dela, não da viagem.
	Bank *PersonBankBody `json:"bank" validate:"omitempty"`
	// IntermediaryID é o identificador do emitente no cadastro do intermediador
	// (NF-e infIntermed/idCadIntTran) — o "seller id" do marketplace. É
	// invariante do par emitente↔plataforma, então mora na pessoa.
	IntermediaryID *string `json:"intermediary_id" validate:"omitempty,min=2,max=60"`
	// TechnicalManagerCpf é o CPF do responsável técnico agronômico
	// (NF-e agropecuario/defensivo/CPFRespTec). É o mesmo agrônomo em toda nota
	// de defensivo do emitente, então mora no cadastro, não na emissão.
	TechnicalManagerCpf *string `json:"technical_manager_cpf" validate:"omitempty,cpf"`
}

// FreightRetentionBody é o grupo retTransp: serviço, base, alíquota, CFOP e
// município do fato gerador. vICMSRet é calculado na emissão.
type FreightRetentionBody struct {
	VServ    *string `json:"v_serv" validate:"omitempty,money2"`
	VBcRet   *string `json:"v_bc_ret" validate:"omitempty,money2"`
	PIcmsRet *string `json:"p_icms_ret" validate:"omitempty,percent"`
	CFOP     *string `json:"cfop" validate:"omitempty,cfop"`
	CMunFG   *string `json:"c_mun_fg" validate:"omitempty,ibge"`
}

// PersonBankBody é o choice de infBanc: PIX, ou banco+agência, ou CNPJ do IPEF.
type PersonBankBody struct {
	PixKey     *string `json:"pix_key" validate:"omitempty,max=250"`
	BankCode   *string `json:"bank_code" validate:"omitempty,len=3,number"`
	BranchCode *string `json:"branch_code" validate:"omitempty,max=10,number"`
	CNPJIPEF   *string `json:"cnpj_ipef" validate:"omitempty,cnpj"`
}

// ── Persons ──────────────────────────────────────────────────────────────────

// personRolesValidation is the shared `validate` tag for the person role list.
// The accepted values mirror services.AllPersonRoles; TestPersonRolesTagMatchesAllPersonRoles
// fails if the two drift apart.
const personRolesValidation = "omitempty,dive,oneof=customer supplier carrier driver provider freight_contractor intermediary"

// PersonCreateBody is the body for POST /persons.
//
// Roles is a registry filter (customer/supplier/carrier/driver/provider), not a
// fiscal rule — a person may hold several at once, and an absent list is valid:
// that person simply never shows up in a role-filtered listing.
type PersonCreateBody struct {
	// CpfOrCnpj e IDEstrangeiro são alternativos: o XSD de dest é um choice
	// entre CPF, CNPJ e idEstrangeiro. Exatamente um dos dois é obrigatório.
	CpfOrCnpj string `json:"cpf_or_cnpj" validate:"omitempty,cpfcnpj"`
	// IDEstrangeiro é o documento de pessoa no exterior (dest/idEstrangeiro).
	IDEstrangeiro *string          `json:"id_estrangeiro" validate:"omitempty,max=20"`
	Name          string           `json:"name" validate:"required,min=2,max=255"`
	Roles         []string         `json:"roles" validate:"omitempty,dive,oneof=customer supplier carrier driver provider freight_contractor intermediary"`
	Person        PersonObjectBody `json:"person" validate:"required"`
}

// PersonUpdateBody is the body for PUT /persons/:cpf_cnpj (partial; the document
// is taken from the path, never the body).
//
// Roles is a pointer with `omitempty` on purpose: um corpo sem `roles` não pode
// tocar nos papéis (nulo vira REMOVE no update do DynamoDB), e `"roles": []`
// continua sendo a forma de limpar todos os papéis.
type PersonUpdateBody struct {
	Name   *string           `json:"name" validate:"omitempty,min=2,max=255"`
	Roles  *[]string         `json:"roles,omitempty" validate:"omitempty,dive,oneof=customer supplier carrier driver provider freight_contractor intermediary"`
	Person *PersonObjectBody `json:"person" validate:"omitempty"`
}

// ── Organizations ────────────────────────────────────────────────────────────

// OrganizationCreateBody is the body for POST /organizations.
type OrganizationCreateBody struct {
	CpfOrCnpj   string           `json:"cpf_or_cnpj" validate:"required,cpfcnpj"`
	Name        string           `json:"name" validate:"required,min=2,max=255"`
	Description *string          `json:"description" validate:"omitempty,max=120"`
	Person      PersonObjectBody `json:"person" validate:"required"`
}

// OrganizationUpdateBody is the body for PUT /organizations/:org_pk (partial).
type OrganizationUpdateBody struct {
	Name        *string           `json:"name" validate:"omitempty,min=2,max=255"`
	Description *string           `json:"description" validate:"omitempty,max=120"`
	Person      *PersonObjectBody `json:"person" validate:"omitempty"`
}

// AuthorizedViewerBody is one entry in an organization's SEFAZ autXML list
// (CPF/CNPJ + name authorized to view that organization's NF-e XMLs).
type AuthorizedViewerBody struct {
	CpfOrCnpj string `json:"cpf_or_cnpj" validate:"required,cpfcnpj"`
	Name      string `json:"name" validate:"required,min=2,max=60"`
}

// ── Products ─────────────────────────────────────────────────────────────────

// ConversionFactorBody is a unit-conversion factor for a product.
type ConversionFactorBody struct {
	OriginUnit string  `json:"origin_unit" validate:"required,unit"`
	TargetUnit string  `json:"target_unit" validate:"required,unit"`
	Factor     float64 `json:"factor" validate:"required,gt=0"`
}

// TaxFieldsBody is the tax treatment itself — every ICMS/CSOSN, ST, PIS,
// COFINS, IBS/CBS, IPI, IS and ISSQN field, with no CFOP attached. It is
// embedded by CfopConfigBody (treatment bound to one CFOP inside a product)
// and by TaxProfileBody (the same treatment named once and shared by many
// products). Two copies of ~60 fields is precisely the duplication the tax
// profiles exist to remove, so there is exactly one definition.
type TaxFieldsBody struct {
	// ICMS: CST (Regime Normal) or CSOSN (Simples Nacional) — validated by the
	// service against the org CRT, so kept loose here.
	Csosn *string `json:"csosn" validate:"omitempty"`
	Icms  *string `json:"icms" validate:"omitempty"`
	// ICMS alíquotas / modalidade
	IcmsModBc         *string `json:"icms_mod_bc" validate:"omitempty"`
	IcmsAliqOverride  *string `json:"icms_aliq_override" validate:"omitempty,percent"`
	IcmsFcpOverride   *string `json:"icms_fcp_override" validate:"omitempty,percent"`
	IcmsSnCredAliq    *string `json:"icms_sn_cred_aliq" validate:"omitempty,percent"`
	IcmsIndDeduzDeson *string `json:"icms_ind_deduz_deson" validate:"omitempty"`
	// ICMS ST
	IcmsStModBc   *string `json:"icms_st_mod_bc" validate:"omitempty"`
	IcmsStMva     *string `json:"icms_st_mva" validate:"omitempty,percent"`
	IcmsStRedBc   *string `json:"icms_st_red_bc" validate:"omitempty,percent"`
	IcmsStAliq    *string `json:"icms_st_aliq" validate:"omitempty,percent"`
	IcmsStFcpAliq *string `json:"icms_st_fcp_aliq" validate:"omitempty,percent"`
	// Conditional ICMS (Regime Normal)
	IcmsPRedBc     *string `json:"icms_p_red_bc" validate:"omitempty,percent"`
	IcmsMotDes     *string `json:"icms_mot_des" validate:"omitempty"`
	IcmsPDif       *string `json:"icms_p_dif" validate:"omitempty,percent"`
	IcmsPautaValor *string `json:"icms_pauta_valor" validate:"omitempty,money2"`
	// ICMS monofásico combustíveis
	IcmsAdRem       *string `json:"icms_ad_rem" validate:"omitempty,percent"`
	IcmsAdRemReten  *string `json:"icms_ad_rem_reten" validate:"omitempty,percent"`
	IcmsPRedAdRem   *string `json:"icms_p_red_ad_rem" validate:"omitempty,percent"`
	IcmsMotRedAdRem *string `json:"icms_mot_red_ad_rem" validate:"omitempty"`
	IcmsPDifMono    *string `json:"icms_p_dif_mono" validate:"omitempty,percent"`
	// ICMS60 — ST retida anteriormente
	IcmsVBcStRet     *string `json:"icms_v_bc_st_ret" validate:"omitempty"`
	IcmsVIcmsStRet   *string `json:"icms_v_icms_st_ret" validate:"omitempty"`
	IcmsPSt          *string `json:"icms_p_st" validate:"omitempty,percent"`
	IcmsFcpVBcStRet  *string `json:"icms_fcp_v_bc_st_ret" validate:"omitempty"`
	IcmsFcpStRetAliq *string `json:"icms_fcp_st_ret_aliq" validate:"omitempty,percent"`
	// ICMSST (CST 41) — repasse, na operação interestadual, da ST já retida.
	IcmsVBcStDest   *string `json:"icms_v_bc_st_dest" validate:"omitempty,money2"`
	IcmsVIcmsStDest *string `json:"icms_v_icms_st_dest" validate:"omitempty,money2"`
	// ICMS efetivo (ICMS60, ICMSST e ICMSSN500) — exigido por algumas UFs na
	// revenda de mercadoria com ST retida.
	IcmsPRedBcEfet *string `json:"icms_p_red_bc_efet" validate:"omitempty,percent"`
	IcmsPIcmsEfet  *string `json:"icms_p_icms_efet" validate:"omitempty,percent"`
	// ICMSPart — partilha do ICMS entre a UF de origem e a de destino. Não há
	// CST próprio: é o par abaixo que troca ICMS10/ICMS90 por ICMSPart.
	IcmsPartPBCOp *string `json:"icms_part_p_bc_op" validate:"omitempty,percent"`
	IcmsPartUFST  *string `json:"icms_part_uf_st" validate:"omitempty,uf"`
	// ST desonerada (ICMS10/70/90) e FCP diferido (ICMS51/90).
	IcmsMotDesSt *string `json:"icms_mot_des_st" validate:"omitempty"`
	IcmsPFcpDif  *string `json:"icms_p_fcp_dif" validate:"omitempty,percent"`
	// PIS / COFINS
	Pis            string  `json:"pis" validate:"required,digits2"`
	Cofins         string  `json:"cofins" validate:"required,digits2"`
	PisAliq        *string `json:"pis_aliq" validate:"omitempty,percent"`
	CofinsAliq     *string `json:"cofins_aliq" validate:"omitempty,percent"`
	PisAliqUnid    *string `json:"pis_aliq_unid" validate:"omitempty,percent"`
	CofinsAliqUnid *string `json:"cofins_aliq_unid" validate:"omitempty,percent"`
	// PIS / COFINS-ST — substituição tributária (grupo opcional)
	PisStAliq    *string `json:"pis_st_aliq" validate:"omitempty,percent"`
	CofinsStAliq *string `json:"cofins_st_aliq" validate:"omitempty,percent"`
	PisStVBc     *string `json:"pis_st_v_bc" validate:"omitempty,money2"`
	CofinsStVBc  *string `json:"cofins_st_v_bc" validate:"omitempty,money2"`
	// IPI por unidade (bebidas, cigarros): vUnid presente troca vBC+pIPI por
	// qUnid+vUnid — são choice no XSD.
	IpiVUnid *string `json:"ipi_v_unid" validate:"omitempty,money"`
	// Observação fiscal do item (det/obsItem). Pode vir da tributação ou do
	// produto; a tributação vence por ser a mais específica do cenário.
	ObsItemXCampo *string `json:"obs_item_x_campo" validate:"omitempty,max=20"`
	ObsItemXTexto *string `json:"obs_item_x_texto" validate:"omitempty,max=60"`
	// IBS / CBS (Reforma Tributária) — opcional, tudo-ou-nada (ver validateIbsCbsGroup).
	// Vigência obrigatória: 2026-08-03 (não-Simples) / 2027-01-04 (Simples/MEI) —
	// até lá, e mesmo depois para quem ainda não migrou, o grupo pode ficar ausente.
	IbsCbsCst       *string `json:"ibs_cbs_cst" validate:"omitempty,ibscst"`
	IbsCbsClassTrib *string `json:"ibs_cbs_class_trib" validate:"omitempty,class6"`
	IbsUfAliq       *string `json:"ibs_uf_aliq" validate:"omitempty,percent"`
	IbsMunAliq      *string `json:"ibs_mun_aliq" validate:"omitempty,percent"`
	CbsAliq         *string `json:"cbs_aliq" validate:"omitempty,percent"`
	IbsUfPRed       *string `json:"ibs_uf_p_red" validate:"omitempty,percent"`
	IbsMunPRed      *string `json:"ibs_mun_p_red" validate:"omitempty,percent"`
	CbsPRed         *string `json:"cbs_p_red" validate:"omitempty,percent"`
	IbsUfPDif       *string `json:"ibs_uf_p_dif" validate:"omitempty,percent"`
	IbsMunPDif      *string `json:"ibs_mun_p_dif" validate:"omitempty,percent"`
	CbsPDif         *string `json:"cbs_p_dif" validate:"omitempty,percent"`
	// IbsIndDoacao: o XSD (TIndDoacao) enumera um valor só, "1". "S"/"N" era o
	// domínio de uma NT anterior e hoje é rejeição.
	IbsIndDoacao *string `json:"ibs_ind_doacao" validate:"omitempty,oneof=1"`
	IbsAdRem     *string `json:"ibs_ad_rem" validate:"omitempty,percent"`
	CbsAdRem     *string `json:"cbs_ad_rem" validate:"omitempty,percent"`
	// IbsCbsPDevTrib é o percentual de devolução de tributo ao adquirente. Vale
	// nas três esferas (gIBSUF/gDevTrib, gIBSMun/gDevTrib, gCBS/gDevTrib): o
	// vDevTrib de cada uma é este percentual sobre o tributo daquela esfera.
	IbsCbsPDevTrib *string `json:"ibs_cbs_p_dev_trib" validate:"omitempty,percent"`
	// Monofasia da reforma (gIBSCBSMono). A alíquota específica é por unidade,
	// não percentual: a base é a quantidade. ibs_ad_rem/cbs_ad_rem são o
	// gMonoPadrao; *_reten é a retenção, *_ret o já retido e *_p_dif_mono o
	// diferimento — cada par é tudo-ou-nada.
	IbsAdRemReten *string `json:"ibs_ad_rem_reten" validate:"omitempty,money"`
	CbsAdRemReten *string `json:"cbs_ad_rem_reten" validate:"omitempty,money"`
	IbsAdRemRet   *string `json:"ibs_ad_rem_ret" validate:"omitempty,money"`
	CbsAdRemRet   *string `json:"cbs_ad_rem_ret" validate:"omitempty,money"`
	IbsPDifMono   *string `json:"ibs_p_dif_mono" validate:"omitempty,percent"`
	CbsPDifMono   *string `json:"cbs_p_dif_mono" validate:"omitempty,percent"`
	// Tributação de referência (gTribRegular): quanto o item pagaria fora do
	// regime ou benefício. Sem ibs_reg_cst, o bloco não é emitido.
	IbsRegCst       *string `json:"ibs_reg_cst" validate:"omitempty,ibscst"`
	IbsRegClassTrib *string `json:"ibs_reg_class_trib" validate:"omitempty,class6"`
	IbsRegUfAliq    *string `json:"ibs_reg_uf_aliq" validate:"omitempty,percent"`
	IbsRegMunAliq   *string `json:"ibs_reg_mun_aliq" validate:"omitempty,percent"`
	CbsRegAliq      *string `json:"cbs_reg_aliq" validate:"omitempty,percent"`
	// Tributação de compra governamental (gTribCompraGov): quanto o item
	// pagaria se o comprador não fosse ente público. Não tem CST próprio.
	IbsGovUfAliq  *string `json:"ibs_gov_uf_aliq" validate:"omitempty,percent"`
	IbsGovMunAliq *string `json:"ibs_gov_mun_aliq" validate:"omitempty,percent"`
	CbsGovAliq    *string `json:"cbs_gov_aliq" validate:"omitempty,percent"`
	// Crédito presumido da operação (gCredPresOper). O valor de cada esfera é o
	// percentual sobre a base; cond_sus só muda a tag de destino (o choice
	// vCredPres | vCredPresCondSus), nunca a conta.
	IbsCbsCCredPres       *string `json:"ibs_cbs_c_cred_pres" validate:"omitempty,len=2,number"`
	IbsPCredPres          *string `json:"ibs_p_cred_pres" validate:"omitempty,percent"`
	CbsPCredPres          *string `json:"cbs_p_cred_pres" validate:"omitempty,percent"`
	IbsCbsCredPresCondSus *string `json:"ibs_cbs_cred_pres_cond_sus" validate:"omitempty,oneof=1"`
	// Crédito presumido do IBS na ZFM (gCredPresIBSZFM). A classificação vem do
	// produto (tp_cred_pres_ibs_zfm); aqui só o percentual.
	IbsZfmPCredPres *string `json:"ibs_zfm_p_cred_pres" validate:"omitempty,percent"`
	// Alíquota zero da CBS em ALC/ZFM (gCBS/gALCZFMCBS). tp: 1 ou 2; a alíquota
	// de referência é cbs_reg_aliq.
	AlcZfmTpCbs        *string `json:"alc_zfm_tp_cbs" validate:"omitempty,oneof=1 2"`
	AlcZfmNProcSuframa *string `json:"alc_zfm_n_proc_suframa" validate:"omitempty,min=8,max=12"`
	// IPI
	IpiCst  *string `json:"ipi_cst" validate:"omitempty"`
	IpiAliq *string `json:"ipi_aliq" validate:"omitempty,percent"`
	// IS — Imposto Seletivo
	IsCst       *string `json:"is_cst" validate:"omitempty"`
	IsAliq      *string `json:"is_aliq" validate:"omitempty,percent"`
	IsClassTrib *string `json:"is_class_trib" validate:"omitempty,class6"`
	IsAliqEspec *string `json:"is_aliq_espec" validate:"omitempty,percent"`
	IsUnidTrib  *string `json:"is_unid_trib" validate:"omitempty"`
	// ISSQN
	IssqnIndIss    *string `json:"issqn_ind_iss" validate:"omitempty"`
	IssqnCListServ *string `json:"issqn_c_list_serv" validate:"omitempty"`
	IssqnCMunFg    *string `json:"issqn_c_mun_fg" validate:"omitempty,ibge"`
	IssqnAliq      *string `json:"issqn_aliq" validate:"omitempty,percent"`
	IssqnVDeducao  *string `json:"issqn_v_deducao" validate:"omitempty"`
	IssqnVIssRet   *string `json:"issqn_v_iss_ret" validate:"omitempty"`
	// Restante do grupo ISSQN do leiaute.
	IssqnVOutro       *string `json:"issqn_v_outro" validate:"omitempty,money2"`
	IssqnVDescIncond  *string `json:"issqn_v_desc_incond" validate:"omitempty,money2"`
	IssqnVDescCond    *string `json:"issqn_v_desc_cond" validate:"omitempty,money2"`
	IssqnCServico     *string `json:"issqn_c_servico" validate:"omitempty,max=20"`
	IssqnCMun         *string `json:"issqn_c_mun" validate:"omitempty,ibge"`
	IssqnCPais        *string `json:"issqn_c_pais" validate:"omitempty,max=4,number"`
	IssqnNProcesso    *string `json:"issqn_n_processo" validate:"omitempty,max=30"`
	IssqnIndIncentivo *string `json:"issqn_ind_incentivo" validate:"omitempty,oneof=1 2"`
}

// UfTaxOverride is a partial TaxFieldsBody override applied only when the
// operation's destination UF is in Ufs. It does not duplicate all ~60 tax
// fields — only the ones that diverge for that set of UFs (design spec
// 2026-08-09-tax-config-redesign §Modelo de dados 1).
type UfTaxOverride struct {
	Ufs       []string       `json:"ufs" validate:"required,min=1,dive,uf"`
	Overrides map[string]any `json:"overrides" validate:"omitempty"`
}

// CfopConfigBody is one per-CFOP tax configuration entry of a product.
// Optional tax fields are nullable and only format-checked when present.
type CfopConfigBody struct {
	Cfop        string          `json:"cfop" validate:"required,cfop"`
	UfOverrides []UfTaxOverride `json:"uf_overrides" validate:"omitempty,dive"`
	TaxFieldsBody
}

// ── Vehicle sets (composições veiculares) ────────────────────────────────────

// VehicleSetBody is the body for POST/PUT /vehicle-sets.
//
// Hoje o MDF-e escolhe veículo, até 3 reboques e condutores um a um, todo dia,
// para os mesmos conjuntos. Aqui o conjunto é nomeado uma vez ("Carreta 1 —
// ABC1D23"). Na emissão, **cada campo expandido continua sobrescrevível
// individualmente** — trocar o motorista de um dia não exige criar outro conjunto.
type VehicleSetBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// TractorSK é o veículo de tração; validado como role=tractor no serviço.
	TractorSK string `json:"tractor_sk" validate:"required"`
	// TrailerSKs são os reboques (máx. 3 pelo leiaute), role=trailer.
	TrailerSKs []string `json:"trailer_sks" validate:"omitempty,max=3,dive,required"`
	// DriverDocs são CPFs de pessoas do cadastro, tipicamente com papel driver.
	DriverDocs []string `json:"driver_docs" validate:"omitempty,dive,cpf"`
	RNTRC      *string  `json:"rntrc" validate:"omitempty,rntrc"`
	CIOT       *string  `json:"ciot" validate:"omitempty"`
}

// ── Payment terms (condições de pagamento) ───────────────────────────────────

// PaymentTermBody is the body for POST/PUT /payment-terms.
//
// Hoje as parcelas de uma NF-e são digitadas uma a uma para condições fixas
// ("30/60/90", "à vista", "boleto 28 dias"). Aqui a condição é nomeada uma vez
// e expandida na emissão a partir do total do documento.
type PaymentTermBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// PaymentType usa o mesmo domínio de NfePaymentItem.PaymentType.
	PaymentType string `json:"payment_type" validate:"required,oneof=01 02 03 04 05 10 11 12 13 14 15 16 17 18 19 20 21 22 90 99"`
	// IndPag: 0 à vista, 1 a prazo. Ausente, é derivado das parcelas.
	IndPag       *string        `json:"ind_pag" validate:"omitempty,oneof=0 1"`
	Installments int            `json:"installments" validate:"required,min=1,max=120"`
	IntervalDays int            `json:"interval_days" validate:"omitempty,min=0,max=365"`
	FirstDueDays int            `json:"first_due_days" validate:"omitempty,min=0,max=365"`
	Card         map[string]any `json:"card" validate:"omitempty"`
}

// ── Operations (naturezas de operação) ───────────────────────────────────────

// OperationBody is the body for POST/PUT /operations.
//
// Uma natureza de operação junta os valores que **sempre andam juntos** por
// cenário de negócio ("venda para revenda", "remessa para conserto",
// "devolução de compra"): natOp, tpNF, finNFe, indFinal, indPres, o CFOP de
// cada item e a mensagem fiscal. Hoje o produto trata cada um como pergunta
// independente, e o operador responde as mesmas seis perguntas toda vez.
//
// Valor explícito no request de emissão sempre vence a operação — a operação é
// default, não prisão.
type OperationBody struct {
	Name     string   `json:"name" validate:"required,min=2,max=120"`
	DocTypes []string `json:"doc_types" validate:"omitempty,dive,oneof=nfe nfce cte mdfe nfse"`

	NatOp    *string `json:"nat_op" validate:"omitempty,max=60"`
	TpNF     *string `json:"tp_nf" validate:"omitempty,oneof=0 1"`
	FinNFe   *string `json:"fin_nfe" validate:"omitempty,oneof=1 2 3 4"`
	IndFinal *string `json:"ind_final" validate:"omitempty,oneof=0 1"`
	IndPres  *string `json:"ind_pres" validate:"omitempty,oneof=0 1 2 3 4 5 9"`

	// CfopSuffix é a natureza fiscal (3 dígitos). O dígito de escopo (5/6/7) é
	// resolvido na emissão por services.ResolveCFOPScope, a partir das UFs.
	CfopSuffix *string `json:"cfop_suffix" validate:"omitempty,len=3,number"`

	TaxProfileID  *string `json:"tax_profile_id" validate:"omitempty"`
	PaymentTermID *string `json:"payment_term_id" validate:"omitempty"`
	ModFrete      *string `json:"mod_frete" validate:"omitempty,oneof=0 1 2 3 4 9"`

	// Espécie e marca padrão dos volumes (transp/vol). São característica da
	// operação, não da nota — quem sempre despacha em caixa não redigita.
	VolEsp   *string `json:"vol_esp" validate:"omitempty,max=60"`
	VolMarca *string `json:"vol_marca" validate:"omitempty,max=60"`

	// Aceitam placeholders {{chave}} — ver services.AllPlaceholders. Chave
	// desconhecida é erro aqui, no cadastro, nunca silêncio no XML.
	InfAdFisco *string `json:"inf_ad_fisco" validate:"omitempty,max=2000"`
	InfCpl     *string `json:"inf_cpl" validate:"omitempty,max=5000"`

	// ObsCont/ObsFisco são observações de campo livre do leiaute (máx 10 cada).
	// Aceitam os mesmos placeholders de inf_cpl.
	ObsCont  []ObsBody `json:"obs_cont" validate:"omitempty,max=10,dive"`
	ObsFisco []ObsBody `json:"obs_fisco" validate:"omitempty,max=10,dive"`

	// ExportUFSaidaPais e ExportLocDespachoIndex montam infNFe/exporta. O local
	// aponta um índice em organizations.pickup_locations — o endereço não é
	// copiado, é referenciado.
	ExportUFSaidaPais      *string `json:"export_uf_saida_pais" validate:"omitempty,uf"`
	ExportLocDespachoIndex *int    `json:"export_loc_despacho_index" validate:"omitempty,min=0"`

	// RetTrib é o perfil de retenções federais do cenário (total/retTrib). Os
	// percentuais são invariantes da operação; os valores saem da base da nota.
	RetTrib *RetTribBody `json:"ret_trib" validate:"omitempty"`

	// CompraXNEmp é a nota de empenho do cenário de venda a órgão público
	// (infNFe/compra/xNEmp). Quem vende por empenho vende sempre por empenho;
	// pedido e contrato variam por nota e vão no request de emissão.
	CompraXNEmp *string `json:"compra_x_n_emp" validate:"omitempty,min=1,max=22"`

	// IntermediaryPersonID é o marketplace/plataforma do cenário
	// (infNFe/infIntermed) e IndIntermed marca ide/indIntermed. Uma operação
	// por canal de venda: "venda no site próprio" é 0, "venda no marketplace X"
	// é 1 mais a pessoa da plataforma.
	IntermediaryPersonID *string `json:"intermediary_person_id" validate:"omitempty"`
	IndIntermed          *string `json:"ind_intermed" validate:"omitempty,oneof=0 1"`

	// Reforma tributária no ide. Todos são do cenário, não da nota: o local da
	// operação de fornecimento, o município do fato gerador do IBS/CBS (só
	// quando ind_pres é 5 e não há endereço de destinatário nem de entrega) e o
	// par nota de débito / nota de crédito.
	// TpNFDebito (01–08) e TpNFCredito (01–06) são os motivos da nota de débito
	// e da nota de crédito da reforma (TTpNFDebito / TTpNFCredito). São códigos
	// de dois dígitos, não o 0/1 de entrada/saída do tpNF.
	CIndOp      *string `json:"c_ind_op" validate:"omitempty,len=6,number"`
	CMunFGIBS   *string `json:"c_mun_fg_ibs" validate:"omitempty,ibge"`
	TpNFDebito  *string `json:"tp_nf_debito" validate:"omitempty,oneof=01 02 03 04 05 06 07 08"`
	TpNFCredito *string `json:"tp_nf_credito" validate:"omitempty,oneof=01 02 03 04 05 06"`

	// Compras governamentais (ide/gCompraGov). tp_ente_gov: 1 União, 2 Estados,
	// 3 DF, 4 Municípios, 5 Consórcio Público, 6 Comitê Gestor do IBS.
	// tp_oper_gov: 1 fornecimento com pagamento posterior, 2 recebimento do
	// pagamento com fornecimento já realizado, 3 fornecimento com pagamento já
	// realizado, 4 recebimento do pagamento com fornecimento posterior. As
	// chaves de refDFeAnt são da nota, não do cadastro.
	CompraGovTpEnte   *string `json:"compra_gov_tp_ente" validate:"omitempty,oneof=1 2 3 4 5 6"`
	CompraGovPRedutor *string `json:"compra_gov_p_redutor" validate:"omitempty,percent"`
	CompraGovTpOper   *string `json:"compra_gov_tp_oper" validate:"omitempty,oneof=1 2 3 4"`

	// DhSaiEntOffsetDays é o prazo padrão de saída da mercadoria, em dias
	// corridos a partir da emissão (ide/dhSaiEnt). Quem despacha sempre no dia
	// seguinte cadastra 1 e nunca mais digita a data.
	DhSaiEntOffsetDays *int `json:"dh_sai_ent_offset_days" validate:"omitempty,min=0,max=365"`

	// CanaSafra é a safra do registro de aquisição de cana (infNFe/cana/safra),
	// ex. "2025/2026". O mês de referência e os fornecimentos diários variam
	// por nota e vão no request.
	CanaSafra *string `json:"cana_safra" validate:"omitempty,min=4,max=9"`

	// RequiresReceiver falso habilita emissão sem destinatário (self_issuance).
	RequiresReceiver *bool `json:"requires_receiver" validate:"omitempty"`
	// IsDefault marca a operação pré-selecionada da organização. Só uma pode
	// estar marcada; marcar uma desmarca a anterior no mesmo TransactWrite.
	// Vale para NFS-e apenas quando doc_types inclui "nfse": este contrato não
	// cria um segundo default implícito nem muda o default dos demais.
	IsDefault bool `json:"is_default"`

	// Nfse são os defaults da natureza de operação quando doc_types inclui
	// "nfse". Fica num subobjeto porque quase nada acima se aplica à NFS-e: a
	// competência é municipal, não há CFOP, tpNF nem volume.
	Nfse *OperationNfseBody `json:"nfse" validate:"omitempty"`
}

// OperationNfseBody são os defaults de NFS-e de uma natureza de operação.
//
// A ordem de resolução na emissão é **operação → serviço → request**: o valor
// do request sempre vence, e o serviço vence a operação, porque o serviço é o
// que descreve a atividade e a operação é só o cenário de negócio.
type OperationNfseBody struct {
	// Local de prestação: escolha exclusiva município OU país, como no XSD.
	CLocPrestacao  *string `json:"c_loc_prestacao" validate:"omitempty,ibge"`
	CPaisPrestacao *string `json:"c_pais_prestacao" validate:"omitempty,len=2,alpha"`

	// Defaults de comércio exterior do cenário. O valor em moeda, a DI e o RE
	// nascem por emissão e não têm default.
	ForeignTrade *ServiceForeignTradeDefaultsBody `json:"foreign_trade" validate:"omitempty"`

	// Pedido e documento técnico do cenário (serv/infoCompl). As referências de
	// item do pedido variam por nota e vão no request.
	IdDocTec *string `json:"id_doc_tec" validate:"omitempty,max=20"`
	DocRef   *string `json:"doc_ref" validate:"omitempty,max=255"`
	XPed     *string `json:"x_ped" validate:"omitempty,max=20"`

	// Descontos padrão do cenário, em percentual do valor do serviço. Valor
	// absoluto varia por nota e vai no request.
	PDescIncond *string `json:"p_desc_incond" validate:"omitempty,percent"`
	PDescCond   *string `json:"p_desc_cond" validate:"omitempty,percent"`

	// Finalidade e ente governamental do IBS/CBS. tp_ente_gov só se aplica a
	// fornecimento para a administração pública.
	IndFinal  *string `json:"ind_final" validate:"omitempty,nfseenum=TSRTCIndFinal"`
	TpOper    *string `json:"tp_oper" validate:"omitempty,nfseenum=TSRTCTpOper"`
	IndDest   *string `json:"ind_dest" validate:"omitempty,nfseenum=TSRTCIndDest"`
	TpEnteGov *string `json:"tp_ente_gov" validate:"omitempty,nfseenum=TSRTCTpEnteGov"`

	// Aceita os mesmos placeholders de inf_cpl (services.AllPlaceholders).
	// Vira serv/infoCompl/xInfComp na DPS.
	XInfComp *string `json:"x_inf_comp" validate:"omitempty,max=2000"`
}

// ── Locais de prestação de serviço (NFS-e) ───────────────────────────────────

// Papéis combináveis de um local de prestação. O XSD repete o mesmo endereço em
// `serv/obra`, `serv/atvEvento` e `IBSCBS/imovel`; um canteiro que também é o
// endereço do imóvel tributado seria dois cadastros idênticos se os papéis
// fossem exclusivos.
const (
	ServiceLocationRoleWork       = "work"
	ServiceLocationRoleProperty   = "property"
	ServiceLocationRoleEventVenue = "event_venue"
)

// ServiceLocationBody é o body de POST/PUT /service-locations.
//
// Um endereço só, com os identificadores fiscais que cada papel exige. Os
// campos são opcionais aqui porque um mesmo local pode ter só o CIB (imóvel já
// cadastrado no fisco) ou só o endereço (obra sem CNO ainda); a emissão é que
// exige o identificador do papel que a nota usa.
type ServiceLocationBody struct {
	Name  string   `json:"name" validate:"required,min=2,max=120"`
	Roles []string `json:"roles" validate:"required,min=1,dive,oneof=work property event_venue"`

	Address ServiceLocationAddressBody `json:"address" validate:"required"`

	// InscImobFisc é a inscrição imobiliária fiscal do município (até 30).
	InscImobFisc *string `json:"insc_imob_fisc" validate:"omitempty,min=1,max=30"`
	// CObra é o código da obra (CNO/CEI), até 30 caracteres.
	CObra *string `json:"c_obra" validate:"omitempty,min=1,max=30"`
	// CIB é o Cadastro Imobiliário Brasileiro: exatamente 8 caracteres.
	CIB *string `json:"cib" validate:"omitempty,len=8"`
	// IDAtvEvt identifica a atividade de evento (até 30). Nome e período do
	// evento variam por nota e vão no request de emissão, não aqui.
	IDAtvEvt *string `json:"id_atv_evt" validate:"omitempty,min=1,max=30"`
}

// ServiceLocationAddressBody é o endereço do local. Nacional usa CEP; exterior
// usa código postal, cidade e região — a mesma escolha de TCEnderecoSimples.
type ServiceLocationAddressBody struct {
	Street       string  `json:"street" validate:"required,max=255"`
	Number       string  `json:"number" validate:"required,max=60"`
	Complement   *string `json:"complement" validate:"omitempty,max=156"`
	Neighborhood string  `json:"neighborhood" validate:"required,max=60"`

	// Nacional: CEP e município IBGE.
	PostalCode   *string `json:"postal_code" validate:"omitempty,cep"`
	CityIBGECode *string `json:"city_ibge_code" validate:"omitempty,ibge"`

	// Exterior: código postal livre, cidade e estado/província/região.
	ForeignPostalCode *string `json:"foreign_postal_code" validate:"omitempty,max=11"`
	ForeignCity       *string `json:"foreign_city" validate:"omitempty,max=60"`
	ForeignRegion     *string `json:"foreign_region" validate:"omitempty,max=60"`
}

// Validate cobre a regra que as tags não expressam: CNO, CIB e inscrição
// imobiliária são registros fiscais brasileiros. Num endereço no exterior eles
// não existem, e aceitá-los aqui produziria um cadastro que a emissão só
// conseguiria usar gerando uma DPS inválida.
func (b ServiceLocationBody) Validate() error {
	if b.Address.ForeignPostalCode == nil || *b.Address.ForeignPostalCode == "" {
		return nil
	}
	for field, value := range map[string]*string{
		"c_obra": b.CObra, "cib": b.CIB, "insc_imob_fisc": b.InscImobFisc,
	} {
		if value != nil && *value != "" {
			return problem.BadRequest(field + " não se aplica a um local no exterior")
		}
	}
	return nil
}

// ── Documentos referenciados (NFS-e) ─────────────────────────────────────────

// Famílias documentais aceitas. A mesma entidade alimenta `vDedRed/documentos`
// e `gReeRepRes/documentos`: o leiaute pede formas diferentes do mesmo
// documento nos dois grupos, e cadastrar duas vezes convidaria a divergência.
const (
	ReferenceDocumentKindDFe       = "dfe"
	ReferenceDocumentKindNFSeMun   = "nfse_municipal"
	ReferenceDocumentKindNFNFS     = "nf_nfs"
	ReferenceDocumentKindTaxOther  = "doc_fiscal_outro"
	ReferenceDocumentKindNonFiscal = "doc_nao_fiscal"
)

// ReferenceDocumentBody é o body de POST/PUT /reference-documents.
//
// União tipada: `kind` decide qual subobjeto é obrigatório, e os demais têm de
// estar ausentes. Um documento com dois subobjetos preenchidos deixaria a
// emissão escolher em silêncio qual ramo do `xs:choice` gerar.
type ReferenceDocumentBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	Kind string `json:"kind" validate:"required,oneof=dfe nfse_municipal nf_nfs doc_fiscal_outro doc_nao_fiscal"`

	DFe       *ReferenceDocumentDFeBody       `json:"dfe" validate:"omitempty"`
	NFSeMun   *ReferenceDocumentNFSeMunBody   `json:"nfse_municipal" validate:"omitempty"`
	NFNFS     *ReferenceDocumentNFNFSBody     `json:"nf_nfs" validate:"omitempty"`
	TaxOther  *ReferenceDocumentTaxOtherBody  `json:"doc_fiscal_outro" validate:"omitempty"`
	NonFiscal *ReferenceDocumentNonFiscalBody `json:"doc_nao_fiscal" validate:"omitempty"`

	// SupplierPersonID aponta o fornecedor no cadastro de pessoas. Nunca é
	// copiado: o documento referencia a pessoa, como no resto do produto.
	SupplierPersonID *string `json:"supplier_person_id" validate:"omitempty"`

	IssuedAt     string  `json:"issued_at" validate:"required,isodate"`
	CompetenceAt *string `json:"competence_at" validate:"omitempty,isodate"`
	Description  *string `json:"description" validate:"omitempty,min=1,max=150"`
}

// Validate cobre as duas regras que as tags não expressam: a competência não
// pode ser anterior à emissão do documento, e a descrição livre só existe para
// as famílias em que o leiaute não traz um campo descritivo próprio.
func (b ReferenceDocumentBody) Validate() error {
	if b.CompetenceAt != nil && *b.CompetenceAt != "" && *b.CompetenceAt < b.IssuedAt {
		return problem.BadRequest("competence_at não pode ser anterior a issued_at")
	}
	if b.Kind == ReferenceDocumentKindDFe && b.DFe != nil {
		// chNFSe tem 50 dígitos e chNFe tem 44: o tipo declarado e o
		// comprimento da chave têm de concordar, ou a dedução aponta para um
		// documento que não existe.
		want, ok := referenceDFeKeyLength[b.DFe.TipoChaveDFe]
		if ok && len(b.DFe.ChaveDFe) != want {
			return problem.BadRequest("chave_dfe deve ter " + strconv.Itoa(want) +
				" dígitos para tipo_chave_dfe " + b.DFe.TipoChaveDFe)
		}
	}
	return nil
}

// referenceDFeKeyLength é o comprimento da chave por tipo de DF-e. CT-e e
// "outro" não entram: o RTC aceita chave de até 50 caracteres sem fixar o
// tamanho, e inventar um limite recusaria documento válido.
var referenceDFeKeyLength = map[string]int{
	"1": 50, // NFS-e
	"2": 44, // NF-e
}

// ReferenceDocumentDFeBody é um documento do Repositório Nacional pela chave.
// `tipo_chave_dfe` é o domínio do RTC; em `vDedRed` ele decide entre `chNFSe`
// (NFS-e, 50 dígitos) e `chNFe` (NF-e, 44).
type ReferenceDocumentDFeBody struct {
	TipoChaveDFe  string  `json:"tipo_chave_dfe" validate:"required,nfseenum=TSRTCTipoChaveDFe"`
	ChaveDFe      string  `json:"chave_dfe" validate:"required,min=1,max=50,number"`
	XTipoChaveDFe *string `json:"x_tipo_chave_dfe" validate:"omitempty,min=1,max=255"`
}

// ReferenceDocumentNFSeMunBody é a NFS-e municipal anterior ao Sistema
// Nacional (TCDocOutNFSe).
type ReferenceDocumentNFSeMunBody struct {
	CMunNFSeMun   string `json:"c_mun_nfse_mun" validate:"required,ibge"`
	NNFSeMun      string `json:"n_nfse_mun" validate:"required,len=15,number"`
	CVerifNFSeMun string `json:"c_verif_nfse_mun" validate:"required,min=1,max=9,alphanum"`
}

// ReferenceDocumentNFNFSBody é a nota fiscal ou nota fiscal de serviço não
// eletrônica (TCDocNFNFS).
type ReferenceDocumentNFNFSBody struct {
	NNFS     string `json:"n_nfs" validate:"required,len=7,number"`
	ModNFS   string `json:"mod_nfs" validate:"required,len=15,number"`
	SerieNFS string `json:"serie_nfs" validate:"required,min=1,max=15,alphanum"`
}

// ReferenceDocumentTaxOtherBody é outro documento fiscal. Município e descrição
// só existem em `gReeRepRes`; em `vDedRed` o leiaute pede apenas o número, que
// sai daqui sem cadastro extra.
type ReferenceDocumentTaxOtherBody struct {
	NDocFiscal    string  `json:"n_doc_fiscal" validate:"required,min=1,max=255"`
	CMunDocFiscal *string `json:"c_mun_doc_fiscal" validate:"omitempty,ibge"`
	XDocFiscal    *string `json:"x_doc_fiscal" validate:"omitempty,min=1,max=255"`
}

// ReferenceDocumentNonFiscalBody é um documento não fiscal (recibo, contrato).
type ReferenceDocumentNonFiscalBody struct {
	NDoc string  `json:"n_doc" validate:"required,min=1,max=255"`
	XDoc *string `json:"x_doc" validate:"omitempty,min=1,max=255"`
}

// PaymentTerminalBody é o body de POST/PUT /payment-terminals.
//
// Um terminal de captura (POS) tem CNPJ recebedor e identificador próprios,
// invariantes por maquininha. Ficam aqui para que a NFC-e só aponte o terminal.
type PaymentTerminalBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// CNPJReceb — CNPJ do estabelecimento credenciado que recebe o pagamento.
	CNPJReceb string `json:"cnpj_receb" validate:"required,cnpj"`
	// IdTermPag — identificador do terminal, atribuído pela adquirente.
	IdTermPag string `json:"id_term_pag" validate:"required,max=40"`
	// CNPJPag/UFPag identificam o pagador institucional quando a operação de
	// pagamento ocorre fora do estabelecimento emitente (detPag/CNPJPag).
	CNPJPag *string `json:"cnpj_pag" validate:"omitempty,cnpj"`
	UFPag   *string `json:"uf_pag" validate:"omitempty,uf"`
	// TBand é a bandeira default (card/tBand). Sobrescrevível na emissão.
	TBand *string `json:"t_band" validate:"omitempty,max=2"`
}

// TollProviderBody é o body de POST/PUT /toll-providers.
//
// Vale-pedágio é obrigatório no transporte rodoviário de carga (Lei 10.209). A
// fornecedora e o pagador são invariantes; por viagem muda só o número da
// compra e o valor, que vão no corpo da emissão.
type TollProviderBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// CNPJForn — CNPJ da fornecedora do vale-pedágio.
	CNPJForn string `json:"cnpj_forn" validate:"required,cnpj"`
	// Pagador do vale, quando não é o emitente. Um dos dois, nunca ambos.
	CNPJPg *string `json:"cnpj_pg" validate:"omitempty,cnpj,excluded_with=CPFPg"`
	CPFPg  *string `json:"cpf_pg" validate:"omitempty,cpf,excluded_with=CNPJPg"`
	// TpValePed: 01 TAG, 02 cupom, 03 cartão.
	TpValePed *string `json:"tp_vale_ped" validate:"omitempty,oneof=01 02 03"`
}

// CargoUnitBody é o body de POST/PUT /cargo-units.
//
// Uma unidade de transporte (carreta, vagão) ou de carga (contêiner, pallet)
// recorre entre viagens e tem identificação própria. O rateio (qtdRat) não vive
// aqui: é calculado dos pesos dos documentos a cada manifesto.
type CargoUnitBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// Kind separa infUnidTransp de infUnidCarga — a estrutura é a mesma, o nó não.
	Kind string `json:"kind" validate:"required,oneof=transport cargo"`
	// TpUnidTransp: 1 rodoviário tração, 2 rodoviário reboque, 3 navio, 4 balsa,
	// 5 aeronave, 6 vagão, 7 outros. TpUnidCarga: 1 contêiner, 2 ULD, 3 pallet, 4 outros.
	TpUnid string `json:"tp_unid" validate:"required,oneof=1 2 3 4 5 6 7"`
	// IdUnid é a identificação (placa, número do contêiner, número do vagão).
	IdUnid string `json:"id_unid" validate:"required,max=20"`
	// Seals são os lacres fixos da unidade, quando houver.
	Seals []string `json:"seals" validate:"omitempty,dive,max=60"`
}

// RetTribBody são os percentuais de retenção federal da operação. O que sai no
// XML (vRetPIS, vRetCOFINS, vRetCSLL, vIRRF, vRetPrev) é calculado da base.
type RetTribBody struct {
	PRetPis      *string `json:"p_ret_pis" validate:"omitempty,percent"`
	PRetCofins   *string `json:"p_ret_cofins" validate:"omitempty,percent"`
	PRetCsll     *string `json:"p_ret_csll" validate:"omitempty,percent"`
	PRetIrrf     *string `json:"p_ret_irrf" validate:"omitempty,percent"`
	PRetPrevInss *string `json:"p_ret_prev_inss" validate:"omitempty,percent"`
}

// ImportDeclarationBody é o body de POST/PUT /import-declarations.
//
// Uma DI cobre várias notas e vários itens. Ela é cadastrada uma vez, com suas
// adições; na emissão o item só aponta qual adição o representa, e nAdicao /
// nSeqAdic saem desse vínculo.
type ImportDeclarationBody struct {
	Name       string `json:"name" validate:"required,min=2,max=120"`
	NDI        string `json:"n_di" validate:"required,max=15"`
	DDI        string `json:"d_di" validate:"required,isodate"`
	XLocDesemb string `json:"x_loc_desemb" validate:"required,max=60"`
	UFDesemb   string `json:"uf_desemb" validate:"required,uf"`
	DDesemb    string `json:"d_desemb" validate:"required,isodate"`
	// tpViaTransp: 01 marítima … 12 por reboque (TViaTransp do XSD).
	TpViaTransp string `json:"tp_via_transp" validate:"required,len=2,number"`
	// vAFRMM é obrigatório quando tpViaTransp = 01 (marítima).
	VAFRMM *string `json:"v_afrmm" validate:"omitempty,money2"`
	// tpIntermedio: 1 conta própria, 2 conta e ordem, 3 encomenda.
	TpIntermedio string               `json:"tp_intermedio" validate:"required,oneof=1 2 3"`
	CNPJ         *string              `json:"cnpj" validate:"omitempty,cnpj"`
	UFTerceiro   *string              `json:"uf_terceiro" validate:"omitempty,uf"`
	CExportador  string               `json:"c_exportador" validate:"required,max=60"`
	Additions    []ImportAdditionBody `json:"additions" validate:"required,min=1,max=100,dive"`
}

// Validate cobre a regra que as tags não expressam: o AFRMM é obrigatório no
// transporte marítimo, e uma DI sem ele seria recusada só lá na SEFAZ.
func (b ImportDeclarationBody) Validate() error {
	if b.TpViaTransp == tpViaTranspMaritima && (b.VAFRMM == nil || *b.VAFRMM == "") {
		return problem.BadRequest("v_afrmm é obrigatório quando a via de transporte é marítima (01)")
	}
	return nil
}

// tpViaTranspMaritima é a via de transporte 01 (marítima), a única que exige AFRMM.
const tpViaTranspMaritima = "01"

// ImportAdditionBody é uma adição da DI (prod/DI/adi).
type ImportAdditionBody struct {
	NAdicao     string  `json:"n_adicao" validate:"required,max=3,number"`
	CFabricante string  `json:"c_fabricante" validate:"required,max=60"`
	VDescDI     *string `json:"v_desc_di" validate:"omitempty,money2"`
	NDraw       *string `json:"n_draw" validate:"omitempty,max=20"`
}

// FuelPumpBody é o body de POST/PUT /fuel-pumps.
//
// Bico, bomba e tanque são físicos: recorrem em toda venda do posto. A leitura
// do encerrante (`last_v_enc_fin`) **não** entra aqui — ela é escrita pela
// emissão, na mesma transação que reserva o número da nota, e digitá-la à mão
// quebraria a sequência que a SEFAZ confere.
type FuelPumpBody struct {
	Name    string `json:"name" validate:"required,min=2,max=120"`
	NBico   string `json:"n_bico" validate:"required,max=3,number"`
	NBomba  string `json:"n_bomba" validate:"omitempty,max=3,number"`
	NTanque string `json:"n_tanque" validate:"omitempty,max=3,number"`
}

// ProductLotBody é o body de POST/PUT /product-lots.
//
// O lote é do produto e reaparece em várias notas até acabar, então é cadastro
// e não campo de emissão: a nota só aponta qual lote saiu, e a quantidade é
// rateada pela quantidade vendida.
type ProductLotBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// ProductID amarra o lote ao produto — um lote de um produto não pode sair
	// no item de outro.
	ProductID string `json:"product_id" validate:"required"`
	NLote     string `json:"n_lote" validate:"required,max=20"`
	// QLote é a quantidade produzida no lote. Serve de saldo e de referência; a
	// quantidade que sai em cada nota vem do item.
	QLote  string  `json:"q_lote" validate:"required,decimalv"`
	DFab   string  `json:"d_fab" validate:"required,isodate"`
	DVal   string  `json:"d_val" validate:"required,isodate"`
	CAgreg *string `json:"c_agreg" validate:"omitempty,max=20"`
}

// Validate cobre a regra que as tags não expressam: um lote que vence antes de
// ser fabricado é erro de digitação, não dado.
func (b ProductLotBody) Validate() error {
	if b.DVal < b.DFab {
		return problem.BadRequest("d_val não pode ser anterior a d_fab")
	}
	return nil
}

// InsurancePolicyBody é o body de POST/PUT /insurance-policies.
//
// A apólice e a seguradora recorrem entre viagens; por viagem muda só a
// averbação, que vai no corpo da emissão do MDF-e.
type InsurancePolicyBody struct {
	Name string `json:"name" validate:"required,min=2,max=120"`
	// RespSeg: 1 emitente do MDF-e, 2 contratante do serviço de transporte.
	RespSeg string `json:"resp_seg" validate:"required,oneof=1 2"`
	// Documento do responsável pelo seguro. Só é informado quando o responsável
	// não é o emitente — logo, obrigatório quando resp_seg = 2.
	CNPJ *string `json:"cnpj" validate:"omitempty,cnpj,excluded_with=CPF"`
	CPF  *string `json:"cpf" validate:"omitempty,cpf,excluded_with=CNPJ"`
	// Seguradora: nome e CNPJ (infSeg). Andam juntos ou nenhum dos dois.
	XSeg    *string `json:"x_seg" validate:"omitempty,min=2,max=30"`
	CNPJSeg *string `json:"cnpj_seg" validate:"omitempty,cnpj"`
	NApol   *string `json:"n_apol" validate:"omitempty,max=20"`
}

// Validate cobre as duas regras que as tags não expressam: o responsável que
// não é o emitente precisa se identificar, e a seguradora é nome + CNPJ ou
// nada — meia seguradora o XSD recusa.
func (b InsurancePolicyBody) Validate() error {
	if b.RespSeg == respSegContratante && emptyStr(b.CNPJ) && emptyStr(b.CPF) {
		return problem.BadRequest("cnpj ou cpf é obrigatório quando o responsável pelo seguro é o contratante (resp_seg = 2)")
	}
	if emptyStr(b.XSeg) != emptyStr(b.CNPJSeg) {
		return problem.BadRequest("x_seg e cnpj_seg devem ser informados juntos")
	}
	return nil
}

// respSegContratante é o responsável pelo seguro = contratante do serviço.
const respSegContratante = "2"

func emptyStr(s *string) bool { return s == nil || *s == "" }

// CombOrigBody é uma origem do combustível (comb/origComb): de onde veio e em
// que proporção.
type CombOrigBody struct {
	// IndImport: 0 nacional, 1 importado.
	IndImport string `json:"ind_import" validate:"required,oneof=0 1"`
	// CUFOrig é o código IBGE da UF de origem (2 dígitos).
	CUFOrig string `json:"c_uf_orig" validate:"required,len=2,number"`
	POrig   string `json:"p_orig" validate:"required,percent"`
}

// ObsBody é um par campo/texto de infAdic (obsCont ou obsFisco).
type ObsBody struct {
	XCampo string `json:"x_campo" validate:"required,max=20"`
	XTexto string `json:"x_texto" validate:"required,max=60"`
}

// ProductTaxProfileRef liga um produto a um perfil fiscal, opcionalmente
// sobrescrevendo campos do perfil só para este produto. Um produto sem
// `tax_profiles` resolve a tributação exatamente como sempre resolveu.
//
// Precedência na emissão (spec §3.2), da maior para a menor:
// cfop_config[cfop] → overrides → perfil.
type ProductTaxProfileRef struct {
	TaxProfileID string `json:"tax_profile_id" validate:"required"`
	// Overrides é parcial de propósito: só as chaves presentes vencem o perfil.
	// Ele não é validado como um TaxFieldsBody completo justamente porque um
	// override de uma alíquota só não deveria exigir reenviar os outros 60 campos.
	Overrides map[string]any `json:"overrides" validate:"omitempty"`
}

// ── Tax profiles ─────────────────────────────────────────────────────────────

// TaxProfileBody is the body for POST/PUT /tax-profiles.
//
// A profile is one tax treatment applied to a set of CFOPs, named once and
// shared by many products, instead of ~60 fiscal fields copied into every
// product's cfop_config[]. 5102 and 6102 normally share a profile: the
// interstate rate is already derived by resolveICMSAliq, so what differs
// between them is derived data, not configuration. When the treatment genuinely
// differs per CFOP, create a second profile — there is no per-CFOP nesting
// inside a profile.
type TaxProfileBody struct {
	Name        string          `json:"name" validate:"required,min=2,max=120"`
	Description *string         `json:"description" validate:"omitempty,max=255"`
	Cfops       []string        `json:"cfops" validate:"required,min=1,dive,cfop"`
	UfOverrides []UfTaxOverride `json:"uf_overrides" validate:"omitempty,dive"`
	TaxFieldsBody
}

// ProductBody is the body for POST /products and PUT /products/:product_id.
// The frontend sends the full object on both create and update.
type ProductBody struct {
	Code        string  `json:"code" validate:"required,max=60,prodcode"`
	Description string  `json:"description" validate:"required,min=2,max=255"`
	Brand       *string `json:"brand" validate:"omitempty,max=60"`
	Ncm         string  `json:"ncm" validate:"required,ncm"`
	Cest        *string `json:"cest" validate:"omitempty,cest"`
	Origin      string  `json:"origin" validate:"required"`
	Unit        string  `json:"unit" validate:"required,max=6"`
	TaxableUnit *string `json:"taxable_unit" validate:"omitempty,max=6"`
	Cean        *string `json:"cean" validate:"omitempty,cean"`
	TaxableCean *string `json:"taxable_cean" validate:"omitempty,cean"`
	Value       string  `json:"value" validate:"required,money"`
	ValueResale *string `json:"value_resale" validate:"omitempty,money"`
	NetWeight   *string `json:"net_weight" validate:"omitempty,weight3"`
	GrossWeight *string `json:"gross_weight" validate:"omitempty,weight3"`
	// Campos fiscais do produto
	CBenef            *string                `json:"c_benef" validate:"omitempty,cbenef"`
	ExtIpi            *string                `json:"ext_ipi" validate:"omitempty,extipi"`
	IndEscala         *string                `json:"ind_escala" validate:"omitempty,oneof=S N"`
	CnpjFab           *string                `json:"cnpj_fab" validate:"omitempty,digits14"`
	IndTot            string                 `json:"ind_tot" validate:"required,oneof=0 1"`
	IcmsAliqOverride  *string                `json:"icms_aliq_override" validate:"omitempty,percent"`
	FcpAliqOverride   *string                `json:"fcp_aliq_override" validate:"omitempty,percent"`
	InfAdProd         *string                `json:"inf_ad_prod" validate:"omitempty,max=500"`
	CfopNfce          string                 `json:"cfop_nfce" validate:"required,cfop"`
	CfopConfig        []CfopConfigBody       `json:"cfop_config" validate:"required_without=TaxProfiles,omitempty,dive"`
	TaxProfiles       []ProductTaxProfileRef `json:"tax_profiles" validate:"required_without=CfopConfig,omitempty,dive"`
	ConversionFactors []ConversionFactorBody `json:"conversion_factors" validate:"omitempty,dive"`
	// Tipo específico e campos especiais
	ProdType     *string `json:"prod_type" validate:"omitempty,oneof=generic comb med veiculo arma"`
	CombCProdAnp *string `json:"comb_c_prod_anp" validate:"omitempty,digits9"`
	CombDescAnp  *string `json:"comb_desc_anp" validate:"omitempty,max=95"`
	CombUfCons   *string `json:"comb_uf_cons" validate:"omitempty,letters2"`
	CombCodif    *string `json:"comb_codif" validate:"omitempty,max=21"`
	CombPGlp     *string `json:"comb_p_glp" validate:"omitempty,percent"`
	CombPGnn     *string `json:"comb_p_gnn" validate:"omitempty,percent"`
	CombPGni     *string `json:"comb_p_gni" validate:"omitempty,percent"`
	CombVPart    *string `json:"comb_v_part" validate:"omitempty,money2"`
	CombPBio     *string `json:"comb_p_bio" validate:"omitempty,percent"`
	// CombCideVAliqProd é a alíquota da CIDE do produto. A base (qBCProd) é a
	// quantidade vendida e o vCIDE é o produto dos dois — nenhum dos dois é
	// digitado.
	CombCideVAliqProd *string `json:"comb_cide_v_aliq_prod" validate:"omitempty,money"`
	// CombOrig é a origem do combustível (prod/comb/origComb), até 30 entradas.
	CombOrig          []CombOrigBody `json:"comb_orig" validate:"omitempty,max=30,dive"`
	MedCProdAnvisa    *string        `json:"med_c_prod_anvisa" validate:"omitempty,min=5"`
	MedXMotivoIsencao *string        `json:"med_x_motivo_isencao" validate:"omitempty,max=255"`
	MedVPmc           *string        `json:"med_v_pmc" validate:"omitempty,money2"`
	// Classificação de produto perigoso (MDF-e peri). Cadastrada uma vez; o
	// MDF-e a encontra sozinho ao referenciar a NF-e que contém o item.
	// NVE, FCI e códigos de barra próprios — nível produto.
	// Reforma tributária no produto. GCred são os créditos presumidos da UF
	// aplicados ao item (máx. 4 pelo leiaute): código e percentual são
	// cadastrados; o valor é derivado do percentual sobre o valor do item.
	GCred []GCredBody `json:"gcred" validate:"omitempty,max=4,dive"`
	// TpCredPresIBSZFM é a classificação para subapuração do IBS na ZFM.
	TpCredPresIBSZFM *string `json:"tp_cred_pres_ibs_zfm" validate:"omitempty,oneof=0 1 2 3 4"`
	// IndBemMovelUsado marca fornecimento de bem móvel usado. O XSD enumera um
	// valor só: 1.
	IndBemMovelUsado *string `json:"ind_bem_movel_usado" validate:"omitempty,oneof=1"`
	// NRecopi é o número do RECOPI do papel imune (prod/nRECOPI). É do produto,
	// e o XSD o coloca no mesmo choice de comb/med/veicProd/arma.
	NRecopi    *string  `json:"n_recopi" validate:"omitempty,len=20,number"`
	Nve        []string `json:"nve" validate:"omitempty,max=8,dive,len=6"`
	NFci       *string  `json:"n_fci" validate:"omitempty,uuid"`
	CBarra     *string  `json:"c_barra" validate:"omitempty,max=30"`
	CBarraTrib *string  `json:"c_barra_trib" validate:"omitempty,max=30"`
	// Observação fiscal padrão do produto (det/obsItem).
	ObsItemXCampo *string `json:"obs_item_x_campo" validate:"omitempty,max=20"`
	ObsItemXTexto *string `json:"obs_item_x_texto" validate:"omitempty,max=60"`
	// Selo de controle do IPI e enquadramento legal — nível produto.
	IpiCnpjProd   *string `json:"ipi_cnpj_prod" validate:"omitempty,digits14"`
	IpiCSelo      *string `json:"ipi_c_selo" validate:"omitempty,max=60"`
	IpiQSelo      *string `json:"ipi_q_selo" validate:"omitempty,max=12,number"`
	IpiCEnq       *string `json:"ipi_c_enq" validate:"omitempty,max=3,number"`
	PeriNOnu      *string `json:"peri_n_onu" validate:"omitempty,max=4,number"`
	PeriXNomeAE   *string `json:"peri_x_nome_ae" validate:"omitempty,max=150"`
	PeriXClaRisco *string `json:"peri_x_cla_risco" validate:"omitempty,max=40"`
	PeriGrEmb     *string `json:"peri_gr_emb" validate:"omitempty,max=6"`
	PeriQVolTipo  *string `json:"peri_q_vol_tipo" validate:"omitempty,max=60"`
	// veicProd — dados do modelo
	VeicTpOp         *string `json:"veic_tp_op" validate:"omitempty,oneof=0 1 2 3"`
	VeicTpComb       *string `json:"veic_tp_comb" validate:"omitempty,max=2"`
	VeicTpPint       *string `json:"veic_tp_pint" validate:"omitempty,max=1"`
	VeicTpVeic       *string `json:"veic_tp_veic" validate:"omitempty,d12"`
	VeicEspVeic      *string `json:"veic_esp_veic" validate:"omitempty,d1"`
	VeicVin          *string `json:"veic_vin" validate:"omitempty,oneof=R N"`
	VeicCondVeic     *string `json:"veic_cond_veic" validate:"omitempty,oneof=1 2 3"`
	VeicCMod         *string `json:"veic_c_mod" validate:"omitempty,d16"`
	VeicCCorDenatran *string `json:"veic_c_cor_denatran" validate:"omitempty,d12"`
	VeicLota         *string `json:"veic_lota" validate:"omitempty,max=3"`
	VeicTpRest       *string `json:"veic_tp_rest" validate:"omitempty,oneof=0 1 2 3 4 9"`
	VeicAnoMod       *string `json:"veic_ano_mod" validate:"omitempty,d4"`
	VeicAnoFab       *string `json:"veic_ano_fab" validate:"omitempty,d4"`
	VeicPot          *string `json:"veic_pot" validate:"omitempty,max=4"`
	VeicCilin        *string `json:"veic_cilin" validate:"omitempty,max=4"`
	VeicCmt          *string `json:"veic_cmt" validate:"omitempty,max=9"`
	VeicDist         *string `json:"veic_dist" validate:"omitempty,max=4"`
	VeicCCor         *string `json:"veic_c_cor" validate:"omitempty,max=4"`
	VeicXCor         *string `json:"veic_x_cor" validate:"omitempty,max=40"`
	// arma
	ArmaTpArma *string `json:"arma_tp_arma" validate:"omitempty,oneof=0 1"`
	ArmaDescr  *string `json:"arma_descr" validate:"omitempty,max=256"`
}

// GCredBody é um crédito presumido da UF aplicado ao item (prod/gCred). O
// vCredPresumido não está aqui: é o percentual sobre o valor do item, calculado
// na emissão.
type GCredBody struct {
	// CCredPresumido é o código do benefício, de 8 ou 10 caracteres.
	CCredPresumido string `json:"c_cred_presumido" validate:"required,ccredpres"`
	PCredPresumido string `json:"p_cred_presumido" validate:"required,percent"`
}

// ── Serviços (catálogo NFS-e) ────────────────────────────────────────────────

// ServiceIssBody são os defaults de ISSQN do serviço (grupo tribMun do DPS).
// ServiceSchemaVersion é a versão dos subgrupos de organization_services. É
// gravada pelo servidor, nunca enviada pelo cliente: registro legado sem o
// campo continua legível e responde como versão 1, sem migração destrutiva.
const ServiceSchemaVersion = 2

// ServiceLocationDefaultsBody é o default de local de prestação do serviço.
// TCLocPrest é uma escolha exclusiva no XSD: município OU país, nunca os dois.
type ServiceLocationDefaultsBody struct {
	CLocPrestacao  *string `json:"c_loc_prestacao" validate:"omitempty,ibge"`
	CPaisPrestacao *string `json:"c_pais_prestacao" validate:"omitempty,len=2,alpha"`
}

// ServiceForeignTradeDefaultsBody são os defaults de comExt. O valor em moeda,
// a DI e o RE nascem na emissão e por isso não ficam aqui.
type ServiceForeignTradeDefaultsBody struct {
	MdPrestacao *string `json:"md_prestacao" validate:"omitempty,nfseenum=TSModoPrestacao"`
	VincPrest   *string `json:"vinc_prest" validate:"omitempty,nfseenum=TSVincPrest"`
	TpMoeda     *string `json:"tp_moeda" validate:"omitempty,len=3,number"`
	MecAFComexP *string `json:"mec_af_comex_p" validate:"omitempty,nfseenum=TSMecAFComExPrest"`
	MecAFComexT *string `json:"mec_af_comex_t" validate:"omitempty,nfseenum=TSMecAFComExToma"`
	MovTempBens *string `json:"mov_temp_bens" validate:"omitempty,nfseenum=TSMovTempBens"`
	Mdic        *string `json:"mdic" validate:"omitempty,nfseenum=TSEnvMDIC"`
}

// ServiceIssSuspensionBody é o grupo exigSusp: os dois campos são obrigatórios
// juntos no XSD, então o subobjeto inteiro é opcional e, presente, é completo.
type ServiceIssSuspensionBody struct {
	TpSusp    string `json:"tp_susp" validate:"required,nfseenum=TSOpExigSuspensa"`
	NProcesso string `json:"n_processo" validate:"required,len=30,number"`
}

// ServiceIssBenefitBody é o grupo BM (benefício municipal). A redução vem em
// valor OU percentual, conforme o tipo de benefício declarado pelo município.
type ServiceIssBenefitBody struct {
	NBM      string  `json:"n_bm" validate:"required,len=14,number"`
	VRedBCBM *string `json:"v_red_bc_bm" validate:"omitempty,money2"`
	PRedBCBM *string `json:"p_red_bc_bm" validate:"omitempty,percent"`
}

// ServiceIbsCbsRegularBody é gTribRegular: a tributação que existiria sem o
// benefício, exigida quando a operação usa CST desonerado.
type ServiceIbsCbsRegularBody struct {
	CSTReg        string `json:"cst_reg" validate:"required,len=3,number"`
	CClassTribReg string `json:"c_class_trib_reg" validate:"required,class6"`
}

// ServiceIbsCbsDifBody é gDif: os três percentuais de diferimento são
// obrigatórios juntos no XSD.
type ServiceIbsCbsDifBody struct {
	PDifUF  string `json:"p_dif_uf" validate:"required,percent"`
	PDifMun string `json:"p_dif_mun" validate:"required,percent"`
	PDifCBS string `json:"p_dif_cbs" validate:"required,percent"`
}

// ServiceRequirementsBody declara o que este serviço exige ou permite na
// emissão. São flags de UX e de validação, não campos do XML: a emissão usa
// para decidir quais grupos pedir em vez de mostrar todos sempre.
type ServiceRequirementsBody struct {
	RequiresWork         bool `json:"requires_work"`
	RequiresEvent        bool `json:"requires_event"`
	AllowsDeductions     bool `json:"allows_deductions"`
	AllowsReimbursements bool `json:"allows_reimbursements"`
}

type ServiceIssBody struct {
	// 1 operação tributável | 2 imunidade | 3 exportação de serviço | 4 não incidência
	TribISSQN int    `json:"trib_issqn" validate:"required,oneof=1 2 3 4"`
	TaxRate   string `json:"tax_rate" validate:"required,percent"`
	// 1 não retido | 2 retido pelo tomador | 3 retido pelo intermediário
	TpRetISSQN *int `json:"tp_ret_issqn" validate:"omitempty,oneof=1 2 3"`
	// Somente para trib_issqn=2 (imunidade). 0 tipo não informado | 1-5 hipóteses
	// constitucionais específicas (CF88 Art 150, VI) — ver TSTipoImunidadeISSQN.
	TpImunidade    *int    `json:"tp_imunidade" validate:"omitempty,gte=0,lte=5"`
	CPaisResultado *string `json:"c_pais_resultado" validate:"omitempty,len=2,alpha"`
	// Grupos opcionais do XSD; presentes, vêm completos.
	ExigSusp *ServiceIssSuspensionBody `json:"exig_susp" validate:"omitempty"`
	BM       *ServiceIssBenefitBody    `json:"bm" validate:"omitempty"`
}

// ServiceFederalBody são os defaults de tributos federais do serviço.
type ServiceFederalBody struct {
	CstPisCofins *string `json:"cst_pis_cofins" validate:"omitempty,len=2,number"`
	AliqPis      *string `json:"aliq_pis" validate:"omitempty,percent"`
	AliqCofins   *string `json:"aliq_cofins" validate:"omitempty,percent"`
	// 0 nenhum retido | 1 PIS/COFINS retidos | 2 PIS/COFINS não retidos
	// 3 PIS/COFINS/CSLL retidos | 4 PIS/COFINS retidos, CSLL não | 5 PIS retido, COFINS/CSLL não
	// 6 COFINS retido, PIS/CSLL não | 7 PIS não retido, COFINS/CSLL retidos
	// 8 PIS/COFINS não retidos, CSLL retido | 9 COFINS não retido, PIS/CSLL retidos
	TpRetPisCofins *int    `json:"tp_ret_pis_cofins" validate:"omitempty,oneof=0 1 2 3 4 5 6 7 8 9"`
	VRetCP         *string `json:"v_ret_cp" validate:"omitempty,money2"`
	VRetIRRF       *string `json:"v_ret_irrf" validate:"omitempty,money2"`
	VRetCSLL       *string `json:"v_ret_csll" validate:"omitempty,money2"`
}

// ServiceIbsCbsBody são os defaults de IBS/CBS do serviço (reforma tributária).
type ServiceIbsCbsBody struct {
	CIndOp     *string `json:"c_ind_op" validate:"required,indop"`
	Cst        *string `json:"cst" validate:"required,len=3,number"`
	CClassTrib *string `json:"c_class_trib" validate:"required,class6"`
	// 0 destinatário é o próprio tomador | 1 destinatário diferente do tomador
	IndDest *int `json:"ind_dest" validate:"required,oneof=0 1"`
	// 1 fornecimento com pagamento posterior | 2 recebimento com fornecimento já realizado
	// 3 fornecimento com pagamento já realizado | 4 recebimento com fornecimento posterior
	// 5 fornecimento e recebimento concomitantes
	TpOper *int `json:"tp_oper" validate:"omitempty,oneof=1 2 3 4 5"`
	// Valor fixo — TSRTCFinNFSe só admite 0 (NFS-e regular).
	FinNFSe *int `json:"fin_nfse" validate:"required,oneof=0"`
	// 0 consumidor não final | 1 consumidor final
	IndFinal *string `json:"ind_final" validate:"omitempty,nfseenum=TSRTCIndFinal"`
	// Ente governamental adquirente: só é preenchido em compra governamental.
	TpEnteGov *string `json:"tp_ente_gov" validate:"omitempty,nfseenum=TSRTCTpEnteGov"`
	// cCredPres tem 2 dígitos (TSRTCCodCredPres), não 6 como cClassTrib.
	CCredPres   *string                   `json:"c_cred_pres" validate:"omitempty,digits2"`
	TribRegular *ServiceIbsCbsRegularBody `json:"trib_regular" validate:"omitempty"`
	Dif         *ServiceIbsCbsDifBody     `json:"dif" validate:"omitempty"`
}

// ServiceTotTribBody é a Lei da Transparência (grupo totTrib do DPS).
type ServiceTotTribBody struct {
	// Valor fixo — TSTipoIndTotTrib só admite 0 (Decreto 8.264/2014 veda estimar
	// tributos na NFS-e).
	IndTotTrib int     `json:"ind_tot_trib" validate:"oneof=0"`
	PTotTribSN *string `json:"p_tot_trib_sn" validate:"omitempty,percent"`
}

// ServiceBody is the body for POST /services and PUT /services/:service_id.
// O frontend envia o objeto completo em ambos.
type ServiceBody struct {
	Code              string              `json:"code" validate:"required,max=60"`
	Description       string              `json:"description" validate:"required,min=2,max=2000"`
	TribNacionalCode  string              `json:"trib_nacional_code" validate:"required,tribnac"`
	TribMunicipalCode *string             `json:"trib_municipal_code" validate:"omitempty,max=20"`
	NbsCode           *string             `json:"nbs_code" validate:"omitempty,nbs"`
	Cnae              *string             `json:"cnae" validate:"omitempty,cnae"`
	Unit              string              `json:"unit" validate:"required,unit"`
	Value             string              `json:"value" validate:"required,money"`
	Iss               ServiceIssBody      `json:"iss" validate:"required"`
	Federal           *ServiceFederalBody `json:"federal" validate:"omitempty"`
	IbsCbs            *ServiceIbsCbsBody  `json:"ibs_cbs" validate:"required"`
	TotTrib           *ServiceTotTribBody `json:"tot_trib" validate:"omitempty"`

	LocationDefaults     *ServiceLocationDefaultsBody     `json:"location_defaults" validate:"omitempty"`
	ForeignTradeDefaults *ServiceForeignTradeDefaultsBody `json:"foreign_trade_defaults" validate:"omitempty"`
	Requirements         *ServiceRequirementsBody         `json:"requirements" validate:"omitempty"`
}

// ── Config NFS-e ─────────────────────────────────────────────────────────────

// NfseAbrasfBody é a configuração específica do provider abrasf204.
type NfseAbrasfBody struct {
	EndpointURL      string `json:"endpoint_url" validate:"required,url"`
	WsdlVersion      string `json:"wsdl_version" validate:"required,max=10"`
	MunicipalityCode string `json:"municipality_code" validate:"required,ibge"`
	Synchronous      bool   `json:"synchronous"`
}

// NfseConfigBody is the body for PUT /organizations/:org_pk/nfse-config.
// Inscrição municipal e regime tributário do prestador NÃO ficam aqui — vêm do
// grupo `nfse` do objeto person da própria organização, porque quando ela emite
// como tomador ou intermediário o prestador é outra pessoa. Ver
// docs/specs/2026-08-04-nfse-design.md §3.2 e §3.3.
//
// Não embeda fiscalConfigBase (usado por FiscalConfigBody/NfceConfigBody):
// a NFS-e tem uma única `serie` (não uma por ambiente), mas compartilha a
// validação de timezone usada pelos demais documentos fiscais.
type NfseConfigBody struct {
	Provider          string          `json:"provider" validate:"required,oneof=nacional abrasf204"`
	Environment       int             `json:"environment" validate:"required,oneof=1 2"`
	Timezone          string          `json:"timezone" validate:"required,timezone"`
	CLocEmi           string          `json:"c_loc_emi" validate:"required,ibge"`
	Serie             string          `json:"serie" validate:"required,max=5,number"`
	ProdCurrentNumber int             `json:"prod_current_number" validate:"gte=0"`
	HomCurrentNumber  int             `json:"hom_current_number" validate:"gte=0"`
	CertificateSK     *string         `json:"certificate_sk" validate:"omitempty,max=60"`
	Abrasf            *NfseAbrasfBody `json:"abrasf" validate:"omitempty"`
}

// ── Fiscal event actions (NF-e / NFC-e / MDF-e) ──────────────────────

// CancelEventBody is the payload for POST …/:access_key/cancel, shared
// across NF-e / NFC-e / MDF-e. Justification is required (min 15);
// SequenceNumber is optional — the service applies its defaultSeq when zero.
type CancelEventBody struct {
	Justification  string `json:"justification" validate:"required,min=15,max=255"`
	SequenceNumber int    `json:"sequence_number" validate:"omitempty,gte=1"`
}

// SubstituteCancelBody extends CancelEventBody with the key of the
// NF-e that substitutes the cancelled one (event 110112).
type SubstituteCancelBody struct {
	CancelEventBody
	SubstituteKey string `json:"substitute_key" validate:"required,len=44,numeric"`
}

// ── Vehicles ─────────────────────────────────────────────────────────────────

// VehicleOwnerBody is the owner (proprietário) of a vehicle.
//
// Serve de default para o grupo veicTracao/prop do MDF-e quando a emissão não
// traz um proprietário explícito (mdfes.MdfeOwner, que continua vencendo).
// Proprietário cadastrado igual ao emitente significa frota própria: nesse caso
// nenhum grupo prop é gerado, e ide/tpTransp fica exatamente como sempre ficou.
type VehicleOwnerBody struct {
	CpfCnpj string `json:"cpf_cnpj" validate:"required,cpfcnpj"`
	Rntrc   string `json:"rntrc" validate:"required,rntrc"`
	Name    string `json:"name" validate:"required,min=2,max=255"`
	Type    string `json:"type" validate:"required,oneof=TAC ETC CTC"`
}

// VehicleCreateBody is the body for POST /vehicles. Only Plate, PlateUf and
// Role are required — everything else is completed later, gated at
// emission time per doc-type/role (see services.Missing).
type VehicleCreateBody struct {
	Plate    string            `json:"plate" validate:"required,placa"`
	PlateUf  string            `json:"plate_uf" validate:"required,uf"`
	Role     string            `json:"role" validate:"required,oneof=tractor trailer"`
	Wheelset string            `json:"wheelset" validate:"omitempty"`
	Bodywork string            `json:"bodywork" validate:"omitempty"`
	Renavam  string            `json:"renavam" validate:"omitempty,renavam"`
	Weight   int               `json:"weight" validate:"omitempty,gte=0"`
	CapKG    int               `json:"cap_kg" validate:"omitempty,gte=0"`
	CapM3    int               `json:"cap_m3" validate:"omitempty,gte=0"`
	Cint     string            `json:"cint" validate:"omitempty,max=10"`
	Owner    *VehicleOwnerBody `json:"owner" validate:"omitempty"`
}

// VehicleUpdateBody is the body for PUT /vehicles/:sk (partial).
type VehicleUpdateBody struct {
	Plate    *string           `json:"plate" validate:"omitempty,placa"`
	PlateUf  *string           `json:"plate_uf" validate:"omitempty,uf"`
	Role     *string           `json:"role" validate:"omitempty,oneof=tractor trailer"`
	Wheelset *string           `json:"wheelset" validate:"omitempty"`
	Bodywork *string           `json:"bodywork" validate:"omitempty"`
	Renavam  *string           `json:"renavam" validate:"omitempty,renavam"`
	Weight   *int              `json:"weight" validate:"omitempty,gte=0"`
	CapKG    *int              `json:"cap_kg" validate:"omitempty,gte=0"`
	CapM3    *int              `json:"cap_m3" validate:"omitempty,gte=0"`
	Cint     *string           `json:"cint" validate:"omitempty,max=10"`
	Owner    *VehicleOwnerBody `json:"owner" validate:"omitempty"`
}

// ── Fiscal configs ───────────────────────────────────────────────────────────

// fiscalConfigBase holds the fields common to all four fiscal config variants.
type fiscalConfigBase struct {
	Timezone          string `json:"timezone" validate:"required,timezone"`
	Environment       int    `json:"environment" validate:"required,oneof=1 2"`
	ProdCurrentSerie  int    `json:"prod_current_serie" validate:"gte=0"`
	ProdCurrentNumber int    `json:"prod_current_number" validate:"gte=0"`
	HomCurrentSerie   int    `json:"hom_current_serie" validate:"gte=0"`
	HomCurrentNumber  int    `json:"hom_current_number" validate:"gte=0"`

	// CSRT do responsável técnico (NT 2018.005). Segredo: a API nunca o devolve
	// — ver redactFiscalSecrets em helpers.go.
	CsrtID *string `json:"csrt_id" validate:"omitempty,max=2,number"`
	Csrt   *string `json:"csrt" validate:"omitempty,len=36"`
}

// FiscalConfigBody is the body for PUT /…/nfe-config, cte-config, mdfe-config.
type FiscalConfigBody struct {
	fiscalConfigBase
}

// MdfeConfigBody is the body for PUT /…/mdfe-config. Os três campos extras
// são do leiaute do MDF-e e não existem na NF-e/CT-e, por isso um body próprio
// em vez de poluir o FiscalConfigBody compartilhado.
type MdfeConfigBody struct {
	fiscalConfigBase
	// IndCanalVerde — participação da organização no Canal Verde (ide/indCanalVerde).
	IndCanalVerde bool `json:"ind_canal_verde"`
	// IndCarregaPosterior — a organização inclui DF-e por evento depois de
	// emitir o manifesto (ide/indCarregaPosterior).
	IndCarregaPosterior bool `json:"ind_carrega_posterior"`
	// InfAdFisco — mensagem de interesse do fisco repetida em toda emissão
	// (infAdic/infAdFisco). A observação da viagem continua no corpo da emissão.
	InfAdFisco *string `json:"inf_ad_fisco" validate:"omitempty,max=2000"`
}

// NfceConfigBody is the body for PUT /…/nfce-config (adds CSC fields).
type NfceConfigBody struct {
	fiscalConfigBase
	ProdCsc   string `json:"prod_csc" validate:"required,max=36"`
	ProdCscID int    `json:"prod_csc_id" validate:"required,gt=0"`
	HomCsc    string `json:"hom_csc" validate:"required,max=36"`
	HomCscID  int    `json:"hom_csc_id" validate:"required,gt=0"`
}

func init() {
	validation.RegisterStructRule(validateIbsCbsGroup, TaxFieldsBody{})
	validation.RegisterStructRule(validateServiceLocationDefaults, ServiceLocationDefaultsBody{})
	validation.RegisterStructRule(validateServiceIssBenefit, ServiceIssBenefitBody{})
	validation.RegisterStructRule(validateOperationNfseLocation, OperationNfseBody{})
	validation.RegisterStructRule(validateServiceLocationAddress, ServiceLocationAddressBody{})
	validation.RegisterStructRule(validateServiceLocationRoles, ServiceLocationBody{})
	validation.RegisterStructRule(validateReferenceDocumentUnion, ReferenceDocumentBody{})
}

// validateServiceLocationAddress exige um endereço nacional OU um do exterior,
// nunca os dois nem nenhum: TCEnderecoSimples é a mesma escolha, e um endereço
// pela metade só falharia na emissão, longe de quem digitou.
func validateServiceLocationAddress(sl validator.StructLevel) {
	f := sl.Current().Interface().(ServiceLocationAddressBody)
	national := f.PostalCode != nil && *f.PostalCode != ""
	foreign := f.ForeignPostalCode != nil && *f.ForeignPostalCode != ""
	switch {
	case national && foreign:
		sl.ReportError(f.ForeignPostalCode, "foreign_postal_code", "ForeignPostalCode", "excluded_with", "postal_code")
	case !national && !foreign:
		sl.ReportError(f.PostalCode, "postal_code", "PostalCode", "required_without", "foreign_postal_code")
	case foreign:
		if f.ForeignCity == nil || *f.ForeignCity == "" {
			sl.ReportError(f.ForeignCity, "foreign_city", "ForeignCity", "required_with", "foreign_postal_code")
		}
		if f.ForeignRegion == nil || *f.ForeignRegion == "" {
			sl.ReportError(f.ForeignRegion, "foreign_region", "ForeignRegion", "required_with", "foreign_postal_code")
		}
	case national:
		if f.CityIBGECode == nil || *f.CityIBGECode == "" {
			sl.ReportError(f.CityIBGECode, "city_ibge_code", "CityIBGECode", "required_with", "postal_code")
		}
	}
}

// validateServiceLocationRoles impede guardar `c_obra` e `cib` no mesmo local.
// `serv/obra` é a escolha cObra|cCIB|end: com os dois gravados, a emissão
// decidiria em silêncio qual ramo gerar. O endereço é sempre obrigatório aqui,
// então o terceiro ramo da escolha nunca falta.
func validateServiceLocationRoles(sl validator.StructLevel) {
	f := sl.Current().Interface().(ServiceLocationBody)
	if f.CIB != nil && *f.CIB != "" && f.CObra != nil && *f.CObra != "" {
		sl.ReportError(f.CIB, "cib", "CIB", "excluded_with", "c_obra")
	}
}

// referenceDocumentSubobjects mapeia cada família ao seu subobjeto. Manter o
// mapa aqui é o que garante que uma família nova não passe sem regra.
func referenceDocumentSubobjects(f ReferenceDocumentBody) map[string]any {
	return map[string]any{
		ReferenceDocumentKindDFe:       f.DFe,
		ReferenceDocumentKindNFSeMun:   f.NFSeMun,
		ReferenceDocumentKindNFNFS:     f.NFNFS,
		ReferenceDocumentKindTaxOther:  f.TaxOther,
		ReferenceDocumentKindNonFiscal: f.NonFiscal,
	}
}

// validateReferenceDocumentUnion exige exatamente o subobjeto de `kind` e
// nenhum outro.
func validateReferenceDocumentUnion(sl validator.StructLevel) {
	f := sl.Current().Interface().(ReferenceDocumentBody)
	for kind, value := range referenceDocumentSubobjects(f) {
		present := !isNilSubobject(value)
		switch {
		case kind == f.Kind && !present:
			sl.ReportError(value, kind, kind, "required_with", "kind")
		case kind != f.Kind && present:
			sl.ReportError(value, kind, kind, "excluded_with", "kind")
		}
	}
}

// isNilSubobject trata o ponteiro tipado nulo guardado numa interface, que
// nunca é igual a nil por comparação direta.
func isNilSubobject(value any) bool {
	switch typed := value.(type) {
	case *ReferenceDocumentDFeBody:
		return typed == nil
	case *ReferenceDocumentNFSeMunBody:
		return typed == nil
	case *ReferenceDocumentNFNFSBody:
		return typed == nil
	case *ReferenceDocumentTaxOtherBody:
		return typed == nil
	case *ReferenceDocumentNonFiscalBody:
		return typed == nil
	default:
		return value == nil
	}
}

// validateOperationNfseLocation repete na operação a escolha exclusiva de
// TCLocPrest. A regra é a mesma do serviço, mas os campos são outros: um
// struct rule por tipo é o que o validator suporta.
func validateOperationNfseLocation(sl validator.StructLevel) {
	f := sl.Current().Interface().(OperationNfseBody)
	city, country := f.CLocPrestacao != nil && *f.CLocPrestacao != "", f.CPaisPrestacao != nil && *f.CPaisPrestacao != ""
	if city && country {
		sl.ReportError(f.CPaisPrestacao, "c_pais_prestacao", "CPaisPrestacao", "excluded_with", "c_loc_prestacao")
	}
}

// validateServiceLocationDefaults enforces TCLocPrest as the exclusive choice
// it is in the XSD: município OU país. Aceitar os dois deixaria a emissão
// escolher em silêncio, e a DPS sairia com o local errado.
func validateServiceLocationDefaults(sl validator.StructLevel) {
	f := sl.Current().Interface().(ServiceLocationDefaultsBody)
	city, country := f.CLocPrestacao != nil && *f.CLocPrestacao != "", f.CPaisPrestacao != nil && *f.CPaisPrestacao != ""
	if city && country {
		sl.ReportError(f.CPaisPrestacao, "c_pais_prestacao", "CPaisPrestacao", "excluded_with", "c_loc_prestacao")
	}
}

// validateServiceIssBenefit exige exatamente uma forma de redução: valor ou
// percentual. Nenhuma das duas torna o benefício inócuo; as duas juntas não
// têm regra de precedência no leiaute.
func validateServiceIssBenefit(sl validator.StructLevel) {
	f := sl.Current().Interface().(ServiceIssBenefitBody)
	value, percent := f.VRedBCBM != nil && *f.VRedBCBM != "", f.PRedBCBM != nil && *f.PRedBCBM != ""
	switch {
	case value && percent:
		sl.ReportError(f.PRedBCBM, "p_red_bc_bm", "PRedBCBM", "excluded_with", "v_red_bc_bm")
	case !value && !percent:
		sl.ReportError(f.VRedBCBM, "v_red_bc_bm", "VRedBCBM", "required_without", "p_red_bc_bm")
	}
}

// validateIbsCbsGroup enforces the IBS/CBS group as all-or-nothing: if any of
// the 5 key fields is filled, the other 4 are required too. A product with
// NONE of them is valid — the group is simply omitted at emission time
// (design spec 2026-08-09-tax-config-redesign §Modelo de dados 4).
func validateIbsCbsGroup(sl validator.StructLevel) {
	f := sl.Current().Interface().(TaxFieldsBody)
	vals := []*string{f.IbsCbsCst, f.IbsCbsClassTrib, f.IbsUfAliq, f.IbsMunAliq, f.CbsAliq}
	present := 0
	for _, v := range vals {
		if v != nil && *v != "" {
			present++
		}
	}
	if present == 0 || present == len(vals) {
		return
	}
	jsonNames := []string{"ibs_cbs_cst", "ibs_cbs_class_trib", "ibs_uf_aliq", "ibs_mun_aliq", "cbs_aliq"}
	structNames := []string{"IbsCbsCst", "IbsCbsClassTrib", "IbsUfAliq", "IbsMunAliq", "CbsAliq"}
	for i, v := range vals {
		if v == nil || *v == "" {
			sl.ReportError(v, jsonNames[i], structNames[i], "required_with_group", "ibs_cbs")
		}
	}
}
