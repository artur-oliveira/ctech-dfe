import type {DfeStatus} from '@/lib/data/dfe_status'
import type {PersonRole} from '@/lib/schemas/entity'

// Auth
export interface TokenResponse {
  access_token: string
  token_type: string
}

export interface RoleOut {
  name: string
  description: string
}

export interface MeResponse {
  user_id: string
  username: string
  email: string
  first_name: string
  last_name: string
  email_verified: boolean
  is_enabled: boolean
  last_login_at: string | null
  organizations: UserOrganization[]
  terms_addendum_accepted: boolean
}

export interface UserOrganization {
  pk: string
  name: string
  description: string | null
  role: string
  permissions: string[]
  state_federation: string | null
}

// Members & invitations
export interface MemberOut {
  user_id: string
  /** Display-only name snapshot taken when the membership was granted. */
  name: string
  role: string
  permissions: string[]
  invited_by: string
  created_at: string
}

export interface InvitationOut {
  pk: string
  org_pk: string
  role: string
  status: string
  invited_by_name: string
  expires_at: string
  created_at: string
  /** Present only in the create response — the raw single-use token. */
  token?: string
}

export interface InvitationPreview {
  org_pk: string
  org_name: string
  role: string
  invited_by_name: string
  status: string
  expired: boolean
  already_member: boolean
}

export interface LookupAddressOut {
  street: string | null
  number: string | null
  complement: string | null
  neighborhood: string | null
  city: string | null
  postal_code: string | null
  state_federation: string | null
  city_ibge_code: string | null
}

export interface LookupStateRegistrationOut {
  uf: string
  state_registration: string
}

export interface LookupOrganizationOut {
  cpf_cnpj: string
  name: string
  crt: string | number | null;
  uf: string
  status: string
  addresses: LookupAddressOut[]
  state_registrations: LookupStateRegistrationOut[]
}

// Organizations
export interface AddressOut {
  city_ibge_code: string
  street: string
  neighborhood: string
  number: string
  city: string
  state_federation: string
  postal_code: string
  complement?: string
}

export interface StateRegistrationOut {
  uf: string
  state_registration: string
}

/** Grupo `nfse` do cadastro (NfseInfoBody em api/internal/api/v1/dto.go) —
 *  inscrição municipal e regime tributário exigidos pela emissão de NFS-e. */
export interface NfseRegTrib {
  op_simp_nac: number
  reg_ap_trib_sn?: number | null
  reg_esp_trib: number
}

export interface NfseInfo {
  im?: string | null
  caepf?: string | null
  nif?: string | null
  c_nao_nif?: number | null
  reg_trib?: NfseRegTrib | null
}

export interface PersonOut {
  fantasy_name: string
  crt: string | number;
  state_registrations: StateRegistrationOut[]
  addresses: AddressOut[]
  contacts: ContactsOut
  nfse?: NfseInfo | null
}

export interface ContactsOut {
  emails: string[]
  phones: string[]
}

export interface AuthorizedViewerOut {
  cpf_cnpj: string
  name: string
}

export interface OrganizationOut {
  pk: string
  name: string
  description: string
  person: PersonOut
  created_at: string
  updated_at: string
  /** Locais de retirada salvos de emissões de NF-e anteriores (org = remetente). */
  pickup_locations?: NfeLocalOut[]
  /** Pessoas autorizadas a ver o XML das NF-e desta organização (SEFAZ autXML). */
  authorized_xml_viewers?: AuthorizedViewerOut[]
}

export interface OrganizationCreate {
  cpf_or_cnpj: string
  name: string
  description?: string
  person: PersonObject
}

export interface OrganizationUpdate {
  name?: string
  description?: string
  person?: Partial<PersonObject>
}

// Certificates
export interface CertificateOut {
  pk: string
  sk: string
  alias: string
  md5: string
  s3_key: string
  expires_at: string
  created_at: string
}

// Fiscal configs
export interface NFeConfigOut {
  pk: string
  timezone: string
  environment: number
  prod_current_number: number
  prod_current_serie: number
  hom_current_number: number
  hom_current_serie: number
  prod_nsu: number
  prod_last_dist_nsu_at: string | null
  hom_nsu: number
  hom_last_dist_nsu_at: string | null
  /** Identificador do CSRT (NT 2018.005). O `csrt` em si nunca volta da API. */
  csrt_id?: string | null
  updated_at: string
}

export interface NFCeConfigOut {
  pk: string
  timezone: string
  environment: number
  prod_current_number: number
  prod_current_serie: number
  prod_csc: string
  prod_csc_id: number
  hom_current_number: number
  hom_current_serie: number
  hom_csc: string
  hom_csc_id: number
  updated_at: string
}

export type CTeConfigOut = NFeConfigOut
/** O MDF-e tem três campos de leiaute que a NF-e/CT-e não têm. */
export interface MDFeConfigOut extends NFeConfigOut {
  /** Participação no Canal Verde → `ide/indCanalVerde`. */
  ind_canal_verde?: boolean
  /** Inclusão de DF-e por evento após a emissão → `ide/indCarregaPosterior`. */
  ind_carrega_posterior?: boolean
  /** Mensagem ao fisco repetida em toda emissão → `infAdic/infAdFisco`. */
  inf_ad_fisco?: string | null
}

// NFS-e usa uma única série para os dois ambientes.
export interface NfseAbrasfBody {
  endpoint_url: string
  wsdl_version: string
  municipality_code: string
  synchronous: boolean
}

export interface NfseConfigOut {
  pk: string
  provider: 'nacional' | 'abrasf204'
  environment: number
  timezone: string
  c_loc_emi: string
  serie: string
  prod_current_number: number
  hom_current_number: number
  certificate_sk: string | null
  abrasf: NfseAbrasfBody | null
  prod_nsu: number
  prod_last_dist_nsu_at: string | null
  hom_nsu: number
  hom_last_dist_nsu_at: string | null
  updated_at: string
}

// Products
/** Override parcial de tributação aplicado só quando a UF de destino da
 *  operação está em `ufs` (design spec 2026-08-09-tax-config-redesign). */
export interface UfTaxOverride {
  ufs: string[]
  overrides: Record<string, unknown>
}

