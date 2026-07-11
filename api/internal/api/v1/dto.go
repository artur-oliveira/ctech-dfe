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

// PersonObjectBody is the nested `person` object shared by person and
// organization payloads. crt is sent as a number (1–4) or null.
type PersonObjectBody struct {
	FantasyName        *string                 `json:"fantasy_name" validate:"omitempty,max=255"`
	Crt                *int                    `json:"crt" validate:"omitempty,oneof=1 2 3 4"`
	StateRegistrations []StateRegistrationBody `json:"state_registrations" validate:"omitempty,dive"`
	Addresses          []AddressBody           `json:"addresses" validate:"required,min=1,dive"`
	Contacts           *ContactsBody           `json:"contacts" validate:"omitempty"`
}

// ── Persons ──────────────────────────────────────────────────────────────────

// PersonCreateBody is the body for POST /persons.
type PersonCreateBody struct {
	CpfOrCnpj string           `json:"cpf_or_cnpj" validate:"required,cpfcnpj"`
	Name      string           `json:"name" validate:"required,min=2,max=255"`
	Person    PersonObjectBody `json:"person" validate:"required"`
}

// PersonUpdateBody is the body for PUT /persons/:cpf_cnpj (partial; the document
// is taken from the path, never the body).
type PersonUpdateBody struct {
	Name   *string           `json:"name" validate:"omitempty,min=2,max=255"`
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

// ── Products ─────────────────────────────────────────────────────────────────

// ConversionFactorBody is a unit-conversion factor for a product.
type ConversionFactorBody struct {
	OriginUnit string  `json:"origin_unit" validate:"required,unit"`
	TargetUnit string  `json:"target_unit" validate:"required,unit"`
	Factor     float64 `json:"factor" validate:"required,gt=0"`
}

// CfopConfigBody is one per-CFOP tax configuration entry of a product.
// Optional tax fields are nullable and only format-checked when present.
type CfopConfigBody struct {
	Cfop string `json:"cfop" validate:"required,cfop"`
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

// ── Vehicles ─────────────────────────────────────────────────────────────────

// VehicleOwnerBody is the owner (proprietário) of a vehicle. Optional static
// metadata — not used for MDF-e prop building (that's a per-emission input,
// see mdfes.MdfeOwner); kept only as informational fleet-management data.
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
