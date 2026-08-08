import type {DfeStatus} from '@/lib/data/dfe_status'

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
export type MDFeConfigOut = NFeConfigOut

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
export interface CfopConfigItem {
  cfop: string
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
  pis: string
  cofins: string
  pis_aliq?: string | null
  cofins_aliq?: string | null
  pis_aliq_unid?: string | null
  cofins_aliq_unid?: string | null
  ibs_cbs_cst: string
  ibs_cbs_class_trib: string
  ibs_uf_aliq: string
  ibs_mun_aliq: string
  cbs_aliq: string
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
}

export interface PersonItemOut {
  pk: string
  sk: string
  name: string
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
  cpf_or_cnpj: string
  name: string
  person: PersonObject
}

export interface PersonUpdate {
  name?: string
  person?: PersonObject
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
}

export interface NfeEmit {
  receiver_id?: string | null  // person sk: CPF_xxx or CNPJ_xxx — omit when self_issuance=true
  self_issuance?: boolean
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
  vehicle: MdfeVehicleIn
  trailers?: { sk: string }[]
  drivers: MdfeDriverIn[]
  predominant?: MdfeProdPredIn | null
  bulk_cargo?: MdfeBulkCargoIn | null
  trip_start?: string | null
  rntrc?: string | null
  ciot?: string | null
  additional_info?: string | null
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