export interface CfopConfigItem {
  cfop: string
  uf_overrides?: UfTaxOverride[] | null
  // Regime Normal (CRT 3): CST ICMS. Simples Nacional (CRT 1/2/4): CSOSN.
  icms: string | null;
  csosn: string | null;
  // ICMS alíquotas e modalidade
  icms_mod_bc?: string | null
  icms_aliq_override?: string | null
  icms_fcp_override?: string | null
  icms_sn_cred_aliq?: string | null
  icms_ind_deduz_deson?: string | null
  // ICMS ST
  icms_st_mod_bc?: string | null
  icms_st_mva?: string | null
  icms_st_red_bc?: string | null
  icms_st_aliq?: string | null
  icms_st_fcp_aliq?: string | null
  // Conditional ICMS fields (Regime Normal only)
  icms_p_red_bc?: string | null   // % redução BC — CST 20, 70
  icms_mot_des?: string | null    // motivo desoneração — CST 40, 41, 50, 51
  icms_p_dif?: string | null      // % diferimento — CST 51
  icms_pauta_valor?: string | null // valor da pauta fiscal — icms_mod_bc Pauta/PMPF
  // ICMS monofásico combustíveis (CST 02, 15, 53, 61)
  icms_ad_rem?: string | null
  icms_ad_rem_reten?: string | null
  icms_p_red_ad_rem?: string | null
  icms_mot_red_ad_rem?: string | null
  icms_p_dif_mono?: string | null
  // ICMS60 — ST retida anteriormente (opcional)
  icms_v_bc_st_ret?: string | null
  icms_v_icms_st_ret?: string | null
  icms_p_st?: string | null
  icms_fcp_v_bc_st_ret?: string | null
  icms_fcp_st_ret_aliq?: string | null
  /** ICMSST (CST 41) — repasse da ST retida na operação interestadual. */
  icms_v_bc_st_dest?: string | null
  icms_v_icms_st_dest?: string | null
  /** ICMS efetivo — ICMS60, ICMSST e ICMSSN500. */
  icms_p_red_bc_efet?: string | null
  icms_p_icms_efet?: string | null
  /** ICMSPart — partilha do ICMS entre UF de origem e destino. */
  icms_part_p_bc_op?: string | null
  icms_part_uf_st?: string | null
  /** ST desonerada (ICMS10/70/90) e FCP diferido (ICMS51/90). */
  icms_mot_des_st?: string | null
  icms_p_fcp_dif?: string | null
  /** Restante do grupo ISSQN do leiaute. */
  issqn_v_outro?: string | null
  issqn_v_desc_incond?: string | null
  issqn_v_desc_cond?: string | null
  issqn_c_servico?: string | null
  issqn_c_mun?: string | null
  issqn_c_pais?: string | null
  issqn_n_processo?: string | null
  issqn_ind_incentivo?: string | null
  /** Observação fiscal do item (det/obsItem). */
  obs_item_x_campo?: string | null
  obs_item_x_texto?: string | null
  /** IPI por unidade (bebidas, cigarros) — choice com vBC+pIPI. */
  ipi_v_unid?: string | null
  pis: string
  cofins: string
  pis_aliq?: string | null
  cofins_aliq?: string | null
  pis_aliq_unid?: string | null
  cofins_aliq_unid?: string | null
  // PIS/COFINS-ST — substituição tributária (grupo opcional)
  pis_st_aliq?: string | null
  cofins_st_aliq?: string | null
  pis_st_v_bc?: string | null
  cofins_st_v_bc?: string | null
  // IBS/CBS — opcional, tudo-ou-nada (vigência obrigatória ainda não cobre
  // todos os regimes; ver design spec 2026-08-09-tax-config-redesign)
  ibs_cbs_cst?: string | null
  ibs_cbs_class_trib?: string | null
  ibs_uf_aliq?: string | null
  ibs_mun_aliq?: string | null
  cbs_aliq?: string | null
  // IBS/CBS redução e diferimento
  ibs_uf_p_red?: string | null
  ibs_mun_p_red?: string | null
  cbs_p_red?: string | null
  ibs_uf_p_dif?: string | null
  ibs_mun_p_dif?: string | null
  cbs_p_dif?: string | null
  ibs_ind_doacao?: string | null
  ibs_ad_rem?: string | null
  cbs_ad_rem?: string | null
  ibs_cbs_p_dev_trib?: string | null
  // IPI
  ipi_cst?: string | null
  ipi_aliq?: string | null
  // IS — Imposto Seletivo (NT 2024.001)
  is_cst?: string | null
  is_aliq?: string | null
  is_class_trib?: string | null
  is_aliq_espec?: string | null
  is_unid_trib?: string | null
  // ISSQN — Imposto Sobre Serviços (LC 116/2003)
  issqn_ind_iss?: string | null
  issqn_c_list_serv?: string | null
  issqn_c_mun_fg?: string | null
  issqn_aliq?: string | null
  issqn_v_deducao?: string | null
  issqn_v_iss_ret?: string | null
}

export interface ConversionFactorItem {
  origin_unit: string
  target_unit: string
  factor: number
}

