package v1

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
}

// ── Persons ──────────────────────────────────────────────────────────────────

// personRolesValidation is the shared `validate` tag for the person role list.
// The accepted values mirror services.AllPersonRoles; TestPersonRolesTagMatchesAllPersonRoles
// fails if the two drift apart.
const personRolesValidation = "omitempty,dive,oneof=customer supplier carrier driver provider"

// PersonCreateBody is the body for POST /persons.
//
// Roles is a registry filter (customer/supplier/carrier/driver/provider), not a
// fiscal rule — a person may hold several at once, and an absent list is valid:
// that person simply never shows up in a role-filtered listing.
type PersonCreateBody struct {
	CpfOrCnpj string           `json:"cpf_or_cnpj" validate:"required,cpfcnpj"`
	Name      string           `json:"name" validate:"required,min=2,max=255"`
	Roles     []string         `json:"roles" validate:"omitempty,dive,oneof=customer supplier carrier driver provider"`
	Person    PersonObjectBody `json:"person" validate:"required"`
}

// PersonUpdateBody is the body for PUT /persons/:cpf_cnpj (partial; the document
// is taken from the path, never the body).
type PersonUpdateBody struct {
	Name   *string           `json:"name" validate:"omitempty,min=2,max=255"`
	Roles  []string          `json:"roles" validate:"omitempty,dive,oneof=customer supplier carrier driver provider"`
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
	IcmsPRedBc *string `json:"icms_p_red_bc" validate:"omitempty,percent"`
	IcmsMotDes *string `json:"icms_mot_des" validate:"omitempty"`
	IcmsPDif   *string `json:"icms_p_dif" validate:"omitempty,percent"`
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
	// PIS / COFINS
	Pis            string  `json:"pis" validate:"required,digits2"`
	Cofins         string  `json:"cofins" validate:"required,digits2"`
	PisAliq        *string `json:"pis_aliq" validate:"omitempty,percent"`
	CofinsAliq     *string `json:"cofins_aliq" validate:"omitempty,percent"`
	PisAliqUnid    *string `json:"pis_aliq_unid" validate:"omitempty,percent"`
	CofinsAliqUnid *string `json:"cofins_aliq_unid" validate:"omitempty,percent"`
	// IBS / CBS (Reforma Tributária) — required
	IbsCbsCst       string  `json:"ibs_cbs_cst" validate:"required,ibscst"`
	IbsCbsClassTrib string  `json:"ibs_cbs_class_trib" validate:"required,class6"`
	IbsUfAliq       string  `json:"ibs_uf_aliq" validate:"required,percent"`
	IbsMunAliq      string  `json:"ibs_mun_aliq" validate:"required,percent"`
	CbsAliq         string  `json:"cbs_aliq" validate:"required,percent"`
	IbsUfPRed       *string `json:"ibs_uf_p_red" validate:"omitempty,percent"`
	IbsMunPRed      *string `json:"ibs_mun_p_red" validate:"omitempty,percent"`
	CbsPRed         *string `json:"cbs_p_red" validate:"omitempty,percent"`
	IbsUfPDif       *string `json:"ibs_uf_p_dif" validate:"omitempty,percent"`
	IbsMunPDif      *string `json:"ibs_mun_p_dif" validate:"omitempty,percent"`
	CbsPDif         *string `json:"cbs_p_dif" validate:"omitempty,percent"`
	IbsIndDoacao    *string `json:"ibs_ind_doacao" validate:"omitempty"`
	IbsAdRem        *string `json:"ibs_ad_rem" validate:"omitempty,percent"`
	CbsAdRem        *string `json:"cbs_ad_rem" validate:"omitempty,percent"`
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
}

// CfopConfigBody is one per-CFOP tax configuration entry of a product.
// Optional tax fields are nullable and only format-checked when present.
type CfopConfigBody struct {
	Cfop string `json:"cfop" validate:"required,cfop"`
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
	DocTypes []string `json:"doc_types" validate:"omitempty,dive,oneof=nfe nfce cte mdfe"`

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

	// Aceitam placeholders {{chave}} — ver services.AllPlaceholders. Chave
	// desconhecida é erro aqui, no cadastro, nunca silêncio no XML.
	InfAdFisco *string `json:"inf_ad_fisco" validate:"omitempty,max=2000"`
	InfCpl     *string `json:"inf_cpl" validate:"omitempty,max=5000"`

	// RequiresReceiver falso habilita emissão sem destinatário (self_issuance).
	RequiresReceiver *bool `json:"requires_receiver" validate:"omitempty"`
	// IsDefault marca a operação pré-selecionada da organização. Só uma pode
	// estar marcada; marcar uma desmarca a anterior no mesmo TransactWrite.
	IsDefault bool `json:"is_default"`
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
	Name        string   `json:"name" validate:"required,min=2,max=120"`
	Description *string  `json:"description" validate:"omitempty,max=255"`
	Cfops       []string `json:"cfops" validate:"required,min=1,dive,cfop"`
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
	CfopConfig        []CfopConfigBody       `json:"cfop_config" validate:"required,min=1,dive"`
	TaxProfiles       []ProductTaxProfileRef `json:"tax_profiles" validate:"omitempty,dive"`
	ConversionFactors []ConversionFactorBody `json:"conversion_factors" validate:"omitempty,dive"`
	// Tipo específico e campos especiais
	ProdType          *string `json:"prod_type" validate:"omitempty,oneof=generic comb med veiculo arma"`
	CombCProdAnp      *string `json:"comb_c_prod_anp" validate:"omitempty,digits9"`
	CombDescAnp       *string `json:"comb_desc_anp" validate:"omitempty,max=95"`
	CombUfCons        *string `json:"comb_uf_cons" validate:"omitempty,letters2"`
	CombCodif         *string `json:"comb_codif" validate:"omitempty,max=21"`
	CombPGlp          *string `json:"comb_p_glp" validate:"omitempty,percent"`
	CombPGnn          *string `json:"comb_p_gnn" validate:"omitempty,percent"`
	CombPGni          *string `json:"comb_p_gni" validate:"omitempty,percent"`
	CombVPart         *string `json:"comb_v_part" validate:"omitempty,money2"`
	CombPBio          *string `json:"comb_p_bio" validate:"omitempty,percent"`
	MedCProdAnvisa    *string `json:"med_c_prod_anvisa" validate:"omitempty,min=5"`
	MedXMotivoIsencao *string `json:"med_x_motivo_isencao" validate:"omitempty,max=255"`
	MedVPmc           *string `json:"med_v_pmc" validate:"omitempty,money2"`
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

// ── Serviços (catálogo NFS-e) ────────────────────────────────────────────────

// ServiceIssBody são os defaults de ISSQN do serviço (grupo tribMun do DPS).
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
}

// FiscalConfigBody is the body for PUT /…/nfe-config, cte-config, mdfe-config.
type FiscalConfigBody struct {
	fiscalConfigBase
}

// NfceConfigBody is the body for PUT /…/nfce-config (adds CSC fields).
type NfceConfigBody struct {
	fiscalConfigBase
	ProdCsc   string `json:"prod_csc" validate:"required,max=36"`
	ProdCscID int    `json:"prod_csc_id" validate:"required,gt=0"`
	HomCsc    string `json:"hom_csc" validate:"required,max=36"`
	HomCscID  int    `json:"hom_csc_id" validate:"required,gt=0"`
}