export interface ProductOut {
  pk: string
  sk: string
  code: string
  description: string
  brand: string | null
  ncm: string
  cest: string | null
  origin: string | null
  unit: string | null
  taxable_unit: string | null
  cean: string | null
  taxable_cean: string | null
  value: string
  value_resale?: string | null
  net_weight: string | null
  gross_weight: string | null
  // Campos fiscais do produto
  c_benef?: string | null
  ext_ipi?: string | null
  ind_escala?: string | null
  cnpj_fab?: string | null
  ind_tot?: string | null
  icms_aliq_override?: string | null
  fcp_aliq_override?: string | null
  inf_ad_prod?: string | null
  cfop_nfce: string
  cfop_config: CfopConfigItem[]
  /** Perfis fiscais aplicados. Produto sem perfil resolve a tributação como sempre resolveu. */
  tax_profiles?: ProductTaxProfileRef[] | null
  conversion_factors: ConversionFactorItem[]
  // Tipo específico e campos especiais
  prod_type?: string | null
  comb_c_prod_anp?: string | null
  comb_desc_anp?: string | null
  comb_uf_cons?: string | null
  comb_codif?: string | null
  comb_p_glp?: string | null
  comb_p_gnn?: string | null
  comb_p_gni?: string | null
  comb_v_part?: string | null
  comb_p_bio?: string | null
  med_c_prod_anvisa?: string | null
  med_x_motivo_isencao?: string | null
  med_v_pmc?: string | null
  /** NVE, FCI e códigos de barra próprios do produto. */
  nve?: string[] | null
  n_fci?: string | null
  c_barra?: string | null
  c_barra_trib?: string | null
  /** Selo de controle do IPI e enquadramento legal. */
  ipi_cnpj_prod?: string | null
  ipi_c_selo?: string | null
  ipi_q_selo?: string | null
  ipi_c_enq?: string | null
  /** Classificação de produto perigoso (MDF-e peri). */
  peri_n_onu?: string | null
  peri_x_nome_ae?: string | null
  peri_x_cla_risco?: string | null
  peri_gr_emb?: string | null
  peri_q_vol_tipo?: string | null
  // veicProd — dados do modelo
  veic_tp_op?: string | null
  veic_tp_comb?: string | null
  veic_tp_pint?: string | null
  veic_tp_veic?: string | null
  veic_esp_veic?: string | null
  veic_vin?: string | null
  veic_cond_veic?: string | null
  veic_c_mod?: string | null
  veic_c_cor_denatran?: string | null
  veic_lota?: string | null
  veic_tp_rest?: string | null
  veic_ano_mod?: string | null
  veic_ano_fab?: string | null
  veic_pot?: string | null
  veic_cilin?: string | null
  veic_cmt?: string | null
  veic_dist?: string | null
  veic_c_cor?: string | null
  veic_x_cor?: string | null
  // arma
  arma_tp_arma?: string | null
  arma_descr?: string | null
  created_at: string
  updated_at: string
}

export interface ProductCreate {
  code: string
  description: string
  brand?: string | null
  ncm: string
  cest?: string | null
  origin?: string | null
  cean?: string | null
  unit?: string | null
  taxable_unit?: string | null
  taxable_cean?: string | null
  net_weight?: string | null
  gross_weight?: string | null
  value: string
  value_resale?: string | null
  // Campos fiscais do produto
  c_benef?: string | null
  ext_ipi?: string | null
  ind_escala?: string | null
  cnpj_fab?: string | null
  ind_tot?: string | null
  icms_aliq_override?: string | null
  fcp_aliq_override?: string | null
  inf_ad_prod?: string | null
  cfop_nfce: string
  cfop_config?: CfopConfigItem[]
  tax_profiles?: ProductTaxProfileRef[] | null
  conversion_factors?: ConversionFactorItem[]
  // Tipo específico e campos especiais
  prod_type?: string | null
  comb_c_prod_anp?: string | null
  comb_desc_anp?: string | null
  comb_uf_cons?: string | null
  comb_codif?: string | null
  comb_p_glp?: string | null
  comb_p_gnn?: string | null
  comb_p_gni?: string | null
  comb_v_part?: string | null
  comb_p_bio?: string | null
  med_c_prod_anvisa?: string | null
  med_x_motivo_isencao?: string | null
  med_v_pmc?: string | null
  /** NVE, FCI e códigos de barra próprios do produto. */
  nve?: string[] | null
  n_fci?: string | null
  c_barra?: string | null
  c_barra_trib?: string | null
  /** Selo de controle do IPI e enquadramento legal. */
  ipi_cnpj_prod?: string | null
  ipi_c_selo?: string | null
  ipi_q_selo?: string | null
  ipi_c_enq?: string | null
  /** Classificação de produto perigoso (MDF-e peri). */
  peri_n_onu?: string | null
  peri_x_nome_ae?: string | null
  peri_x_cla_risco?: string | null
  peri_gr_emb?: string | null
  peri_q_vol_tipo?: string | null
  veic_tp_op?: string | null
  veic_tp_comb?: string | null
  veic_tp_pint?: string | null
  veic_tp_veic?: string | null
  veic_esp_veic?: string | null
  veic_vin?: string | null
  veic_cond_veic?: string | null
  veic_c_mod?: string | null
  veic_c_cor_denatran?: string | null
  veic_lota?: string | null
  veic_tp_rest?: string | null
  veic_ano_mod?: string | null
  veic_ano_fab?: string | null
  veic_pot?: string | null
  veic_cilin?: string | null
  veic_cmt?: string | null
  veic_dist?: string | null
  veic_c_cor?: string | null
  veic_x_cor?: string | null
  arma_tp_arma?: string | null
  arma_descr?: string | null
}

export interface ProductUpdate {
  code?: string
  description?: string
  ncm?: string
  cest?: string | null
  cean?: string | null
  taxable_cean?: string | null
  value?: string
  value_resale?: string | null
  cfop_nfce?: string
  cfop_config?: CfopConfigItem[]
  tax_profiles?: ProductTaxProfileRef[] | null
  conversion_factors?: ConversionFactorItem[]
}

// Serviços (catálogo NFS-e)
export interface ServiceIssBody {
  // 1 operação tributável | 2 imunidade | 3 exportação de serviço | 4 não incidência
  trib_issqn: number
  tax_rate: string
  // 1 não retido | 2 retido pelo tomador | 3 retido pelo intermediário
  tp_ret_issqn?: number | null
  tp_imunidade?: number | null
  c_pais_resultado?: string | null
}

export interface ServiceFederalBody {
  cst_pis_cofins?: string | null
  aliq_pis?: string | null
  aliq_cofins?: string | null
  tp_ret_pis_cofins?: number | null
  v_ret_cp?: string | null
  v_ret_irrf?: string | null
  v_ret_csll?: string | null
}

export interface ServiceIbsCbsBody {
  c_ind_op: string
  cst: string
  c_class_trib: string
  ind_dest: number
  tp_oper?: number | null
  fin_nfse: 0
}

export interface ServiceTotTribBody {
  ind_tot_trib: number
  p_tot_trib_sn?: string | null
}

export interface ServiceOut {
  pk: string
  sk: string
  code: string
  description: string
  trib_nacional_code: string
  trib_municipal_code: string | null
  nbs_code: string | null
  cnae: string | null
  unit: string
  value: string
  iss: ServiceIssBody
  federal: ServiceFederalBody | null
  ibs_cbs: ServiceIbsCbsBody | null
  tot_trib: ServiceTotTribBody | null
  created_at: string
  updated_at: string
}

export interface ServiceCreate {
  code: string
  description: string
  trib_nacional_code: string
  trib_municipal_code?: string | null
  nbs_code?: string | null
  cnae?: string | null
  unit: string
  value: string
  iss: ServiceIssBody
  federal?: ServiceFederalBody | null
  ibs_cbs: ServiceIbsCbsBody
  tot_trib?: ServiceTotTribBody | null
}

export type ServiceUpdate = ServiceCreate

// Vehicles
export interface OwnerOut {
  cpf_cnpj: string
  rntrc: string
  name: string
  type: string
}

export interface VehicleOut {
  pk: string
  sk: string
  plate: string
  plate_uf: string
  role: 'tractor' | 'trailer'
  wheelset?: string
  bodywork?: string
  renavam?: string
  weight?: number
  cap_kg?: number
  cap_m3?: number
  cint?: string
  owner?: OwnerOut
  created_at: string
  updated_at: string
}

export interface VehicleCreate {
  plate: string
  plate_uf: string
  role: 'tractor' | 'trailer'
  wheelset?: string
  bodywork?: string
  renavam?: string
  weight?: number
  cap_kg?: number
  cap_m3?: number
  cint?: string
  owner?: {
    cpf_cnpj: string
    rntrc: string
    name: string
    type: string
  }
}

export interface VehicleUpdate {
  plate?: string
  plate_uf?: string
  role?: 'tractor' | 'trailer'
  wheelset?: string
  bodywork?: string
  renavam?: string
  weight?: number
  cap_kg?: number
  cap_m3?: number
  cint?: string
  owner?: VehicleCreate['owner']
}

export interface VehicleRequirements {
  missing: string[]
}

// Persons (Clientes/Fornecedores)
export interface PersonAddressOut {
  city_ibge_code: string
  street: string
  neighborhood: string
  number: string
  city: string
  state_federation: string
  postal_code: string
  complement: string | null
}

export type Crt = "1" | "2" | "3" | "4";

export interface PersonDetailsOut {
  fantasy_name: string
  crt: string | number
  state_registrations: StateRegistrationOut[]
  addresses: PersonAddressOut[]
  contacts?: ContactsOut
  nfse?: NfseInfo | null
  /** Locais de entrega salvos de emissões de NF-e anteriores a este destinatário. */
  delivery_locations?: NfeLocalOut[]
  bank?: PersonBank | null
  freight_retention?: PersonFreightRetention | null
}

/** Perfil de ICMS retido pelo remetente sobre o frete (NF-e transp/retTransp). */
export interface PersonFreightRetention {
  v_serv?: string | null
  v_bc_ret?: string | null
  p_icms_ret?: string | null
  cfop?: string | null
  c_mun_fg?: string | null
}

/** Recebimento do condutor/TAC (MDF-e infANTT/infPag/infBanc). Choice: PIX, ou
 *  banco + agência, ou CNPJ da instituição de pagamento. */
export interface PersonBank {
  pix_key?: string | null
  bank_code?: string | null
  branch_code?: string | null
  cnpj_ipef?: string | null
}

export interface PersonItemOut {
  pk: string
  sk: string
  name: string
  /** Papéis de cadastro (cliente/fornecedor/transportadora/condutor/prestador).
   *  Ausente em pessoas cadastradas antes dos papéis existirem. */
  roles?: PersonRole[] | null
  person: PersonDetailsOut
  created_at: string
  updated_at: string
}

export interface PersonObject {
  fantasy_name: string | null
  crt: string | number | null
  state_registrations: StateRegistrationOut[]
  addresses: PersonAddressOut[]
  contacts?: ContactsOut
  nfse?: NfseInfo | null
}

export interface PersonCreate {
  /** CPF/CNPJ, ou vazio quando `id_estrangeiro` identifica a pessoa. */
  cpf_or_cnpj: string
  /** Documento de pessoa no exterior (dest/idEstrangeiro). Exclusivo com cpf_or_cnpj. */
  id_estrangeiro?: string | null
  name: string
  roles?: PersonRole[]
  person: PersonObject
}

export interface PersonUpdate {
  name?: string
  roles?: PersonRole[]
  person?: PersonObject
}

// Cadastros reutilizáveis — perfis fiscais
export interface TaxProfileCreate extends Record<string, unknown> {
  name: string
  description?: string | null
  cfops: string[]
}

export interface TaxProfileItemOut {
  pk: string
  sk: string
  name: string
  description?: string | null
  cfops: string[]
  created_at: string
  updated_at: string

  [field: string]: unknown
}

/** Vínculo produto → perfil fiscal, com sobrescrita parcial opcional. */
export interface ProductTaxProfileRef {
  tax_profile_id: string
  overrides?: Record<string, unknown> | null
}

// Cadastros reutilizáveis — naturezas de operação
export interface OperationCreate extends Record<string, unknown> {
  name: string
  is_default?: boolean
}

export interface OperationItemOut {
  pk: string
  sk: string
  name: string
  doc_types?: string[] | null
  nat_op?: string | null
  cfop_suffix?: string | null
  is_default?: boolean
  created_at: string
  updated_at: string

  [field: string]: unknown
}

// Cadastros reutilizáveis — terminais de pagamento (POS)
export interface PaymentTerminalCreate extends Record<string, unknown> {
  name: string
  cnpj_receb: string
  id_term_pag: string
  cnpj_pag?: string | null
  uf_pag?: string | null
  t_band?: string | null
}

export interface PaymentTerminalItemOut {
  pk: string
  sk: string
  name: string
  cnpj_receb: string
  id_term_pag: string
  created_at: string
  updated_at: string

  [field: string]: unknown
}

// Cadastros reutilizáveis — fornecedoras de vale-pedágio
export interface TollProviderCreate extends Record<string, unknown> {
  name: string
  cnpj_forn: string
  cnpj_pg?: string | null
  cpf_pg?: string | null
  tp_vale_ped?: string | null
}

export interface TollProviderItemOut {
  pk: string
  sk: string
  name: string
  cnpj_forn: string
  created_at: string
  updated_at: string

  [field: string]: unknown
}

// Cadastros reutilizáveis — apólices de seguro da carga (MDF-e infMDFe/seg)
export interface InsurancePolicyCreate extends Record<string, unknown> {
  name: string
  /** 1 = emitente do MDF-e; 2 = contratante do serviço de transporte. */
  resp_seg: string
  cnpj?: string | null
  cpf?: string | null
  x_seg?: string | null
  cnpj_seg?: string | null
  n_apol?: string | null
}

export interface InsurancePolicyItemOut {
  pk: string
  sk: string
  name: string
  resp_seg: string
  created_at: string
  updated_at: string

  [field: string]: unknown
}

// Cadastros reutilizáveis — unidades de transporte e de carga (MDF-e)
export interface CargoUnitCreate extends Record<string, unknown> {
  name: string
  /** transport = infUnidTransp (carreta, vagão); cargo = infUnidCarga (contêiner, pallet). */
  kind: 'transport' | 'cargo'
  tp_unid: string
  id_unid: string
  seals?: string[] | null
}

export interface CargoUnitItemOut {
  pk: string
  sk: string
  name: string
  kind: 'transport' | 'cargo'
  tp_unid: string
  id_unid: string
  created_at: string
  updated_at: string

  [field: string]: unknown
}

// Cadastros reutilizáveis — declarações de importação (NF-e prod/DI)
export interface ImportAdditionIn {
  n_adicao: string
  c_fabricante: string
  v_desc_di?: string | null
  n_draw?: string | null
}

export interface ImportDeclarationCreate extends Record<string, unknown> {
  name: string
  n_di: string
  d_di: string
  x_loc_desemb: string
  uf_desemb: string
  d_desemb: string
  tp_via_transp: string
  v_afrmm?: string | null
  tp_intermedio: string
  cnpj?: string | null
  uf_terceiro?: string | null
  c_exportador: string
  additions: ImportAdditionIn[]
}

export interface ImportDeclarationItemOut {
  pk: string
  sk: string
  name: string
  n_di: string
  additions?: ImportAdditionIn[]
  created_at: string
  updated_at: string

  [field: string]: unknown
}

/** Exportação indireta do item (prod/detExport). O trio nRE+chNFe+qExport é
 *  tudo-ou-nada. */
export interface NfeDetExportIn {
  n_draw?: string
  n_re?: string
  ch_nfe?: string
  q_export?: string
}

/** Vínculo item↔adição da DI na emissão. nAdicao/nSeqAdic são derivados. */
export interface NfeItemDIIn {
  import_declaration_id: string
  addition_index: number
  n_draw?: string
}

// Cadastros reutilizáveis — condições de pagamento e composições veiculares
export interface PaymentTermCreate extends Record<string, unknown> {
  name: string
  payment_type: string
  installments: number
}

export interface PaymentTermItemOut {
  pk: string
  sk: string
  name: string
  payment_type: string
  ind_pag?: string | null
  installments: number
  interval_days?: number
  first_due_days?: number
  created_at: string
  updated_at: string
}

export interface VehicleSetCreate extends Record<string, unknown> {
  name: string
  tractor_sk: string
}

export interface VehicleSetItemOut {
  pk: string
  sk: string
  name: string
  tractor_sk: string
  trailer_sks?: string[] | null
  driver_docs?: string[] | null
  rntrc?: string | null
  ciot?: string | null
  created_at: string
  updated_at: string
}

// NF-e — tipos auxiliares
export interface NfeCardIn {
  tp_integra: '1' | '2'
  cnpj?: string | null
  t_band?: string | null
  c_aut?: string | null
}

export interface NfeTransportIn {
  mod_frete: '0' | '1' | '2' | '3' | '4' | '9'
  /** SK da transportadora cadastrada. Backend resolve os dados automaticamente. */
  transporta_pk?: string | null
  transporta_cnpj?: string | null
  transporta_cpf?: string | null
  transporta_nome?: string | null
  transporta_ie?: string | null
  transporta_ender?: string | null
  transporta_mun?: string | null
  transporta_uf?: string | null
  /** SK do veículo cadastrado. Backend resolve placa/UF/RNTRC automaticamente. */
  veiculo_sk?: string | null
  veiculo_placa?: string | null
  veiculo_uf?: string | null
  veiculo_rntrc?: string | null
  /** Volumes transportados (transp/vol). Vazio ⇒ backend deriva um volume com o peso dos itens. */
  vols?: NfeVolIn[] | null
  /** Reboques do veículo transportador (transp/reboque, máx 5). */
  reboques?: NfeReboqueIn[] | null
}

/** Volume transportado (transp/vol). */
export interface NfeVolIn {
  q_vol?: string | null
  esp?: string | null
  marca?: string | null
  n_vol?: string | null
  peso_l?: string | null
  peso_b?: string | null
  lacres?: string[] | null
}

/** Reboque do veículo transportador (transp/reboque). */
export interface NfeReboqueIn {
  placa: string
  uf: string
  rntc?: string | null
}

/** TLocal-shaped address (local de retirada/entrega) — lighter than
 * AddressOut/AddressBody (TEndereco): no CEP/IBGE code in the XSD. */
export interface NfeLocalOut {
  cnpj?: string
  cpf?: string
  x_nome?: string
  x_lgr: string
  nro: string
  x_cpl?: string
  x_bairro: string
  c_mun: string
  x_mun: string
  uf: string
  fone?: string
  email?: string
}

export interface NfeLocalIn {
  cnpj?: string | null
  cpf?: string | null
  x_nome?: string | null
  x_lgr: string
  nro: string
  x_cpl?: string | null
  x_bairro: string
  c_mun: string
  x_mun: string
  uf: string
  fone?: string | null
  email?: string | null
}

export interface NfeDuplicataIn {
  n_dup?: string | null
  d_venc?: string | null
  v_dup: string
}

export interface NfeFatIn {
  n_fat?: string | null
  v_orig?: string | null
  v_desc?: string | null
  v_liq?: string | null
}

// NF-e
export const NF_PAYMENT_TYPES: Record<string, string> = {
  '01': 'Dinheiro',
  '02': 'Cheque',
  '03': 'Cartão de Crédito',
  '04': 'Cartão de Débito',
  '05': 'Cartão da Loja (Private Label), Crediário Digital, Outros Crediários',
  '10': 'Vale Alimentação',
  '11': 'Vale Refeição',
  '12': 'Vale Presente',
  '13': 'Vale Combustível',
  '14': 'Duplicata Mercantil',
  '15': 'Boleto Bancário',
  '16': 'Depósito Bancário',
  '17': 'PIX',
  '18': 'Transferência bancária, Carteira Digital',
  '19': 'Programa de fidelidade, Cashback, Crédito Virtual',
  '20': 'PIX (Estático)',
  '21': 'Crédito em Loja',
  '22': 'Pagamento Eletrônico não Informado - falha de hardware do sistema emissor',
  '90': 'Sem pagamento',
  '99': 'Outros',
}

export const displayPaymentTypeLabel = (code: string): string | undefined => {
  return NF_PAYMENT_TYPES[code]
}

export interface NfeArmaIn {
  n_serie: string
  n_cano: string
  descr?: string | null
}

export interface NfeProductIn {
  product_id: string
  cfop: string
  quantity: string
  unit_value?: string | null
  discount?: string
  v_frete?: string | null
  v_seg?: string | null
  v_outro?: string | null
  // veicProd — por unidade
  veic_chassi?: string | null
  veic_n_serie?: string | null
  veic_n_motor?: string | null
  veic_c_cor?: string | null
  veic_x_cor?: string | null
  // arma — por unidade
  armas?: NfeArmaIn[] | null
}

export interface NfePaymentIn {
  payment_type: string
  value: string
  ind_pag?: '0' | '1' | null
  d_pag?: string | null
  card?: NfeCardIn | null
  /** Terminal de captura (organization_payment_terminals) que processou o pagamento. */
  terminal_id?: string | null
  /** Descrição da forma de pagamento quando tPag é 99 (outros). */
  x_pag?: string | null
}

/** Documento referenciado em ide/NFref. Espelha `NfeRefBody` do backend. */
export interface NfeRefIn {
  nfe_id?: string | null
  kind?: 'nfe' | 'nfesig' | 'nf' | 'nfp' | 'cte' | 'ecf' | null
  access_key?: string | null
  c_uf?: string | null
  aamm?: string | null
  cnpj?: string | null
  cpf?: string | null
  ie?: string | null
  mod?: string | null
  serie?: string | null
  n_nf?: string | null
  n_ecf?: string | null
  n_coo?: string | null
}

export interface NfeEmit {
  receiver_id?: string | null  // person sk: CPF_xxx or CNPJ_xxx — omit when self_issuance=true
  self_issuance?: boolean
  /** Natureza de operação do cadastro. Todo valor explícito aqui vence os defaults dela. */
  operation_id?: string | null
  /** Condição de pagamento do cadastro. `payments`/`cobr_*` explícitos vencem a expansão. */
  payment_term_id?: string | null
  products: NfeProductIn[]
  payments: NfePaymentIn[]
  additional_info?: string | null
  // Campos opcionais de emissão
  nat_op?: string | null
  fin_nfe?: '1' | '2' | '3' | '4' | null
  ind_final?: '0' | '1' | null
  ind_pres?: string | null
  tp_nf?: '0' | '1' | null
  transport?: NfeTransportIn | null
  cobr_fat?: NfeFatIn | null
  cobr_duplicatas?: NfeDuplicataIn[] | null
  v_troco?: string | null
  retirada?: NfeLocalIn | null
  entrega?: NfeLocalIn | null
  save_retirada_location?: boolean
  save_entrega_location?: boolean
  /** Documentos referenciados. Obrigatório para fin_nfe 2, 3 e 4. */
  nf_refs?: NfeRefIn[] | null
  /** Processos referenciados (infAdic/procRef). */
  proc_ref?: NfeProcRefIn[] | null
}

/** Processo referenciado em infAdic/procRef. */
export interface NfeProcRefIn {
  n_proc: string
  /** 0 SEFAZ, 1 Justiça Federal, 2 Justiça Estadual, 3 Secex/RFB, 9 outros. */
  ind_proc: '0' | '1' | '2' | '3' | '9'
  tp_ato?: string | null
}

// NFC-e (modelo 65) — no recipient address, transport, or billing.
export interface NfceEmit {
  consumer_cpf?: string | null  // optional; CPF only (pessoa física)
  products: NfeProductIn[]
  payments: NfePaymentIn[]
  additional_info?: string | null
  nat_op?: string | null
}

export interface NfeProductOut {
  product_id: string
  product_code: string
  description: string
  ncm: string
  cfop: string
  unit: string
  quantity: string
  unit_value: string
  discount: string
  total: string
}

export interface NfePaymentOut {
  payment_type: string
  value: string
}

// Todos os documentos passam pelo mesmo worker (DfeService), então compartilham
// o ciclo de vida; `DfeStatus` (lib/data/dfe_status) é a lista canônica.
export type NfeStatus = DfeStatus

export interface NfeListOut {
  pk: string    // {env}#{org_pk}
  sk: string    // 44-digit chave de acesso
  incoming: number
  year: number
  month: number
  day: number
  status: NfeStatus
  sefaz_status: string | null
  sefaz_motive: string | null
  emit_cpf_cnpj: string
  emit_name: string
  dest_cpf_cnpj: string
  dest_name: string
  number: number
  serie: number
  total: string
  dh_emi: string | null
  created_at: string
}

export interface NfeDetailOut extends NfeListOut {
  products: NfeProductOut[] | null;
  payments: NfePaymentOut[] | null;
  additional_info: string | null
  xml_s3_key: string | null
  sefaz_protocol: string | null
}

export interface NfeEventOut {
  pk: string           // org_pk
  sk: string           // timestamp_uuid (event identifier)
  access_key: string
  event_type: string   // tpEvento code or "emission"
  sequence_number: number
  status: DfeStatus
  sefaz_status: string | null
  sefaz_motive: string | null
  sefaz_protocol: string | null
  xml_s3_key: string | null
  created_at: string
  updated_at: string
}

// ─── MDF-e (modelo 58) ──────────────────────────────────────────────────────

export type MdfeStatus = DfeStatus

export interface MdfeMunIn {
  ibge_code: string
  city: string
}

export interface MdfeOwnerIn {
  cpf?: string
  cnpj?: string
  name: string
  ie?: string
  uf?: string
  rntrc: string
  tp_prop?: string   // 0=TAC Agregado, 1=TAC Independente, 2=Outros
  tp_transp?: string // optional CTC(3) override for a CNPJ owner
}

export interface MdfeDocRef {
  type: 'nfe' | 'cte'
  access_key: string
  weight?: string        // optional gross-weight override (kg) when XML carries none
}

export interface MdfeVehicleIn {
  sk?: string | null      // registered vehicle SK (required path for emission)
  placa?: string
  tara?: string
  uf?: string
  renavam?: string | null
  cap_kg?: string | null
  tp_rod?: string
  tp_car?: string
  owner?: MdfeOwnerIn | null  // third-party owner (drives tpTransp / prop)
}

export interface MdfeDriverIn {
  name: string
  cpf: string
}

export interface MdfeProdPredIn {
  tp_carga: string
  x_prod: string
  ncm: string
}

export interface MdfeBulkCargoIn {
  cep_loading: string
  cep_unloading: string
  lat_loading?: string | null
  lon_loading?: string | null
  lat_unloading?: string | null
  lon_unloading?: string | null
}

export interface MdfeEmit {
  modal: 'rodoviario' | 'aereo' | 'aquaviario' | 'ferroviario'
  documents: MdfeDocRef[]
  uf_start?: string
  uf_end?: string
  route?: string[]
  loadings?: MdfeMunIn[]
  unloadings?: MdfeMunIn[]
  /** Composição veicular do cadastro. Cada campo expandido continua sobrescrevível aqui. */
  vehicle_set_id?: string | null
  vehicle: MdfeVehicleIn
  trailers?: { sk: string }[]
  drivers: MdfeDriverIn[]
  predominant?: MdfeProdPredIn | null
  bulk_cargo?: MdfeBulkCargoIn | null
  trip_start?: string | null
  rntrc?: string | null
  ciot?: string | null
  additional_info?: string | null
  /** Vales-pedágio da viagem (infANTT/valePed). O fornecedor vem do cadastro. */
  toll_vouchers?: MdfeTollIn[] | null
  /** Apólices de seguro da carga desta viagem (infMDFe/seg). */
  insurance_policies?: MdfeInsuranceIn[] | null
  contractors?: MdfeContractorIn[] | null
  payments?: MdfePaymentIn[] | null
  /** Unidades de transporte da viagem (infUnidTransp). */
  transport_units?: MdfeTransportUnitIn[] | null
  /** Chaves dos documentos em reentrega (infDoc/.../indReentrega). */
  redelivery_keys?: string[] | null
  /** Lacres da carga (infMDFe/lacres). */
  seals?: string[] | null
  /** Lacres da unidade de transporte (rodo/lacRodo). */
  rodo_seals?: string[] | null
  /** Código do agente portuário (rodo/codAgPorto). */
  port_agent_code?: string | null
  /** Dados do voo — obrigatório quando `modal = aereo` (grupo `aereo`). */
  air?: MdfeAirModalIn | null
  /** Dados do trem — obrigatório quando `modal = ferroviario` (grupo `ferrov`). */
  rail?: MdfeRailModalIn | null
}

/** Grupo `aereo`. Os seis campos são obrigatórios no XSD. */
export interface MdfeAirModalIn {
  nationality: string
  registration: string
  flight_number: string
  origin_airport: string
  dest_airport: string
  flight_date: string
}

/** Um vagão do grupo `ferrov/vag`. */
export interface MdfeRailWagonIn {
  weight_bc: string
  weight_real: string
  wagon_type?: string
  series: string
  number: string
  sequence?: string
  /** Tonelada útil. */
  tu: string
}

/** Grupo `ferrov`. O `qVag` do XML sai do tamanho de `wagons`. */
export interface MdfeRailModalIn {
  train_prefix: string
  train_datetime?: string
  origin_station: string
  dest_station: string
  wagons: MdfeRailWagonIn[]
}

/** Unidade de transporte da viagem: unidade do cadastro, documentos que ela
 *  leva e unidades de carga dentro dela. O rateio é calculado no backend. */
export interface MdfeTransportUnitIn {
  cargo_unit_id: string
  document_keys: string[]
  cargo_unit_ids?: string[]
}

/** Componente do valor do frete (`infPag/Comp`). */
export interface MdfePaymentComponentIn {
  type: string
  value: string
  description?: string
}

/** Pagamento ao transportador autônomo na emissão (`infANTT/infPag`). As
 *  parcelas são derivadas do prazo pelo backend, nunca enviadas. */
export interface MdfePaymentIn {
  person_doc: string
  components: MdfePaymentComponentIn[]
  contract_value: string
  /** indPag: 0 à vista, 1 a prazo. */
  payment_type: string
  advance_value?: string
  high_performance?: string
  installments?: number
  interval_days?: number
  first_due_days?: number
}

/** Contratante do frete. Identidade vem do cadastro; o contrato é da viagem. */
export interface MdfeContractorIn {
  person_doc: string
  contract_number?: string
  contract_value?: string
}

/** Vale-pedágio de uma viagem. Só o que muda entre viagens. */
export interface MdfeInsuranceIn {
  insurance_policy_id: string
  /** Averbações desta viagem (nAver). */
  n_aver?: string[]
}

export interface MdfeTollIn {
  toll_provider_id: string
  n_compra: string
  v_vale_ped: string
}

// ── cargo preview (POST /mdfes/cargo-preview) ──
export interface MdfeCargoPreviewDoc {
  type: 'nfe' | 'cte'
  access_key: string
  emit_name: string
  dest_name: string
  loading: MdfeMunIn
  unloading: MdfeMunIn
  uf_start: string
  uf_end: string
  weight: string
  has_weight: boolean
  value: string
  predominant: MdfeProdPredIn
}

export interface MdfeCargoPreview {
  documents: MdfeCargoPreviewDoc[]
  loadings: MdfeMunIn[]
  unloadings: MdfeMunIn[]
  uf_start: string
  uf_end: string
  total_weight: string
  total_value: string
  predominant: MdfeProdPredIn
}

export interface MdfeListOut {
  pk: string
  sk: string            // 44-digit chave de acesso
  incoming: number
  year: number
  month: number
  day: number
  status: MdfeStatus
  sefaz_status: string | null
  sefaz_motive: string | null
  emit_cpf_cnpj: string
  emit_name: string
  number: number
  serie: number
  modal: string
  doc_type: string
  uf_start: string
  uf_end: string
  cargo_weight: string
  cargo_value: string
  dh_emi: string | null
  created_at: string
}

export interface MdfeUnloadingOut extends MdfeMunIn {
  access_keys: string[]
}

export interface MdfeDetailOut extends MdfeListOut {
  documents: MdfeDocRef[]
  route: string[] | null
  loadings: MdfeMunIn[] | null
  unloadings: MdfeUnloadingOut[] | null
  predominant: MdfeProdPredIn | null
  vehicle: {
    placa: string;
    uf: string;
    tara: string;
    rntrc: string;
    owner?: { cpf: string; cnpj: string; name: string; rntrc: string }
  } | null
  drivers: MdfeDriverIn[] | null
  trip_start: string | null
  bulk_cargo: { cep_loading: string; cep_unloading: string } | null
  xml_s3_key: string | null
  sefaz_protocol: string | null
}

export interface MdfeIncludeDFeDoc {
  unloading_ibge_code: string
  unloading_city: string
  nfe_key: string
}

// ─── NFS-e ───────────────────────────────────────────────────────────────

export type NfseStatus = DfeStatus

export interface NfseServiceItem {
  service_id: string
  description?: string | null
  value?: string | null
  tax_rate?: string | null
  c_trib_mun?: string | null
}

export interface NfseEmit {
  tp_emit: 1 | 2 | 3
  motivo_emis_ti?: 1 | 2 | 3 | 4
  ch_nfse_rej?: string
  competence: string
  provider_person_id?: string | null
  customer_id?: string | null
  intermediary_id?: string | null
  service: NfseServiceItem
  substitutes_access_key?: string | null
  substitutes_reason?: string | null
  additional_info?: string | null
}

export interface NfseEmitInputSnapshot {
  tp_emit: 1 | 2 | 3
  motivo_emis_ti?: 1 | 2 | 3 | 4
  ch_nfse_rej?: string
  provider_person_id?: string | null
  customer_id?: string | null
  intermediary_id?: string | null
  service: NfseServiceItem
  additional_info?: string | null
}

export interface NfseEventBody {
  event_type: string
  sequence_number?: number
  reason_code?: string
  reason_description?: string
  substitute_access_key?: string
  cpf_ag_trib?: string
  id_ev_manif_rej?: string
}

export interface NfseListOut {
  pk: string    // {env}#{org_pk}
  sk: string    // id_dps (45 chars) — não a chave de acesso
  provider: 'nacional' | 'abrasf204'
  status: NfseStatus
  tp_emit: number
  serie: string
  number: number
  competence: string
  dh_emi: string
  c_loc_emi: string
  year: number
  month: number
  emit_cpf_cnpj: string
  emit_name: string
  dest_cpf_cnpj: string | null
  dest_name: string | null
  total: string
  payload: Record<string, unknown>
  /** Presente em emissões novas; permite duplicação sem inferir IDs fiscais. */
  emit_input?: NfseEmitInputSnapshot | null
  access_key: string | null
  xml_s3_key: string | null
  dps_xml_s3_key: string | null
  sefaz_motive: string | null
  c_motivo_emis_ti: number | null
  user_id: string
  user_name: string
  created_at: string
  updated_at: string
}

// GET /nfses e GET /nfses/{id} devolvem o mesmo item — o backend não projeta
// campos entre lista e detalhe (nfses.go, api/internal/api/v1/nfses.go).
export type NfseDetailOut = NfseListOut

export interface NfseEventOut {
  pk: string           // id_dps (não org_pk — spec §3.5)
  sk: string
  access_key: string   // aqui carrega o id_dps, não a chave de acesso
  event_type: string
  sequence_number: number
  status: DfeStatus
  sefaz_status: string | null
  sefaz_motive: string | null
  sefaz_protocol: string | null
  xml_s3_key: string | null
  created_at: string
  updated_at: string
}

// persistNfseIncoming (worker/internal/service/distribution_nfse.go) não faz
// parsing de campos do XML — ao contrário de NF-e/CT-e/MDF-e, não há
// emit_name/dest_name/total/sefaz_*/dh_emi aqui.
export interface NfseDistributionOut {
  nsu: number
  doc_type: string
  schema_type: string
  access_key: string | null
  event_type: string | null
  xml_s3_key: string
  created_at: string
}

// GET /nfse/municipal-parameters/{city}/{kind} — proxy do ADN; a forma varia
// por kind (aliquota/regimes-especiais/beneficio/retencoes), então tipar
// campo a campo aqui reimplementaria o leiaute do ADN sem necessidade.
export type MunicipalParamsOut = Record<string, unknown>

export interface NFeDistributionOut {
  nsu: number
  doc_schema: string
  schema_type: string | null
  access_key: string | null
  emit_name: string | null
  emit_cpf_cnpj: string | null
  dest_name: string | null
  total: string | null
  sefaz_status: string | null
  sefaz_motive: string | null
  sefaz_protocol: string | null
  event_type: string | null
  dh_emi: string | null
  parse_error: boolean | null
  xml_s3_key: string | null
  created_at: string
}

/**
 * Inutilização de numeração (NF-e / NFC-e). Persistida na tabela de eventos do
 * documento — `status` segue o vocabulário compartilhado de DfeStatus, onde
 * `success` significa faixa homologada pela SEFAZ (cStat 102).
 */
export interface InutilizationOut {
  pk: string
  sk: string
  event_type: string
  event_key: string
  status: string
  year: number
  serie: number
  number_start: number
  number_end: number
  justification: string
  sefaz_status: string | null
  sefaz_motive: string | null
  xml_s3_key: string | null
  user_id: string | null
  user_name: string | null
  created_at: string
  updated_at: string
}

/** Faixa contígua de números sem documento utilizável e ainda não inutilizada. */
export interface NumberGapOut {
  serie: number
  number_start: number
  number_end: number
}

/** Corpo de POST /{doc}s/inutilizations. */
export interface InutilizationIn {
  serie: number
  number_start: number
  number_end: number
  justification: string
  year?: number
}

export interface SyncEnqueuedOut {
  status: string
  nsu: number
}

export interface DistributionLookupDoc {
  nsu: number
  schema: string
}

export interface DistributionLookupOut {
  c_stat: string
  x_motivo: string
  ult_nsu: string | null
  max_nsu: string | null
  docs: DistributionLookupDoc[]
  created_at: string
}

// Audit log
export interface AuditLogModification {
  name: string
  before: unknown
  after: unknown
}

export interface AuditLogOut {
  pk: string
  sk: string
  resource_type: string
  resource_id: string
  action: 'CREATE' | 'UPDATE' | 'DELETE'
  modifications: AuditLogModification[]
  user_id: string
  user_name: string
  created_at: string
}

// Error
export interface ApiError {
  detail: string
}

// Pagination — matches backend PaginatedResponse (cursor-based, bidirectional)
export interface PaginatedResponse<T> {
  items: T[]
  next_cursor: string | null
  has_next: boolean
  previous_cursor: string | null
  has_previous: boolean
}
