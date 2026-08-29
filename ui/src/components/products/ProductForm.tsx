'use client'

import Link from 'next/link'
import {useState} from 'react'
import {generateEntityCode} from '@/lib/utils/code'
import {useFieldArray, useForm, type UseFormReturn, useWatch} from 'react-hook-form'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {CurrencyInput} from '@/components/ui/currency-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Combobox} from '@/components/ui/combobox'
import {NcmCombobox} from '@/components/ui/ncm-combobox'
import {UF_IBGE_OPTIONS} from '@/lib/data/cities'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {
  type CfopConfigFormData,
  cfopConfigSchema,
  type ConversionFactorFormData,
  type ProductFormData,
  productSchema
} from '@/lib/schemas/products'
import type {CombOrigIn, ProductCreate, ProductOut} from '@/lib/types/api'
import {getCfopOptionsForNfce, getCfopVariants} from '@/lib/data/cfop'
import {
  CSOSN_ST,
  EMPTY_TAX_GROUPS,
  ICMS_MONO_CSTS,
  ICMS_ST_CSTS,
  icmsConditionalFields,
  PIS_COFINS_ALIQ_CSTS,
  TaxFieldsEditor,
  type TaxGroups,
} from '@/components/tax/TaxFieldsEditor'
import {isRegimeSimples} from '@/lib/constants/tax'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {UNIT_OPTIONS} from '@/lib/data/unit'
import {TP_CRED_PRES_IBS_ZFM_OPTIONS} from '@/lib/data/ibs_cbs_reform'
import {ORIGIN_OPTIONS} from '@/lib/data/origin'
import {
  VEIC_COND_OPTIONS,
  VEIC_COR_DENATRAN_OPTIONS,
  VEIC_ESP_VEIC_OPTIONS,
  VEIC_TP_COMB_OPTIONS,
  VEIC_TP_OP_OPTIONS,
  VEIC_TP_PINT_OPTIONS,
  VEIC_TP_REST_OPTIONS,
  VEIC_TP_VEIC_OPTIONS,
  VEIC_VIN_OPTIONS,
  vehicleYearOptions,
} from '@/lib/data/vehicle'
import {UfOverridesEditor, type UfOverrideFormData} from '@/components/tax/UfOverridesEditor'
import {useIcmsAliqPreview} from '@/lib/hooks/useIcmsAliqPreview'

// ─── Types ────────────────────────────────────────────────────────────────────

type ProductTab = 'produto' | 'unidades' | 'tributacao' | 'especial'

interface ProductFormProps {
  initialData?: ProductOut
  /** CRT do regime tributário da organização ativa (1=SN, 2=SN excesso, 3=Regime Normal, 4=MEI) */
  crt?: number | string;
  /** UF da organização emitente — usado para sugerir alíquota ICMS pelo NCM */
  uf?: string
  onSubmit: (data: ProductCreate) => Promise<void>
  loading?: boolean
}

// indBemMovelUsado (prod/indBemMovelUsado) enumera um valor só no XSD.
const IND_BEM_MOVEL_USADO_SIM = '1'

/** Limite de gCred no leiaute (maxOccurs=4). */
const MAX_GCRED = 4

/**
 * Créditos presumidos da UF aplicados ao item (prod/gCred). O `vCredPresumido`
 * não aparece: é o percentual sobre o valor do item, calculado na emissão —
 * pedir os três seria pedir que o operador feche uma conta que o sistema faz.
 */
function GCredEditor({form}: {form: UseFormReturn<ProductFormData>}) {
  const {fields, append, remove} = useFieldArray({control: form.control, name: 'gcred'})
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <FormLabel>Créditos presumidos da UF</FormLabel>
        <Button type="button" variant="ghost" size="xs" disabled={fields.length >= MAX_GCRED}
                onClick={() => append({c_cred_presumido: '', p_cred_presumido: ''})}>
          + Crédito
        </Button>
      </div>
      {fields.length === 0 && (
        <p className="text-xs text-gray-500">
          Nenhum. O valor de cada crédito é calculado do percentual na emissão.
        </p>
      )}
      {fields.map((row, index) => (
        <div key={row.id}
             className="grid grid-cols-1 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto] gap-2 items-end">
          <FormField control={form.control} name={`gcred.${index}.c_cred_presumido`} render={({field}) => (
            <FormItem>
              <FormLabel>Código do benefício</FormLabel>
              <Input {...field} id={field.name} value={field.value ?? ''} maxLength={10}
                     className="w-full" placeholder="8 ou 10 caracteres"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name={`gcred.${index}.p_cred_presumido`} render={({field}) => (
            <FormItem>
              <FormLabel>% Crédito</FormLabel>
              <NumericInput id={field.name} decimal integerPlaces={3} decimalPlaces={4}
                            value={field.value ?? ''} placeholder="0.0000" onChange={field.onChange}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <Button type="button" variant="ghost" size="xs" className="min-h-11" onClick={() => remove(index)}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}

// ─── Constants ────────────────────────────────────────────────────────────────

const IS_SIMPLES = isRegimeSimples

// Calculada uma vez por carga do módulo: a lista não muda durante a sessão.
const VEHICLE_YEAR_OPTIONS = vehicleYearOptions()

/** Literais do leiaute — nunca digitados, sempre escritos por um controle. */
const SEM_GTIN = 'SEM GTIN'
const ANVISA_ISENTO = 'ISENTO'

/** Tabela estática: recriar o array por render invalidava o memo do Combobox. */
const NFCE_CFOP_OPTIONS = getCfopOptionsForNfce()

const TABS: { id: ProductTab; label: string }[] = [
  {id: 'produto', label: 'Produto'},
  {id: 'unidades', label: 'Preços e Unidades'},
  {id: 'tributacao', label: 'Tributação'},
  {id: 'especial', label: 'Tipo Especial'},
]

/**
 * Em que aba mora cada campo. Sem isso, um erro de validação numa aba inativa é
 * invisível: o submit falha, nada muda na tela e o operador procura às cegas.
 */
const TAB_FIELDS: Record<Exclude<ProductTab, 'especial'>, readonly string[]> = {
  produto: [
    'code', 'description', 'brand', 'ncm', 'cest', 'origin', 'c_benef', 'ext_ipi',
    'ind_escala', 'cnpj_fab', 'ind_tot', 'inf_ad_prod',
  ],
  unidades: [
    'unit', 'taxable_unit', 'cean', 'taxable_cean', 'value', 'value_resale',
    'net_weight', 'gross_weight', 'conversion_factors',
  ],
  tributacao: ['cfop_nfce', 'cfop_config', 'icms_aliq_override', 'fcp_aliq_override'],
}

/** A aba "Tipo Especial" é o resto: tudo que não é identificação, preço ou tributação. */
function tabOfField(field: string): ProductTab {
  for (const [tab, fields] of Object.entries(TAB_FIELDS)) {
    if (fields.includes(field)) return tab as ProductTab
  }
  return 'especial'
}



const EMPTY_CFOP_ROW: CfopConfigFormData = {
  cfop: '',
  csosn: '',
  icms: '',
  icms_mod_bc: '3',
  icms_aliq_override: '',
  icms_fcp_override: '',
  icms_sn_cred_aliq: '',
  icms_ind_deduz_deson: '0',
  icms_st_mod_bc: '4',
  icms_st_mva: '',
  icms_st_red_bc: '',
  icms_st_aliq: '',
  icms_st_fcp_aliq: '',
  icms_p_red_bc: '',
  icms_mot_des: '',
  icms_p_dif: '',
  icms_pauta_valor: '',
  // ICMS monofásico combustíveis
  icms_ad_rem: '',
  icms_ad_rem_reten: '',
  icms_p_red_ad_rem: '',
  icms_mot_red_ad_rem: '',
  icms_p_dif_mono: '',
  // ICMS60 ST ret
  icms_v_bc_st_ret: '',
  icms_v_icms_st_ret: '',
  icms_p_st: '',
  icms_fcp_v_bc_st_ret: '',
  icms_fcp_st_ret_aliq: '',
  icms_v_bc_st_dest: '',
  icms_v_icms_st_dest: '',
  icms_p_red_bc_efet: '',
  icms_p_icms_efet: '',
  icms_part_p_bc_op: '',
  icms_part_uf_st: '',
  icms_mot_des_st: '',
  icms_p_fcp_dif: '',
  ipi_v_unid: '',
  pis: '',
  cofins: '',
  pis_aliq: '',
  cofins_aliq: '',
  pis_aliq_unid: '',
  cofins_aliq_unid: '',
  pis_st_aliq: '',
  cofins_st_aliq: '',
  pis_st_v_bc: '',
  cofins_st_v_bc: '',
  ipi_cst: '',
  ipi_aliq: '',
  is_cst: '',
  is_aliq: '',
  is_class_trib: '',
  is_aliq_espec: '',
  is_unid_trib: '',
  ibs_cbs_cst: '000',
  ibs_cbs_class_trib: '000001',
  ibs_uf_aliq: '0.1000',
  ibs_mun_aliq: '0.0000',
  cbs_aliq: '0.9000',
  ibs_uf_p_red: '',
  ibs_mun_p_red: '',
  cbs_p_red: '',
  ibs_uf_p_dif: '',
  ibs_mun_p_dif: '',
  cbs_p_dif: '',
  ibs_ind_doacao: '',
  ibs_ad_rem_reten: '', cbs_ad_rem_reten: '',
  ibs_ad_rem_ret: '', cbs_ad_rem_ret: '',
  ibs_p_dif_mono: '', cbs_p_dif_mono: '',
  ibs_reg_cst: '', ibs_reg_class_trib: '',
  ibs_reg_uf_aliq: '', ibs_reg_mun_aliq: '', cbs_reg_aliq: '',
  ibs_gov_uf_aliq: '', ibs_gov_mun_aliq: '', cbs_gov_aliq: '',
  ibs_cbs_c_cred_pres: '', ibs_p_cred_pres: '', cbs_p_cred_pres: '',
  ibs_cbs_cred_pres_cond_sus: '', ibs_zfm_p_cred_pres: '',
  alc_zfm_tp_cbs: '', alc_zfm_n_proc_suframa: '',
  ibs_ad_rem: '',
  cbs_ad_rem: '',
  ibs_cbs_p_dev_trib: '',
  // ISSQN
  issqn_ind_iss: '',
  issqn_c_list_serv: '',
  issqn_c_mun_fg: '',
  issqn_aliq: '',
  issqn_v_deducao: '',
  issqn_v_iss_ret: '',
  issqn_v_outro: '',
  issqn_v_desc_incond: '',
  issqn_v_desc_cond: '',
  issqn_c_servico: '',
  issqn_c_mun: '',
  issqn_c_pais: '',
  issqn_n_processo: '',
  issqn_ind_incentivo: '',
  obs_item_x_campo: '',
  obs_item_x_texto: '',
  uf_overrides: [],
}

const EMPTY_CONVERSION_ROW: ConversionFactorFormData = {
  origin_unit: '',
  target_unit: '',
  factor: '',
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function toFormData(p: ProductOut): ProductFormData {
  return {
    code: p.code,
    description: p.description,
    brand: p.brand ?? '',
    ncm: p.ncm,
    cest: p.cest ?? '',
    origin: p.origin ?? '0',
    unit: p.unit ?? 'UN',
    taxable_unit: p.taxable_unit ?? '',
    cean: p.cean ?? '',
    taxable_cean: p.taxable_cean ?? '',
    value: p.value,
    value_resale: p.value_resale ?? '',
    net_weight: p.net_weight ?? '',
    gross_weight: p.gross_weight ?? '',
    c_benef: p.c_benef ?? '',
    ext_ipi: p.ext_ipi ?? '',
    ind_escala: (p.ind_escala as 'S' | 'N' | '') ?? '',
    cnpj_fab: p.cnpj_fab ?? '',
    ind_tot: (p.ind_tot as '0' | '1') ?? '1',
    icms_aliq_override: p.icms_aliq_override ?? '',
    fcp_aliq_override: p.fcp_aliq_override ?? '',
    inf_ad_prod: p.inf_ad_prod ?? '',
    cfop_nfce: p.cfop_nfce,
    cfop_config: p.cfop_config.map((c) => ({
      ...EMPTY_CFOP_ROW,
      ...c,
      icms: c.icms ?? '',
      csosn: c.csosn ?? '',
      icms_mod_bc: c.icms_mod_bc ?? '3',
      icms_aliq_override: c.icms_aliq_override ?? '',
      icms_fcp_override: c.icms_fcp_override ?? '',
      icms_sn_cred_aliq: c.icms_sn_cred_aliq ?? '',
      icms_ind_deduz_deson: c.icms_ind_deduz_deson ?? '0',
      icms_st_mod_bc: c.icms_st_mod_bc ?? '4',
      icms_st_mva: c.icms_st_mva ?? '',
      icms_st_red_bc: c.icms_st_red_bc ?? '',
      icms_st_aliq: c.icms_st_aliq ?? '',
      icms_st_fcp_aliq: c.icms_st_fcp_aliq ?? '',
      icms_p_red_bc: c.icms_p_red_bc ?? '',
      icms_mot_des: c.icms_mot_des ?? '',
      icms_p_dif: c.icms_p_dif ?? '',
      icms_pauta_valor: c.icms_pauta_valor ?? '',
      icms_ad_rem: c.icms_ad_rem ?? '',
      icms_ad_rem_reten: c.icms_ad_rem_reten ?? '',
      icms_p_red_ad_rem: c.icms_p_red_ad_rem ?? '',
      icms_mot_red_ad_rem: c.icms_mot_red_ad_rem ?? '',
      icms_p_dif_mono: c.icms_p_dif_mono ?? '',
      icms_v_bc_st_ret: c.icms_v_bc_st_ret ?? '',
      icms_v_icms_st_ret: c.icms_v_icms_st_ret ?? '',
      icms_p_st: c.icms_p_st ?? '',
      icms_fcp_v_bc_st_ret: c.icms_fcp_v_bc_st_ret ?? '',
      icms_fcp_st_ret_aliq: c.icms_fcp_st_ret_aliq ?? '',
      icms_v_bc_st_dest: c.icms_v_bc_st_dest ?? '',
      icms_v_icms_st_dest: c.icms_v_icms_st_dest ?? '',
      icms_p_red_bc_efet: c.icms_p_red_bc_efet ?? '',
      icms_p_icms_efet: c.icms_p_icms_efet ?? '',
      icms_part_p_bc_op: c.icms_part_p_bc_op ?? '',
      icms_part_uf_st: c.icms_part_uf_st ?? '',
      icms_mot_des_st: c.icms_mot_des_st ?? '',
      icms_p_fcp_dif: c.icms_p_fcp_dif ?? '',
      ipi_v_unid: c.ipi_v_unid ?? '',
      pis_aliq: c.pis_aliq ?? '',
      cofins_aliq: c.cofins_aliq ?? '',
      pis_aliq_unid: c.pis_aliq_unid ?? '',
      cofins_aliq_unid: c.cofins_aliq_unid ?? '',
      pis_st_aliq: c.pis_st_aliq ?? '',
      cofins_st_aliq: c.cofins_st_aliq ?? '',
      pis_st_v_bc: c.pis_st_v_bc ?? '',
      cofins_st_v_bc: c.cofins_st_v_bc ?? '',
      ipi_cst: c.ipi_cst ?? '',
      ipi_aliq: c.ipi_aliq ?? '',
      is_cst: c.is_cst ?? '',
      is_aliq: c.is_aliq ?? '',
      is_class_trib: c.is_class_trib ?? '',
      is_aliq_espec: c.is_aliq_espec ?? '',
      is_unid_trib: c.is_unid_trib ?? '',
      ibs_cbs_cst: c.ibs_cbs_cst ?? '',
      ibs_cbs_class_trib: c.ibs_cbs_class_trib ?? '',
      ibs_uf_aliq: c.ibs_uf_aliq ?? '',
      ibs_mun_aliq: c.ibs_mun_aliq ?? '',
      cbs_aliq: c.cbs_aliq ?? '',
      ibs_uf_p_red: c.ibs_uf_p_red ?? '',
      ibs_mun_p_red: c.ibs_mun_p_red ?? '',
      cbs_p_red: c.cbs_p_red ?? '',
      ibs_uf_p_dif: c.ibs_uf_p_dif ?? '',
      ibs_mun_p_dif: c.ibs_mun_p_dif ?? '',
      cbs_p_dif: c.cbs_p_dif ?? '',
      ibs_ind_doacao: (c.ibs_ind_doacao ?? '') as CfopConfigFormData['ibs_ind_doacao'],
      ibs_ad_rem: c.ibs_ad_rem ?? '',
      cbs_ad_rem: c.cbs_ad_rem ?? '',
      ibs_ad_rem_reten: c.ibs_ad_rem_reten ?? '',
      cbs_ad_rem_reten: c.cbs_ad_rem_reten ?? '',
      ibs_ad_rem_ret: c.ibs_ad_rem_ret ?? '',
      cbs_ad_rem_ret: c.cbs_ad_rem_ret ?? '',
      ibs_p_dif_mono: c.ibs_p_dif_mono ?? '',
      cbs_p_dif_mono: c.cbs_p_dif_mono ?? '',
      ibs_cbs_p_dev_trib: c.ibs_cbs_p_dev_trib ?? '',
      ibs_reg_cst: c.ibs_reg_cst ?? '',
      ibs_reg_class_trib: c.ibs_reg_class_trib ?? '',
      ibs_reg_uf_aliq: c.ibs_reg_uf_aliq ?? '',
      ibs_reg_mun_aliq: c.ibs_reg_mun_aliq ?? '',
      cbs_reg_aliq: c.cbs_reg_aliq ?? '',
      ibs_gov_uf_aliq: c.ibs_gov_uf_aliq ?? '',
      ibs_gov_mun_aliq: c.ibs_gov_mun_aliq ?? '',
      cbs_gov_aliq: c.cbs_gov_aliq ?? '',
      ibs_cbs_c_cred_pres: c.ibs_cbs_c_cred_pres ?? '',
      ibs_p_cred_pres: c.ibs_p_cred_pres ?? '',
      cbs_p_cred_pres: c.cbs_p_cred_pres ?? '',
      ibs_cbs_cred_pres_cond_sus:
        (c.ibs_cbs_cred_pres_cond_sus ?? '') as CfopConfigFormData['ibs_cbs_cred_pres_cond_sus'],
      ibs_zfm_p_cred_pres: c.ibs_zfm_p_cred_pres ?? '',
      alc_zfm_tp_cbs: (c.alc_zfm_tp_cbs ?? '') as CfopConfigFormData['alc_zfm_tp_cbs'],
      alc_zfm_n_proc_suframa: c.alc_zfm_n_proc_suframa ?? '',
      issqn_ind_iss: c.issqn_ind_iss ?? '',
      issqn_c_list_serv: c.issqn_c_list_serv ?? '',
      issqn_c_mun_fg: c.issqn_c_mun_fg ?? '',
      issqn_aliq: c.issqn_aliq ?? '',
      issqn_v_deducao: c.issqn_v_deducao ?? '',
      issqn_v_iss_ret: c.issqn_v_iss_ret ?? '',
      issqn_v_outro: c.issqn_v_outro ?? '',
      issqn_v_desc_incond: c.issqn_v_desc_incond ?? '',
      issqn_v_desc_cond: c.issqn_v_desc_cond ?? '',
      issqn_c_servico: c.issqn_c_servico ?? '',
      issqn_c_mun: c.issqn_c_mun ?? '',
      issqn_c_pais: c.issqn_c_pais ?? '',
      issqn_n_processo: c.issqn_n_processo ?? '',
      issqn_ind_incentivo: c.issqn_ind_incentivo ?? '',
      obs_item_x_campo: c.obs_item_x_campo ?? '',
      obs_item_x_texto: c.obs_item_x_texto ?? '',
      uf_overrides: c.uf_overrides ?? [],
    })),
    conversion_factors: (p.conversion_factors ?? []).map((f) => ({
      origin_unit: f.origin_unit,
      target_unit: f.target_unit,
      factor: String(f.factor),
    })),
    prod_type: (p.prod_type as 'generic' | 'comb' | 'med' | 'veiculo' | 'arma' | '') ?? '',
    comb_c_prod_anp: p.comb_c_prod_anp ?? '',
    comb_desc_anp: p.comb_desc_anp ?? '',
    comb_uf_cons: p.comb_uf_cons ?? '',
    comb_codif: p.comb_codif ?? '',
    comb_p_glp: p.comb_p_glp ?? '',
    comb_p_gnn: p.comb_p_gnn ?? '',
    comb_p_gni: p.comb_p_gni ?? '',
    comb_v_part: p.comb_v_part ?? '',
    comb_p_bio: p.comb_p_bio ?? '',
    comb_cide_v_aliq_prod: p.comb_cide_v_aliq_prod ?? '',
    comb_orig: Array.isArray(p.comb_orig) ? (p.comb_orig as ProductFormData['comb_orig']) : [],
    med_c_prod_anvisa: p.med_c_prod_anvisa ?? '',
    med_x_motivo_isencao: p.med_x_motivo_isencao ?? '',
    med_v_pmc: p.med_v_pmc ?? '',
    nve: Array.isArray(p.nve) ? (p.nve as string[]) : [],
    n_fci: p.n_fci ?? '',
    c_barra: p.c_barra ?? '',
    c_barra_trib: p.c_barra_trib ?? '',
    n_recopi: p.n_recopi ?? '',
    gcred: Array.isArray(p.gcred) ? (p.gcred as ProductFormData['gcred']) : [],
    tp_cred_pres_ibs_zfm: (p.tp_cred_pres_ibs_zfm ?? '') as ProductFormData['tp_cred_pres_ibs_zfm'],
    ind_bem_movel_usado: (p.ind_bem_movel_usado ?? '') as ProductFormData['ind_bem_movel_usado'],
    ipi_cnpj_prod: p.ipi_cnpj_prod ?? '',
    ipi_c_selo: p.ipi_c_selo ?? '',
    ipi_q_selo: p.ipi_q_selo ?? '',
    ipi_c_enq: p.ipi_c_enq ?? '',
    peri_n_onu: p.peri_n_onu ?? '',
    peri_x_nome_ae: p.peri_x_nome_ae ?? '',
    peri_x_cla_risco: p.peri_x_cla_risco ?? '',
    peri_gr_emb: p.peri_gr_emb ?? '',
    peri_q_vol_tipo: p.peri_q_vol_tipo ?? '',
    veic_tp_op: p.veic_tp_op ?? '',
    veic_tp_comb: p.veic_tp_comb ?? '',
    veic_tp_pint: p.veic_tp_pint ?? '',
    veic_tp_veic: p.veic_tp_veic ?? '',
    veic_esp_veic: p.veic_esp_veic ?? '',
    veic_vin: p.veic_vin ?? '',
    veic_cond_veic: p.veic_cond_veic ?? '',
    veic_c_mod: p.veic_c_mod ?? '',
    veic_c_cor_denatran: p.veic_c_cor_denatran ?? '',
    veic_lota: p.veic_lota ?? '',
    veic_tp_rest: p.veic_tp_rest ?? '',
    veic_ano_mod: p.veic_ano_mod ?? '',
    veic_ano_fab: p.veic_ano_fab ?? '',
    veic_pot: p.veic_pot ?? '',
    veic_cilin: p.veic_cilin ?? '',
    veic_cmt: p.veic_cmt ?? '',
    veic_dist: p.veic_dist ?? '',
    veic_c_cor: p.veic_c_cor ?? '',
    veic_x_cor: p.veic_x_cor ?? '',
    arma_tp_arma: p.arma_tp_arma ?? '',
    arma_descr: p.arma_descr ?? '',
  }
}

function toApiPayload(data: ProductFormData): ProductCreate {
  const nullify = (v: string | undefined) => (v ? v : null)
  const hasDifferentUnits = data.unit && data.taxable_unit && data.unit !== data.taxable_unit
  return {
    code: data.code,
    description: data.description,
    brand: nullify(data.brand),
    ncm: data.ncm,
    cest: nullify(data.cest),
    origin: data.origin,
    unit: data.unit,
    taxable_unit: nullify(data.taxable_unit),
    cean: nullify(data.cean),
    taxable_cean: nullify(data.taxable_cean),
    value: data.value,
    value_resale: nullify(data.value_resale),
    net_weight: nullify(data.net_weight),
    gross_weight: nullify(data.gross_weight),
    c_benef: nullify(data.c_benef),
    ext_ipi: nullify(data.ext_ipi),
    ind_escala: nullify(data.ind_escala),
    cnpj_fab: nullify(data.cnpj_fab),
    ind_tot: data.ind_tot,
    icms_aliq_override: nullify(data.icms_aliq_override),
    fcp_aliq_override: nullify(data.fcp_aliq_override),
    inf_ad_prod: nullify(data.inf_ad_prod),
    cfop_nfce: data.cfop_nfce,
    cfop_config: data.cfop_config.map((c) => ({
      cfop: c.cfop,
      csosn: nullify(c.csosn),
      icms: nullify(c.icms),
      icms_mod_bc: nullify(c.icms_mod_bc),
      icms_aliq_override: nullify(c.icms_aliq_override),
      icms_fcp_override: nullify(c.icms_fcp_override),
      icms_sn_cred_aliq: nullify(c.icms_sn_cred_aliq),
      icms_ind_deduz_deson: nullify(c.icms_ind_deduz_deson),
      icms_st_mod_bc: nullify(c.icms_st_mod_bc),
      icms_st_mva: nullify(c.icms_st_mva),
      icms_st_red_bc: nullify(c.icms_st_red_bc),
      icms_st_aliq: nullify(c.icms_st_aliq),
      icms_st_fcp_aliq: nullify(c.icms_st_fcp_aliq),
      icms_p_red_bc: nullify(c.icms_p_red_bc),
      icms_mot_des: nullify(c.icms_mot_des),
      icms_p_dif: nullify(c.icms_p_dif),
      icms_pauta_valor: nullify(c.icms_pauta_valor),
      pis: c.pis,
      cofins: c.cofins,
      pis_aliq: nullify(c.pis_aliq),
      cofins_aliq: nullify(c.cofins_aliq),
      pis_aliq_unid: nullify(c.pis_aliq_unid),
      cofins_aliq_unid: nullify(c.cofins_aliq_unid),
      pis_st_aliq: nullify(c.pis_st_aliq),
      cofins_st_aliq: nullify(c.cofins_st_aliq),
      pis_st_v_bc: nullify(c.pis_st_v_bc),
      cofins_st_v_bc: nullify(c.cofins_st_v_bc),
      ibs_cbs_cst: nullify(c.ibs_cbs_cst),
      ibs_cbs_class_trib: nullify(c.ibs_cbs_class_trib),
      ibs_uf_aliq: nullify(c.ibs_uf_aliq),
      ibs_mun_aliq: nullify(c.ibs_mun_aliq),
      cbs_aliq: nullify(c.cbs_aliq),
      ibs_uf_p_red: nullify(c.ibs_uf_p_red),
      ibs_mun_p_red: nullify(c.ibs_mun_p_red),
      cbs_p_red: nullify(c.cbs_p_red),
      ibs_uf_p_dif: nullify(c.ibs_uf_p_dif),
      ibs_mun_p_dif: nullify(c.ibs_mun_p_dif),
      cbs_p_dif: nullify(c.cbs_p_dif),
      ibs_ind_doacao: nullify(c.ibs_ind_doacao),
      ibs_ad_rem_reten: nullify(c.ibs_ad_rem_reten),
      cbs_ad_rem_reten: nullify(c.cbs_ad_rem_reten),
      ibs_ad_rem_ret: nullify(c.ibs_ad_rem_ret),
      cbs_ad_rem_ret: nullify(c.cbs_ad_rem_ret),
      ibs_p_dif_mono: nullify(c.ibs_p_dif_mono),
      cbs_p_dif_mono: nullify(c.cbs_p_dif_mono),
      ibs_reg_cst: nullify(c.ibs_reg_cst),
      ibs_reg_class_trib: nullify(c.ibs_reg_class_trib),
      ibs_reg_uf_aliq: nullify(c.ibs_reg_uf_aliq),
      ibs_reg_mun_aliq: nullify(c.ibs_reg_mun_aliq),
      cbs_reg_aliq: nullify(c.cbs_reg_aliq),
      ibs_gov_uf_aliq: nullify(c.ibs_gov_uf_aliq),
      ibs_gov_mun_aliq: nullify(c.ibs_gov_mun_aliq),
      cbs_gov_aliq: nullify(c.cbs_gov_aliq),
      ibs_cbs_c_cred_pres: nullify(c.ibs_cbs_c_cred_pres),
      ibs_p_cred_pres: nullify(c.ibs_p_cred_pres),
      cbs_p_cred_pres: nullify(c.cbs_p_cred_pres),
      ibs_cbs_cred_pres_cond_sus: nullify(c.ibs_cbs_cred_pres_cond_sus),
      ibs_zfm_p_cred_pres: nullify(c.ibs_zfm_p_cred_pres),
      alc_zfm_tp_cbs: nullify(c.alc_zfm_tp_cbs),
      alc_zfm_n_proc_suframa: nullify(c.alc_zfm_n_proc_suframa),
      ibs_ad_rem: nullify(c.ibs_ad_rem),
      cbs_ad_rem: nullify(c.cbs_ad_rem),
      ibs_cbs_p_dev_trib: nullify(c.ibs_cbs_p_dev_trib),
      uf_overrides: c.uf_overrides,
      ipi_cst: nullify(c.ipi_cst),
      ipi_aliq: nullify(c.ipi_aliq),
      is_cst: nullify(c.is_cst),
      is_aliq: nullify(c.is_aliq),
      is_class_trib: nullify(c.is_class_trib),
      is_aliq_espec: nullify(c.is_aliq_espec),
      is_unid_trib: nullify(c.is_unid_trib),
      // ICMS monofásico
      icms_ad_rem: nullify(c.icms_ad_rem),
      icms_ad_rem_reten: nullify(c.icms_ad_rem_reten),
      icms_p_red_ad_rem: nullify(c.icms_p_red_ad_rem),
      icms_mot_red_ad_rem: nullify(c.icms_mot_red_ad_rem),
      icms_p_dif_mono: nullify(c.icms_p_dif_mono),
      icms_v_bc_st_ret: nullify(c.icms_v_bc_st_ret),
      icms_v_icms_st_ret: nullify(c.icms_v_icms_st_ret),
      icms_p_st: nullify(c.icms_p_st),
      icms_fcp_v_bc_st_ret: nullify(c.icms_fcp_v_bc_st_ret),
      icms_fcp_st_ret_aliq: nullify(c.icms_fcp_st_ret_aliq),
      icms_v_bc_st_dest: nullify(c.icms_v_bc_st_dest),
      icms_v_icms_st_dest: nullify(c.icms_v_icms_st_dest),
      icms_p_red_bc_efet: nullify(c.icms_p_red_bc_efet),
      icms_p_icms_efet: nullify(c.icms_p_icms_efet),
      icms_part_p_bc_op: nullify(c.icms_part_p_bc_op),
      icms_part_uf_st: nullify(c.icms_part_uf_st),
      icms_mot_des_st: nullify(c.icms_mot_des_st),
      icms_p_fcp_dif: nullify(c.icms_p_fcp_dif),
      ipi_v_unid: nullify(c.ipi_v_unid),
      // ISSQN
      issqn_ind_iss: nullify(c.issqn_ind_iss),
      issqn_c_list_serv: nullify(c.issqn_c_list_serv),
      issqn_c_mun_fg: nullify(c.issqn_c_mun_fg),
      issqn_aliq: nullify(c.issqn_aliq),
      issqn_v_deducao: nullify(c.issqn_v_deducao),
      issqn_v_iss_ret: nullify(c.issqn_v_iss_ret),
      issqn_v_outro: nullify(c.issqn_v_outro),
      issqn_v_desc_incond: nullify(c.issqn_v_desc_incond),
      issqn_v_desc_cond: nullify(c.issqn_v_desc_cond),
      issqn_c_servico: nullify(c.issqn_c_servico),
      issqn_c_mun: nullify(c.issqn_c_mun),
      issqn_c_pais: nullify(c.issqn_c_pais),
      issqn_n_processo: nullify(c.issqn_n_processo),
      issqn_ind_incentivo: nullify(c.issqn_ind_incentivo),
      obs_item_x_campo: nullify(c.obs_item_x_campo),
      obs_item_x_texto: nullify(c.obs_item_x_texto),
    })),
    conversion_factors: hasDifferentUnits && data.conversion_factors.length > 0
      ? data.conversion_factors.map((f) => ({
        origin_unit: f.origin_unit,
        target_unit: f.target_unit,
        factor: parseFloat(f.factor),
      }))
      : [],
    prod_type: nullify(data.prod_type),
    comb_c_prod_anp: nullify(data.comb_c_prod_anp),
    comb_desc_anp: nullify(data.comb_desc_anp),
    comb_uf_cons: nullify(data.comb_uf_cons),
    comb_codif: nullify(data.comb_codif),
    comb_p_glp: nullify(data.comb_p_glp),
    comb_p_gnn: nullify(data.comb_p_gnn),
    comb_p_gni: nullify(data.comb_p_gni),
    comb_v_part: nullify(data.comb_v_part),
    comb_p_bio: nullify(data.comb_p_bio),
    comb_cide_v_aliq_prod: nullify(data.comb_cide_v_aliq_prod),
    comb_orig: data.comb_orig?.length ? data.comb_orig : null,
    med_c_prod_anvisa: nullify(data.med_c_prod_anvisa),
    med_x_motivo_isencao: nullify(data.med_x_motivo_isencao),
    med_v_pmc: nullify(data.med_v_pmc),
    nve: data.nve?.length ? data.nve : null,
    gcred: data.gcred?.length ? data.gcred : null,
    tp_cred_pres_ibs_zfm: nullify(data.tp_cred_pres_ibs_zfm),
    ind_bem_movel_usado: nullify(data.ind_bem_movel_usado),
    n_fci: nullify(data.n_fci),
    c_barra: nullify(data.c_barra),
    c_barra_trib: nullify(data.c_barra_trib),
    n_recopi: nullify(data.n_recopi),
    ipi_cnpj_prod: nullify(data.ipi_cnpj_prod),
    ipi_c_selo: nullify(data.ipi_c_selo),
    ipi_q_selo: nullify(data.ipi_q_selo),
    ipi_c_enq: nullify(data.ipi_c_enq),
    peri_n_onu: nullify(data.peri_n_onu),
    peri_x_nome_ae: nullify(data.peri_x_nome_ae),
    peri_x_cla_risco: nullify(data.peri_x_cla_risco),
    peri_gr_emb: nullify(data.peri_gr_emb),
    peri_q_vol_tipo: nullify(data.peri_q_vol_tipo),
    veic_tp_op: nullify(data.veic_tp_op),
    veic_tp_comb: nullify(data.veic_tp_comb),
    veic_tp_pint: nullify(data.veic_tp_pint),
    veic_tp_veic: nullify(data.veic_tp_veic),
    veic_esp_veic: nullify(data.veic_esp_veic),
    veic_vin: nullify(data.veic_vin),
    veic_cond_veic: nullify(data.veic_cond_veic),
    veic_c_mod: nullify(data.veic_c_mod),
    veic_c_cor_denatran: nullify(data.veic_c_cor_denatran),
    veic_lota: nullify(data.veic_lota),
    veic_tp_rest: nullify(data.veic_tp_rest),
    veic_ano_mod: nullify(data.veic_ano_mod),
    veic_ano_fab: nullify(data.veic_ano_fab),
    veic_pot: nullify(data.veic_pot),
    veic_cilin: nullify(data.veic_cilin),
    veic_cmt: nullify(data.veic_cmt),
    veic_dist: nullify(data.veic_dist),
    veic_c_cor: nullify(data.veic_c_cor),
    veic_x_cor: nullify(data.veic_x_cor),
    arma_tp_arma: nullify(data.arma_tp_arma),
    arma_descr: nullify(data.arma_descr),
  }
}

// ─── Component ────────────────────────────────────────────────────────────────

export function ProductForm({initialData, crt = 3, uf, onSubmit, loading = false}: ProductFormProps) {
  const {selectedOrg} = useAuth()
  const [activeTab, setActiveTab] = useState<ProductTab>('produto')
  const [cfopRow, setCfopRow] = useState<CfopConfigFormData>(EMPTY_CFOP_ROW)
  const [cfopError, setCfopError] = useState<string | null>(null)
  // Remonta o TaxFieldsEditor para zerar seus toggles internos quando
  // uma linha de CFOP é adicionada à lista.
  const [taxEditorKey, setTaxEditorKey] = useState(0)
  const [taxGroups, setTaxGroups] = useState<TaxGroups>(EMPTY_TAX_GROUPS)
  // Perfis fiscais vinculados. Fora do zod de propósito: o productSchema já é
  // grande o bastante para que mais um array aninhado estoure a inferência do
  // resolver do react-hook-form. É só uma lista de ids — não há o que validar
  // aqui que o backend não valide.
  const [taxProfileIds, setTaxProfileIds] = useState<string[]>(
    () => (initialData?.tax_profiles ?? []).map((r) => r.tax_profile_id),
  )

  const {data: taxProfilePage} = useQuery({
    queryKey: queryKeys.taxProfiles.list(selectedOrg?.pk),
    queryFn: () => apiClient.getTaxProfiles({limit: 100}),
    enabled: !!selectedOrg,
  })
  const taxProfiles = taxProfilePage?.items ?? []
  const [ufOverrideRows, setUfOverrideRows] = useState<UfOverrideFormData[]>([])
  const [convRow, setConvRow] = useState<ConversionFactorFormData>(EMPTY_CONVERSION_ROW)
  const [convError, setConvError] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)
  // Código é identificação interna: gerado por padrão, editável (lib/utils/code.ts).
  const [defaultCode] = useState(generateEntityCode)


  const simples = IS_SIMPLES(crt)

  const form = useForm<ProductFormData>({
    resolver: zodResolver(productSchema),
    defaultValues: initialData ? toFormData(initialData) : {
      code: defaultCode,
      description: '',
      brand: '',
      ncm: '',
      cest: '',
      origin: '0',
      unit: 'UN',
      taxable_unit: 'UN',
      cean: SEM_GTIN,
      taxable_cean: SEM_GTIN,
      value: '',
      value_resale: '',
      net_weight: '',
      gross_weight: '',
      c_benef: '',
      ext_ipi: '',
      ind_escala: '',
      cnpj_fab: '',
      ind_tot: '1',
      icms_aliq_override: '',
      fcp_aliq_override: '',
      inf_ad_prod: '',
      cfop_nfce: '',
      cfop_config: [],
      conversion_factors: [],
      prod_type: '',
      comb_c_prod_anp: '', comb_desc_anp: '', comb_uf_cons: '', comb_codif: '',
      comb_p_glp: '', comb_p_gnn: '', comb_p_gni: '', comb_v_part: '', comb_p_bio: '',
      comb_cide_v_aliq_prod: '', comb_orig: [],
      med_c_prod_anvisa: '', med_x_motivo_isencao: '', med_v_pmc: '',
      nve: [], n_fci: '', c_barra: '', c_barra_trib: '', n_recopi: '',
      gcred: [], tp_cred_pres_ibs_zfm: '', ind_bem_movel_usado: '',
      ipi_cnpj_prod: '', ipi_c_selo: '', ipi_q_selo: '', ipi_c_enq: '',
      peri_n_onu: '', peri_x_nome_ae: '', peri_x_cla_risco: '', peri_gr_emb: '', peri_q_vol_tipo: '',
      veic_tp_op: '', veic_tp_comb: '', veic_tp_pint: '', veic_tp_veic: '',
      veic_esp_veic: '', veic_vin: '', veic_cond_veic: '', veic_c_mod: '',
      veic_c_cor_denatran: '', veic_lota: '', veic_tp_rest: '', veic_ano_mod: '',
      veic_ano_fab: '', veic_pot: '', veic_cilin: '', veic_cmt: '', veic_dist: '',
      veic_c_cor: '', veic_x_cor: '',
      arma_tp_arma: '', arma_descr: '',
    },
  })

  const cfopConfig = useWatch({control: form.control, name: 'cfop_config'})
  const conversionFactors = useWatch({control: form.control, name: 'conversion_factors'})
  const watchedUnit = useWatch({control: form.control, name: 'unit'})
  const watchedTaxableUnit = useWatch({control: form.control, name: 'taxable_unit'})
  const watchedCest = useWatch({control: form.control, name: 'cest'})
  const watchedNcm = useWatch({control: form.control, name: 'ncm'})
  const watchedProdType = useWatch({control: form.control, name: 'prod_type'})
  const watchedCombOrig = useWatch({control: form.control, name: 'comb_orig'})
  const showConversionFactors = !!watchedUnit && !!watchedTaxableUnit && watchedUnit !== watchedTaxableUnit

  const [prevShowConvFact, setPrevShowConvFact] = useState(showConversionFactors)
  if (prevShowConvFact !== showConversionFactors) {
    setPrevShowConvFact(showConversionFactors)
    if (showConversionFactors) {
      setConvRow(r => ({...r, origin_unit: watchedUnit ?? '', target_unit: watchedTaxableUnit ?? ''}))
    } else {
      setConvRow(EMPTY_CONVERSION_ROW)
    }
  }

  // Referência do sistema para o override de ICMS/FCP a nível de produto —
  // mostra o valor que o backend resolveria e avisa quando o override
  // digitado diverge dele. Sem autopreenchimento: o campo fica vazio até o
  // usuário digitar algo (design spec 2026-08-09-tax-config-redesign
  // §Modelo de dados 6).
  const productSystemAliq = useIcmsAliqPreview(uf, uf, watchedNcm)
  const watchedIcmsOverride = useWatch({control: form.control, name: 'icms_aliq_override'})
  const productAliqDiverges = !!productSystemAliq && !!watchedIcmsOverride &&
    watchedIcmsOverride !== productSystemAliq.icms_aliq


  const {showPRedBC, showMotDeSon, showPDif} = icmsConditionalFields(cfopRow.icms ?? '')


  // ─── CFOP handlers ──────────────────────────────────────────────────────────

  const addCfop = () => {
    if (!cfopRow.cfop || !/^\d{4}$/.test(cfopRow.cfop)) {
      setCfopError('CFOP deve ter 4 dígitos')
      return
    }
    if (simples && !cfopRow.csosn) {
      setCfopError('CSOSN obrigatório para Simples Nacional')
      return
    }
    if (!simples && !cfopRow.icms && !taxGroups.issqn) {
      setCfopError('ICMS CST obrigatório para Regime Normal (ou habilite ISSQN)')
      return
    }
    if (!cfopRow.pis || !cfopRow.cofins) {
      setCfopError('PIS e COFINS são obrigatórios')
      return
    }
    if (!simples && showMotDeSon && !cfopRow.icms_mot_des) {
      setCfopError('Motivo de desoneração obrigatório para este CST')
      return
    }
    if (taxGroups.ipi && !cfopRow.ipi_cst) {
      setCfopError('CST IPI obrigatório quando IPI está habilitado')
      return
    }
    if (taxGroups.is && !cfopRow.is_cst) {
      setCfopError('CST IS obrigatório quando IS está habilitado')
      return
    }
    if (taxGroups.issqn && !cfopRow.issqn_ind_iss) {
      setCfopError('Exigibilidade ISS obrigatória quando ISSQN está habilitado')
      return
    }

    // Regras de grupo do leiaute (IPI, ICMSPart, pauta, ALC/ZFM, obsItem) vivem
    // no schema, para valerem também no perfil fiscal e no salvamento.
    const groupCheck = cfopConfigSchema.safeParse({...cfopRow, cfop: cfopRow.cfop})
    if (!groupCheck.success) {
      setCfopError(groupCheck.error.issues[0].message)
      return
    }

    const hasSt = (!simples && ICMS_ST_CSTS.has(cfopRow.icms ?? '')) ||
      (simples && CSOSN_ST.has(cfopRow.csosn ?? ''))
    const isMono = !simples && ICMS_MONO_CSTS.has(cfopRow.icms ?? '')

    const newCfopConfigs = getCfopVariants(cfopRow.cfop as never).map(variant => {
      return {
        ...cfopRow,
        cfop: variant,
        csosn: simples ? (cfopRow.csosn ?? '') : '',
        icms: simples ? '' : (cfopRow.icms ?? ''),
        // clear optional IPI/IS if not enabled
        ipi_cst: taxGroups.ipi ? cfopRow.ipi_cst : '',
        ipi_aliq: taxGroups.ipi ? cfopRow.ipi_aliq : '',
        is_cst: taxGroups.is ? cfopRow.is_cst : '',
        is_aliq: taxGroups.is ? cfopRow.is_aliq : '',
        is_class_trib: taxGroups.is ? cfopRow.is_class_trib : '',
        is_aliq_espec: taxGroups.is ? cfopRow.is_aliq_espec : '',
        is_unid_trib: taxGroups.is ? cfopRow.is_unid_trib : '',
        // clear conditional ICMS fields that don't apply to selected CST
        icms_p_red_bc: showPRedBC ? cfopRow.icms_p_red_bc : '',
        icms_mot_des: showMotDeSon ? cfopRow.icms_mot_des : '',
        icms_p_dif: showPDif ? cfopRow.icms_p_dif : '',
        // clear ST fields if not applicable
        icms_st_mod_bc: hasSt ? cfopRow.icms_st_mod_bc : '',
        icms_st_mva: hasSt ? cfopRow.icms_st_mva : '',
        icms_st_red_bc: hasSt ? cfopRow.icms_st_red_bc : '',
        icms_st_aliq: hasSt ? cfopRow.icms_st_aliq : '',
        icms_st_fcp_aliq: hasSt ? cfopRow.icms_st_fcp_aliq : '',
        // clear monofásico fields if not applicable
        icms_ad_rem: isMono || taxGroups.icmsMono ? cfopRow.icms_ad_rem : '',
        icms_ad_rem_reten: (isMono && cfopRow.icms === '15') ? cfopRow.icms_ad_rem_reten : '',
        icms_p_red_ad_rem: (isMono && cfopRow.icms === '15') ? cfopRow.icms_p_red_ad_rem : '',
        icms_mot_red_ad_rem: (isMono && cfopRow.icms === '15') ? cfopRow.icms_mot_red_ad_rem : '',
        icms_p_dif_mono: (isMono && cfopRow.icms === '53') ? cfopRow.icms_p_dif_mono : '',
        // clear pis/cofins alíquotas if not applicable
        pis_aliq: PIS_COFINS_ALIQ_CSTS.has(cfopRow.pis) ? cfopRow.pis_aliq : '',
        cofins_aliq: PIS_COFINS_ALIQ_CSTS.has(cfopRow.cofins) ? cfopRow.cofins_aliq : '',
        pis_aliq_unid: cfopRow.pis === '03' ? cfopRow.pis_aliq_unid : '',
        cofins_aliq_unid: cfopRow.cofins === '03' ? cfopRow.cofins_aliq_unid : '',
        // clear IBS/CBS entirely if the group is off (opcional — Task 8)
        ibs_cbs_cst: taxGroups.ibsCbs ? cfopRow.ibs_cbs_cst : '',
        ibs_cbs_class_trib: taxGroups.ibsCbs ? cfopRow.ibs_cbs_class_trib : '',
        ibs_uf_aliq: taxGroups.ibsCbs ? cfopRow.ibs_uf_aliq : '',
        ibs_mun_aliq: taxGroups.ibsCbs ? cfopRow.ibs_mun_aliq : '',
        cbs_aliq: taxGroups.ibsCbs ? cfopRow.cbs_aliq : '',
        ibs_cbs_p_dev_trib: taxGroups.ibsCbs ? cfopRow.ibs_cbs_p_dev_trib : '',
        ibs_ind_doacao: taxGroups.ibsCbs ? cfopRow.ibs_ind_doacao : '',
        ibs_ad_rem: taxGroups.ibsCbs ? cfopRow.ibs_ad_rem : '',
        cbs_ad_rem: taxGroups.ibsCbs ? cfopRow.cbs_ad_rem : '',
        // clear IBS/CBS advanced reform groups if IBS/CBS itself is off
        ibs_ad_rem_reten: taxGroups.ibsCbs ? cfopRow.ibs_ad_rem_reten : '',
        cbs_ad_rem_reten: taxGroups.ibsCbs ? cfopRow.cbs_ad_rem_reten : '',
        ibs_ad_rem_ret: taxGroups.ibsCbs ? cfopRow.ibs_ad_rem_ret : '',
        cbs_ad_rem_ret: taxGroups.ibsCbs ? cfopRow.cbs_ad_rem_ret : '',
        ibs_p_dif_mono: taxGroups.ibsCbs ? cfopRow.ibs_p_dif_mono : '',
        cbs_p_dif_mono: taxGroups.ibsCbs ? cfopRow.cbs_p_dif_mono : '',
        ibs_reg_cst: taxGroups.ibsCbs ? cfopRow.ibs_reg_cst : '',
        ibs_reg_class_trib: taxGroups.ibsCbs ? cfopRow.ibs_reg_class_trib : '',
        ibs_reg_uf_aliq: taxGroups.ibsCbs ? cfopRow.ibs_reg_uf_aliq : '',
        ibs_reg_mun_aliq: taxGroups.ibsCbs ? cfopRow.ibs_reg_mun_aliq : '',
        cbs_reg_aliq: taxGroups.ibsCbs ? cfopRow.cbs_reg_aliq : '',
        ibs_gov_uf_aliq: taxGroups.ibsCbs ? cfopRow.ibs_gov_uf_aliq : '',
        ibs_gov_mun_aliq: taxGroups.ibsCbs ? cfopRow.ibs_gov_mun_aliq : '',
        cbs_gov_aliq: taxGroups.ibsCbs ? cfopRow.cbs_gov_aliq : '',
        ibs_cbs_c_cred_pres: taxGroups.ibsCbs ? cfopRow.ibs_cbs_c_cred_pres : '',
        ibs_p_cred_pres: taxGroups.ibsCbs ? cfopRow.ibs_p_cred_pres : '',
        cbs_p_cred_pres: taxGroups.ibsCbs ? cfopRow.cbs_p_cred_pres : '',
        ibs_cbs_cred_pres_cond_sus: taxGroups.ibsCbs ? cfopRow.ibs_cbs_cred_pres_cond_sus : '',
        ibs_zfm_p_cred_pres: taxGroups.ibsCbs ? cfopRow.ibs_zfm_p_cred_pres : '',
        alc_zfm_tp_cbs: taxGroups.ibsCbs ? cfopRow.alc_zfm_tp_cbs : '',
        alc_zfm_n_proc_suframa: taxGroups.ibsCbs ? cfopRow.alc_zfm_n_proc_suframa : '',
        // clear IBS/CBS reduction/deferral if not enabled
        ibs_uf_p_red: taxGroups.ibsCbs && taxGroups.ibsRed ? cfopRow.ibs_uf_p_red : '',
        ibs_mun_p_red: taxGroups.ibsCbs && taxGroups.ibsRed ? cfopRow.ibs_mun_p_red : '',
        cbs_p_red: taxGroups.ibsCbs && taxGroups.ibsRed ? cfopRow.cbs_p_red : '',
        ibs_uf_p_dif: taxGroups.ibsCbs && taxGroups.ibsDif ? cfopRow.ibs_uf_p_dif : '',
        ibs_mun_p_dif: taxGroups.ibsCbs && taxGroups.ibsDif ? cfopRow.ibs_mun_p_dif : '',
        cbs_p_dif: taxGroups.ibsCbs && taxGroups.ibsDif ? cfopRow.cbs_p_dif : '',
        // clear PIS/COFINS-ST if not enabled
        pis_st_aliq: taxGroups.pisCofinsSt ? cfopRow.pis_st_aliq : '',
        cofins_st_aliq: taxGroups.pisCofinsSt ? cfopRow.cofins_st_aliq : '',
        pis_st_v_bc: taxGroups.pisCofinsSt ? cfopRow.pis_st_v_bc : '',
        cofins_st_v_bc: taxGroups.pisCofinsSt ? cfopRow.cofins_st_v_bc : '',
        // clear ISSQN if not enabled
        issqn_ind_iss: taxGroups.issqn ? cfopRow.issqn_ind_iss : '',
        issqn_c_list_serv: taxGroups.issqn ? cfopRow.issqn_c_list_serv : '',
        issqn_c_mun_fg: taxGroups.issqn ? cfopRow.issqn_c_mun_fg : '',
        issqn_aliq: taxGroups.issqn ? cfopRow.issqn_aliq : '',
        issqn_v_deducao: taxGroups.issqn ? cfopRow.issqn_v_deducao : '',
        issqn_v_iss_ret: taxGroups.issqn ? cfopRow.issqn_v_iss_ret : '',
        uf_overrides: ufOverrideRows,
      }
    })

    setCfopError(null)
    setTaxGroups(EMPTY_TAX_GROUPS)
    setTaxEditorKey((k) => k + 1)
    form.setValue('cfop_config', [...cfopConfig, ...newCfopConfigs])
    setCfopRow(EMPTY_CFOP_ROW)
    setUfOverrideRows([])
  }

  const removeCfop = (i: number) => {
    form.setValue('cfop_config', cfopConfig.filter((_, idx) => idx !== i))
  }

  // ─── Conversion handlers ─────────────────────────────────────────────────

  const addConversion = () => {
    if (!convRow.origin_unit || !convRow.target_unit || !convRow.factor) {
      setConvError('Preencha todos os campos')
      return
    }
    if (!/^[A-Z]{1,6}$/.test(convRow.origin_unit) || !/^[A-Z]{1,6}$/.test(convRow.target_unit)) {
      setConvError('Unidade inválida (apenas A–Z)')
      return
    }
    if (!/^\d+(\.\d+)?$/.test(convRow.factor) || parseFloat(convRow.factor) <= 0) {
      setConvError('Fator deve ser um número positivo')
      return
    }
    setConvError(null)
    form.setValue('conversion_factors', [...conversionFactors, convRow])
    setConvRow(EMPTY_CONVERSION_ROW)
  }

  const removeConversion = (i: number) => {
    form.setValue('conversion_factors', conversionFactors.filter((_, idx) => idx !== i))
  }

  // ─── Submit ───────────────────────────────────────────────────────────────

  const handleSubmit = form.handleSubmit(async (data) => {
    if (data.cfop_config.length === 0 && taxProfileIds.length === 0) {
      setActiveTab('tributacao')
      setSubmitError('Escolha um perfil fiscal ou adicione uma configuração de CFOP na aba Tributação.')
      return
    }
    setSubmitError(null)
    try {
      await onSubmit({
        ...toApiPayload(data),
        tax_profiles: taxProfileIds.map((id) => ({tax_profile_id: id})),
      })
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  // Quantos campos com erro cada aba tem — o badge é o que faz o submit falho
  // apontar para onde o operador precisa ir.
  const errorsByTab = Object.keys(form.formState.errors).reduce<Record<string, number>>((acc, field) => {
    const tab = tabOfField(field)
    acc[tab] = (acc[tab] ?? 0) + 1
    return acc
  }, {})

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-4">
        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}

        {/* ── Tabs ──────────────────────────────────────────────────────── */}
        <div className="flex gap-1 rounded-xl bg-gray-100 p-1 w-fit">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`rounded-lg px-4 py-1.5 text-sm font-medium transition-all ${
                activeTab === tab.id
                  ? 'bg-white text-gray-900 shadow-card'
                  : 'text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
              {tab.id === 'tributacao' && cfopConfig.length > 0 && (
                <span className="ml-1.5 rounded-full bg-brand-100 px-1.5 py-0.5 text-xs font-semibold text-brand-700">
                  {cfopConfig.length}
                </span>
              )}
              {(errorsByTab[tab.id] ?? 0) > 0 && (
                <span aria-label={`${errorsByTab[tab.id]} campo(s) com erro`}
                      className="ml-1.5 rounded-full bg-red-100 px-1.5 py-0.5 text-xs font-semibold text-red-700">
                  {errorsByTab[tab.id]}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* ── Tab: Produto ──────────────────────────────────────────────── */}
        {activeTab === 'produto' && (
          <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Identificação</p>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <FormField control={form.control} name="code" render={({field}) => (
                <FormItem>
                  <FormLabel>Código *</FormLabel>
                  <Input {...field} id={field.name} placeholder="PROD001" maxLength={60}
                         onChange={(e) => field.onChange(e.target.value.toUpperCase())}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="ncm" render={({field}) => (
                <FormItem>
                  <FormLabel>NCM *</FormLabel>
                  <NcmCombobox id={field.name} value={field.value} onValueChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>

            <FormField control={form.control} name="description" render={({field}) => (
              <FormItem>
                <FormLabel>Descrição *</FormLabel>
                <Input {...field} id={field.name} placeholder="Descrição do produto" maxLength={255}/>
                <FormMessage/>
              </FormItem>
            )}/>

            <FormField control={form.control} name="brand" render={({field}) => (
              <FormItem>
                <FormLabel>Marca</FormLabel>
                <Input {...field} id={field.name} placeholder="Ex: Samsung" maxLength={60}/>
                <FormMessage/>
              </FormItem>
            )}/>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
              <FormField control={form.control} name="origin" render={({field}) => (
                <FormItem>
                  <FormLabel>Origem *</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                 options={ORIGIN_OPTIONS}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="cest" render={({field}) => (
                <FormItem>
                  <FormLabel>CEST</FormLabel>
                  <NumericInput {...field} id={field.name} value={field.value ?? ''} placeholder="1234567"
                                maxLength={7} onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>

            {/* Código benefício + composição do total */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
              <FormField control={form.control} name="c_benef" render={({field}) => (
                <FormItem>
                  <FormLabel>Código de benefício fiscal</FormLabel>
                  <Input {...field} id={field.name} value={field.value ?? ''}
                         placeholder="Ex: SP123456 ou SEM CBENEF" maxLength={10}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name="ind_tot" render={({field}) => (
                <FormItem>
                  <FormLabel>Compõe o total da nota?</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                                 options={[
                                   {value: '1', label: 'Sim (padrão)'},
                                   {value: '0', label: 'Não (brinde/amostra)'},
                                 ]}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            </div>

            {/* Override de alíquota ICMS/FCP */}
            <div className="rounded-lg border border-amber-100 bg-amber-50/30 p-3 space-y-2">
              <p className="text-xs font-semibold text-amber-700 uppercase tracking-wider">
                Alíquota específica de ICMS (opcional)
              </p>
              <p className="text-xs text-gray-500">
                {productSystemAliq
                  ? `Vazio ou igual = usa a alíquota do sistema (${productSystemAliq.icms_aliq}%). Preencha apenas se este produto tem tributação diferenciada.`
                  : 'Deixe em branco para usar a alíquota padrão do sistema. Preencha apenas se este produto tem tributação diferenciada.'}
              </p>
              {productAliqDiverges && (
                <div role="alert"
                     className="rounded bg-amber-100 px-2.5 py-1.5 text-xs text-amber-800">
                  Alíquota digitada ({watchedIcmsOverride}%) diverge da tabela do sistema para {uf} ({productSystemAliq?.icms_aliq}%).
                </div>
              )}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="icms_aliq_override" render={({field}) => (
                  <FormItem>
                    <FormLabel>% ICMS específico</FormLabel>
                    <NumericInput {...field} id={field.name} decimal integerPlaces={3} decimalPlaces={4}
                                  value={field.value ?? ''} placeholder="Ex: 12.0000" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="fcp_aliq_override" render={({field}) => (
                  <FormItem>
                    <FormLabel>% FCP específico</FormLabel>
                    <NumericInput {...field} id={field.name} decimal integerPlaces={2} decimalPlaces={4}
                                  value={field.value ?? ''} placeholder="Ex: 2.0000" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            </div>

            {/* EX TIPI — visível quando IPI ativo em algum cfop_config */}
            {cfopConfig.some(c => c.ipi_cst) && (
              <FormField control={form.control} name="ext_ipi" render={({field}) => (
                <FormItem>
                  <FormLabel>Código EX TIPI</FormLabel>
                  <NumericInput {...field} id={field.name} value={field.value ?? ''}
                                placeholder="001" maxLength={3} onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}/>
            )}

            {/* indEscala + CNPJFab — visível quando CEST preenchido */}
            {watchedCest && watchedCest.length === 7 && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
                <FormField control={form.control} name="ind_escala" render={({field}) => (
                  <FormItem>
                    <FormLabel>Indicador de escala</FormLabel>
                    <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                   options={[
                                     {value: 'S', label: 'S – Produção em escala relevante'},
                                     {value: 'N', label: 'N – Não em escala relevante'},
                                   ]}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="cnpj_fab" render={({field}) => (
                  <FormItem>
                    <FormLabel>CNPJ do fabricante</FormLabel>
                    <NumericInput {...field} id={field.name} value={field.value ?? ''}
                                  placeholder="00000000000000" maxLength={14} onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            )}

            {/* Informação adicional do produto */}
            <FormField control={form.control} name="inf_ad_prod" render={({field}) => (
              <FormItem className="pt-1 border-t border-gray-100">
                <FormLabel>Informação adicional do produto</FormLabel>
                <Input {...field} id={field.name} value={field.value ?? ''}
                       placeholder="Informações adicionais para constar na nota" maxLength={500}/>
                <FormMessage/>
              </FormItem>
            )}/>
          </div>
        )}

        {/* ── Tab: Preços e Unidades ────────────────────────────────────── */}
        {activeTab === 'unidades' && (
          <div className="space-y-4">
            {/* Preços */}
            <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Preço e NFC-e</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="value" render={({field}) => (
                  <FormItem>
                    <FormLabel>Preço (Consumidor Final) *</FormLabel>
                    <CurrencyInput id={field.name} name={field.name} decimalPlaces={2} maxDecimalPlaces={4}
                                   value={field.value ?? ''} placeholder="0,00" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="value_resale" render={({field}) => (
                  <FormItem>
                    <FormLabel>Preço (Revenda)</FormLabel>
                    <CurrencyInput id={field.name} name={field.name} decimalPlaces={2} maxDecimalPlaces={4}
                                   value={field.value ?? ''} placeholder="0,00" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="cfop_nfce" render={({field}) => (
                  <FormItem>
                    <FormLabel>CFOP NFC-e *</FormLabel>
                    <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                              options={NFCE_CFOP_OPTIONS} placeholder="Buscar CFOP"
                              searchPlaceholder="Código ou descrição..."/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            </div>

            {/* Unidades */}
            <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Unidades</p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="unit" render={({field}) => (
                  <FormItem>
                    <FormLabel>Un. Comercial *</FormLabel>
                    <Combobox id={field.name} value={field.value} onValueChange={field.onChange}
                              options={UNIT_OPTIONS} placeholder="Unidade" searchPlaceholder="Buscar unidade..."/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="taxable_unit" render={({field}) => (
                  <FormItem>
                    <FormLabel>Un. Tributável</FormLabel>
                    <Combobox id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                              options={UNIT_OPTIONS} placeholder="Igual à comercial"
                              searchPlaceholder="Buscar unidade..."/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>

              {/* GTIN */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
                {/* "SEM GTIN" é literal do leiaute: digitado à mão vira "sem gtin"
                    e o produto é recusado. O checkbox escreve o valor exato. */}
                {([
                  ['cean', 'EAN comercial'],
                  ['taxable_cean', 'EAN tributável'],
                ] as const).map(([name, label]) => (
                  <FormField key={name} control={form.control} name={name} render={({field}) => {
                    const semGtin = field.value === SEM_GTIN
                    return (
                      <FormItem>
                        <FormLabel>{label}</FormLabel>
                        <Input {...field} id={field.name} value={semGtin ? '' : (field.value ?? '')}
                               disabled={semGtin} inputMode="numeric" maxLength={14}
                               placeholder={semGtin ? SEM_GTIN : 'EAN-13'}
                               onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                        <label className="flex min-h-11 items-center gap-2 text-sm text-gray-600 sm:min-h-0">
                          <input type="checkbox" className="size-4" checked={semGtin}
                                 onChange={(e) => field.onChange(e.target.checked ? SEM_GTIN : '')}/>
                          Produto sem código de barras
                        </label>
                        <FormMessage/>
                      </FormItem>
                    )
                  }}/>
                ))}
              </div>

              {/* Pesos */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1 border-t border-gray-100">
                <FormField control={form.control} name="net_weight" render={({field}) => (
                  <FormItem>
                    <FormLabel>Peso líquido</FormLabel>
                    <NumericInput {...field} id={field.name} decimal integerPlaces={5} decimalPlaces={3}
                                  suffix="KG" value={field.value ?? ''} placeholder="10.500" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="gross_weight" render={({field}) => (
                  <FormItem>
                    <FormLabel>Peso bruto</FormLabel>
                    <NumericInput {...field} id={field.name} decimal integerPlaces={5} decimalPlaces={3}
                                  suffix="KG" value={field.value ?? ''} placeholder="11.000" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            </div>

            {/* Fatores de conversão */}
            {showConversionFactors && (
              <div className="rounded-xl border border-amber-200 bg-amber-50/40 p-5 space-y-3">
                <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                  Fatores de conversão ({watchedUnit} ↔ {watchedTaxableUnit})
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 max-w-lg">
                  <div className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">Un. Origem</label>
                    <Combobox value={convRow.origin_unit}
                              onValueChange={(v) => setConvRow((r) => ({...r, origin_unit: v}))}
                              options={UNIT_OPTIONS} placeholder={watchedUnit} searchPlaceholder="Buscar unidade..."/>
                  </div>
                  <div className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">Un. Destino</label>
                    <Combobox value={convRow.target_unit}
                              onValueChange={(v) => setConvRow((r) => ({...r, target_unit: v}))}
                              options={UNIT_OPTIONS} placeholder={watchedTaxableUnit}
                              searchPlaceholder="Buscar unidade..."/>
                  </div>
                  <div className="grid gap-1">
                    <label className="text-sm font-medium text-gray-700">Fator</label>
                    <NumericInput value={convRow.factor} decimal integerPlaces={7} decimalPlaces={6}
                                  placeholder="20" onChange={(v) => setConvRow((r) => ({...r, factor: v}))}/>
                  </div>
                </div>
                {convError && <p className="text-[0.8rem] font-medium text-destructive">{convError}</p>}
                <Button type="button" variant="ghost" size="sm" onClick={addConversion}
                        className="text-brand-600 hover:text-brand-700 px-0">
                  + Adicionar fator
                </Button>
                {conversionFactors.length > 0 && (
                  <div className="space-y-1">
                    {conversionFactors.map((f, i) => (
                      <div key={i}
                           className="flex items-center justify-between rounded bg-white border border-amber-200 px-3 py-2 text-sm">
                        <span className="font-mono text-gray-700">1 {f.origin_unit} = {f.factor} {f.target_unit}</span>
                        <Button type="button" variant="ghost" size="xs" onClick={() => removeConversion(i)}
                                className="ml-4 text-danger hover:text-red-700">remover
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* ── Tab: Tributação ───────────────────────────────────────────── */}
        {activeTab === 'tributacao' && (
          <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-5">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                Configuração CFOP / Tributação
              </p>
              <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${
                simples ? 'bg-emerald-50 text-emerald-700' : 'bg-blue-50 text-blue-700'
              }`}>
                {simples ? 'Simples Nacional — CSOSN' : 'Regime Normal — ICMS CST'}
              </span>
            </div>

            {/* ── Perfis fiscais ───────────────────────────────────────── */}
            <div className="space-y-2 rounded-lg border border-gray-100 bg-gray-50/50 p-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Perfis fiscais</p>
              <p className="text-xs text-gray-500">
                Escolher um perfil dispensa preencher a tributação aqui. Para sobrescrever um CFOP só
                neste produto, adicione a linha abaixo — ele tem prioridade sobre o perfil fiscal.
              </p>
              {taxProfiles.length === 0 ? (
                <p className="text-xs text-gray-500">
                  Nenhum perfil cadastrado ainda.{' '}
                  <Link href="/tax-profiles/new" className="font-medium text-brand-600 hover:text-brand-700">
                    Criar o primeiro
                  </Link>
                  .
                </p>
              ) : (
                <div className="flex flex-col gap-2 pt-1">
                  {taxProfiles.map((profile) => {
                    const id = extractId(profile.sk, SK_PREFIX.TAX_PROFILE)
                    const checked = taxProfileIds.includes(id)
                    return (
                      <label key={profile.sk}
                             className="flex min-h-11 sm:min-h-0 cursor-pointer items-start gap-2 text-sm text-gray-700">
                        <input type="checkbox" checked={checked}
                               onChange={() => setTaxProfileIds((prev) => checked
                                 ? prev.filter((p) => p !== id)
                                 : [...prev, id])}
                               className="mt-0.5 size-4 cursor-pointer rounded border-gray-300 text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"/>
                        <span>
                          <span className="font-medium text-gray-900">{profile.name}</span>
                          <span className="ml-2 font-mono text-xs text-gray-500">
                            {(profile.cfops ?? []).join(' · ')}
                          </span>
                        </span>
                      </label>
                    )
                  })}
                </div>
              )}
            </div>

            {/* Editor de tributação — o mesmo componente do perfil fiscal. */}
            <TaxFieldsEditor key={taxEditorKey} value={cfopRow} onChange={setCfopRow} simples={simples}
                             groups={taxGroups} onGroupsChange={setTaxGroups}
                             emitUf={uf} destUf={uf} ncm={watchedNcm}/>

            {/* Overrides por UF de destino — só preenche o que diverge */}
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                Overrides por UF de destino (opcional)
              </p>
              <UfOverridesEditor key={taxEditorKey} value={ufOverrideRows} onChange={setUfOverrideRows}
                                 simples={simples}/>
            </div>

            {/* ── Erros + botão ────────────────────────────────────────── */}
            {cfopError && <p className="text-[0.8rem] font-medium text-destructive">{cfopError}</p>}
            {form.formState.errors.cfop_config && (
              <p className="text-[0.8rem] font-medium text-destructive">
                {form.formState.errors.cfop_config.message ?? form.formState.errors.cfop_config.root?.message}
              </p>
            )}

            <Button type="button" variant="ghost" size="sm" onClick={addCfop}
                    className="text-brand-600 hover:text-brand-700 px-0">
              + Adicionar CFOP
            </Button>

            {/* ── Lista de CFOPs configurados ──────────────────────────── */}
            {cfopConfig.length > 0 && (
              <div className="space-y-1">
                {cfopConfig.map((c, i) => {
                  const icmsPart = simples
                    ? `CSOSN ${c.csosn}${c.icms_sn_cred_aliq ? ` cred.${c.icms_sn_cred_aliq}%` : ''}`
                    : `ICMS ${c.icms}${c.icms_aliq_override ? ` ${c.icms_aliq_override}%` : ''}${c.icms_p_red_bc ? ` red.${c.icms_p_red_bc}%` : ''}${c.icms_mot_des ? ` mot.${c.icms_mot_des}` : ''}`
                  const stPart = c.icms_st_aliq ? ` · ST ${c.icms_st_aliq}%` : ''
                  const ipiPart = c.ipi_cst ? ` · IPI ${c.ipi_cst}${c.ipi_aliq ? `/${c.ipi_aliq}%` : ''}` : ''
                  const isPart = c.is_cst ? ` · IS ${c.is_cst}` : ''
                  return (
                    <div key={i}
                         className="flex items-center justify-between rounded bg-gray-50 px-3 py-2 text-sm">
                      <span className="font-mono text-gray-700">
                        CFOP {c.cfop} · {icmsPart}{stPart} · PIS {c.pis} · COF {c.cofins}{ipiPart}{isPart} · IBS/CBS {c.ibs_cbs_cst}
                      </span>
                      <Button type="button" variant="ghost" size="xs" onClick={() => removeCfop(i)}
                              className="ml-4 text-danger hover:text-red-700">remover
                      </Button>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )}

        {/* ── Tab: Tipo Especial ──────────────────────────────────────── */}
        {activeTab === 'especial' && (
          <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-5">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Tipo específico</p>
              <p className="text-xs text-gray-400">Opcional — preencher apenas para combustíveis ou medicamentos</p>
            </div>

            <FormField control={form.control} name="prod_type" render={({field}) => (
              <FormItem>
                <FormLabel>Tipo do produto</FormLabel>
                <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                               options={[
                                 {value: '', label: 'Genérico (padrão)'},
                                 {value: 'comb', label: 'Combustível (comb/ANP)'},
                                 {value: 'med', label: 'Medicamento (ANVISA)'},
                                 {value: 'veiculo', label: 'Veículo novo (RENAVAM)'},
                                 {value: 'arma', label: 'Armamento'},
                               ]}/>
                <FormMessage/>
              </FormItem>
            )}/>

            {/* ── Combustível ────────────────────────────────────────── */}
            {watchedProdType === 'comb' && (
              <div className="rounded-lg border border-orange-100 bg-orange-50/30 p-4 space-y-3">
                <p className="text-xs font-semibold text-orange-700 uppercase tracking-wider">
                  Combustível — Dados ANP
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormField control={form.control} name="comb_c_prod_anp" render={({field}) => (
                    <FormItem>
                      <FormLabel>Código ANP (9 dígitos) *</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={9}
                                    placeholder="Ex: 210203001" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="comb_desc_anp" render={({field}) => (
                    <FormItem>
                      <FormLabel>Descrição ANP *</FormLabel>
                      <Input {...field} id={field.name} value={field.value ?? ''}
                             placeholder="Ex: GASOLINA COMUM" maxLength={95}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="comb_uf_cons" render={({field}) => (
                    <FormItem>
                      <FormLabel>UF de consumo *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''}
                                     onValueChange={field.onChange} options={UF_OPTIONS}
                                     placeholder="UF"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="comb_codif" render={({field}) => (
                    <FormItem>
                      <FormLabel>CODIF (AEAC)</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={21}
                                    placeholder="Apenas UFs com CODIF" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="comb_p_bio" render={({field}) => (
                    <FormItem>
                      <FormLabel>% Biodiesel B100 (pBio)</FormLabel>
                      <NumericInput {...field} id={field.name} decimal integerPlaces={3} decimalPlaces={4}
                                    value={field.value ?? ''} placeholder="Ex: 10.0000" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="comb_cide_v_aliq_prod" render={({field}) => (
                    <FormItem>
                      <FormLabel>Alíquota da CIDE (R$ por unidade)</FormLabel>
                      <NumericInput {...field} id={field.name} decimal integerPlaces={9} decimalPlaces={4}
                                    value={field.value ?? ''} placeholder="0.0000" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                </div>
                <CombOrigFields
                  value={watchedCombOrig ?? []}
                  onChange={(v) => form.setValue('comb_orig', v, {shouldDirty: true})}/>
                <p className="text-xs text-gray-400">
                  Para GLP (ANP 210203001): preencha também pGLP, pGNn, pGNi e vPart nas configurações avançadas.
                  A base e o valor da CIDE saem da quantidade vendida — só a alíquota é cadastrada.
                </p>
              </div>
            )}

            {/* ── Medicamento ─────────────────────────────────────────── */}
            {watchedProdType === 'med' && (
              <div className="rounded-lg border border-teal-100 bg-teal-50/30 p-4 space-y-3">
                <p className="text-xs font-semibold text-teal-700 uppercase tracking-wider">
                  Medicamento — Dados ANVISA
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormField control={form.control} name="med_c_prod_anvisa" render={({field}) => {
                    const isento = field.value === ANVISA_ISENTO
                    return (
                      <FormItem>
                        <FormLabel>Registro ANVISA *</FormLabel>
                        <Input {...field} id={field.name} value={isento ? '' : (field.value ?? '')}
                               disabled={isento} inputMode="numeric" maxLength={13}
                               placeholder={isento ? ANVISA_ISENTO : '11 ou 13 dígitos'}
                               onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                        <label className="flex min-h-11 items-center gap-2 text-sm text-gray-600 sm:min-h-0">
                          <input type="checkbox" className="size-4" checked={isento}
                                 onChange={(e) => field.onChange(e.target.checked ? ANVISA_ISENTO : '')}/>
                          Medicamento isento de registro
                        </label>
                        <FormMessage/>
                      </FormItem>
                    )
                  }}/>
                  <FormField control={form.control} name="med_v_pmc" render={({field}) => (
                    <FormItem>
                      <FormLabel>Preço Máximo ao Consumidor (R$) *</FormLabel>
                      <NumericInput {...field} id={field.name} decimal integerPlaces={9} decimalPlaces={2}
                                    prefix="R$" value={field.value ?? ''} placeholder="0.00" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="med_x_motivo_isencao" render={({field}) => (
                    <FormItem className="sm:col-span-2">
                      <FormLabel>Motivo da isenção ANVISA</FormLabel>
                      <Input {...field} id={field.name} value={field.value ?? ''}
                             placeholder="Obrigatório quando Registro = ISENTO (ex: RDC 12/2023)" maxLength={255}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                </div>
              </div>
            )}

            {/* ── Importação e códigos próprios ───────────────────────── */}
            <div className="rounded-lg border border-gray-100 p-4 space-y-3">
              <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                Importação e códigos próprios
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="nve" render={({field}) => (
                  <FormItem>
                    <FormLabel>NVE (até 8, separados por vírgula)</FormLabel>
                    <Input id={field.name} value={(field.value ?? []).join(', ')}
                           placeholder="AA0001, BB0002"
                           onChange={(e) => field.onChange(
                             e.target.value.split(',').map((v) => v.trim().toUpperCase()).filter(Boolean),
                           )}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="n_fci" render={({field}) => (
                  <FormItem>
                    <FormLabel>Número da FCI</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={36}
                           placeholder="UUID da Ficha de Conteúdo de Importação"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="n_recopi" render={({field}) => (
                  <FormItem>
                    <FormLabel>RECOPI (papel imune)</FormLabel>
                    <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={20}
                                  placeholder="20 dígitos" onChange={field.onChange}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="c_barra" render={({field}) => (
                  <FormItem>
                    <FormLabel>Código de barras próprio</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={30}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="c_barra_trib" render={({field}) => (
                  <FormItem>
                    <FormLabel>Código de barras da unidade tributável</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={30}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            </div>

            {/* ── Reforma tributária — nível produto ──────────────────── */}
            <div className="rounded-lg border border-gray-100 p-4 space-y-3">
              <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                Reforma tributária (IBS/CBS) — produto
              </p>
              <p className="text-xs text-gray-500">
                Crédito presumido da UF, subapuração na ZFM e bem móvel usado. As alíquotas e os CSTs
                do IBS/CBS ficam na aba Tributação, por CFOP.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="tp_cred_pres_ibs_zfm" render={({field}) => (
                  <FormItem>
                    <FormLabel>Subapuração do IBS na ZFM</FormLabel>
                    <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                   options={TP_CRED_PRES_IBS_ZFM_OPTIONS} placeholder="Não se aplica"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="ind_bem_movel_usado" render={({field}) => (
                  <FormItem>
                    <label htmlFor={field.name}
                           className="flex items-center gap-2 min-h-11 text-sm text-gray-700 cursor-pointer">
                      <input type="checkbox" id={field.name}
                             checked={field.value === IND_BEM_MOVEL_USADO_SIM}
                             onChange={(e) => field.onChange(e.target.checked ? IND_BEM_MOVEL_USADO_SIM : '')}
                             className="h-4 w-4 rounded border-gray-300 text-brand-600"/>
                      Fornecimento de bem móvel usado
                    </label>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
              <GCredEditor form={form}/>
            </div>

            {/* ── IPI — selo de controle e enquadramento ──────────────── */}
            <div className="rounded-lg border border-gray-100 p-4 space-y-3">
              <p className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                IPI — selo de controle
              </p>
              <p className="text-xs text-gray-500">
                Só para produto com selo (bebidas, cigarros). O enquadramento legal (cEnq) sai como
                <span className="font-medium"> 999</span> quando não informado.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="ipi_cnpj_prod" render={({field}) => (
                  <FormItem>
                    <FormLabel>CNPJ do produtor</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={18}
                           placeholder="Somente números"
                           onChange={(e) => field.onChange(e.target.value.replace(/\D/g, '').slice(0, 14))}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="ipi_c_enq" render={({field}) => (
                  <FormItem>
                    <FormLabel>Enquadramento legal (cEnq)</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={3}
                           inputMode="numeric" placeholder="999"
                           onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="ipi_c_selo" render={({field}) => (
                  <FormItem>
                    <FormLabel>Código do selo</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={60}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="ipi_q_selo" render={({field}) => (
                  <FormItem>
                    <FormLabel>Quantidade de selos</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={12}
                           inputMode="numeric"
                           onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            </div>

            {/* ── Produto perigoso (MDF-e peri) ───────────────────────── */}
            <div className="rounded-lg border border-amber-100 bg-amber-50/30 p-4 space-y-3">
              <p className="text-xs font-semibold text-amber-700 uppercase tracking-wider">
                Produto perigoso (MDF-e)
              </p>
              <p className="text-xs text-gray-500">
                Preencha só se o produto for classificado como perigoso. O MDF-e monta o grupo
                <span className="font-medium"> peri</span> sozinho a partir daqui — nunca é perguntado por viagem.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control} name="peri_n_onu" render={({field}) => (
                  <FormItem>
                    <FormLabel>Número ONU</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={4}
                           inputMode="numeric" placeholder="Ex: 1203"
                           onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="peri_x_nome_ae" render={({field}) => (
                  <FormItem>
                    <FormLabel>Nome apropriado para embarque</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={150}
                           placeholder="Ex: GASOLINA"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="peri_x_cla_risco" render={({field}) => (
                  <FormItem>
                    <FormLabel>Classe de risco</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={40}
                           placeholder="Ex: 3"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="peri_gr_emb" render={({field}) => (
                  <FormItem>
                    <FormLabel>Grupo de embalagem</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={6}
                           placeholder="Ex: II"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
                <FormField control={form.control} name="peri_q_vol_tipo" render={({field}) => (
                  <FormItem className="sm:col-span-2">
                    <FormLabel>Tipo de volume transportado</FormLabel>
                    <Input {...field} id={field.name} value={field.value ?? ''} maxLength={60}
                           placeholder="Ex: TAMBOR"/>
                    <FormMessage/>
                  </FormItem>
                )}/>
              </div>
            </div>

            {/* ── Veículo novo (veicProd) ─────────────────────────────── */}
            {watchedProdType === 'veiculo' && (
              <div className="rounded-lg border border-indigo-100 bg-indigo-50/30 p-4 space-y-4">
                <p className="text-xs font-semibold text-indigo-700 uppercase tracking-wider">
                  Veículo Novo — Dados do Modelo (RENAVAM)
                </p>
                <p className="text-xs text-gray-400">
                  Campos por unidade (chassi, nSerie, nMotor) são preenchidos na emissão da NF-e.
                </p>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  <FormField control={form.control} name="veic_tp_op" render={({field}) => (
                    <FormItem>
                      <FormLabel>Tipo de operação *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_TP_OP_OPTIONS} placeholder="Tipo"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_tp_comb" render={({field}) => (
                    <FormItem>
                      <FormLabel>Combustível *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_TP_COMB_OPTIONS} placeholder="Combustível"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_cond_veic" render={({field}) => (
                    <FormItem>
                      <FormLabel>Condição *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_COND_OPTIONS} placeholder="Condição"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_ano_mod" render={({field}) => (
                    <FormItem>
                      <FormLabel>Ano modelo *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEHICLE_YEAR_OPTIONS} placeholder="Ano"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_ano_fab" render={({field}) => (
                    <FormItem>
                      <FormLabel>Ano fabricação *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEHICLE_YEAR_OPTIONS} placeholder="Ano"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_c_mod" render={({field}) => (
                    <FormItem>
                      <FormLabel>Código Marca/Modelo *</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={6}
                                    placeholder="RENAVAM" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_pot" render={({field}) => (
                    <FormItem>
                      <FormLabel>Potência (CV) *</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={4}
                                    placeholder="130" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_cilin" render={({field}) => (
                    <FormItem>
                      <FormLabel>Cilindradas (CC) *</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={4}
                                    placeholder="1599" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_tp_veic" render={({field}) => (
                    <FormItem>
                      <FormLabel>Tipo veículo RENAVAM *</FormLabel>
                      <Combobox id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                options={VEIC_TP_VEIC_OPTIONS} placeholder="Tipo RENAVAM"
                                searchPlaceholder="Código ou descrição..."/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_esp_veic" render={({field}) => (
                    <FormItem>
                      <FormLabel>Espécie RENAVAM *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_ESP_VEIC_OPTIONS} placeholder="Espécie"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_tp_rest" render={({field}) => (
                    <FormItem>
                      <FormLabel>Restrição *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_TP_REST_OPTIONS} placeholder="Restrição"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_vin" render={({field}) => (
                    <FormItem>
                      <FormLabel>VIN remarcado *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_VIN_OPTIONS} placeholder="VIN"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_c_cor_denatran" render={({field}) => (
                    <FormItem>
                      <FormLabel>Cor DENATRAN *</FormLabel>
                      <Combobox id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                options={VEIC_COR_DENATRAN_OPTIONS} placeholder="Cor"
                                searchPlaceholder="Cor..."/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_c_cor" render={({field}) => (
                    <FormItem>
                      <FormLabel>Código cor (montadora)</FormLabel>
                      <Input {...field} id={field.name} value={field.value ?? ''} placeholder="Ex: BRAN" maxLength={4}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_x_cor" render={({field}) => (
                    <FormItem>
                      <FormLabel>Descrição da cor</FormLabel>
                      <Input {...field} id={field.name} value={field.value ?? ''} placeholder="Ex: Branco Polar"
                             maxLength={40}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_lota" render={({field}) => (
                    <FormItem>
                      <FormLabel>Lotação (passageiros) *</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={3}
                                    placeholder="5" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_tp_pint" render={({field}) => (
                    <FormItem>
                      <FormLabel>Tipo pintura *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={VEIC_TP_PINT_OPTIONS} placeholder="Pintura"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_cmt" render={({field}) => (
                    <FormItem>
                      <FormLabel>CMT (ton)</FormLabel>
                      <Input {...field} id={field.name} value={field.value ?? ''} placeholder="0.000" maxLength={9}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="veic_dist" render={({field}) => (
                    <FormItem>
                      <FormLabel>Distância eixos (mm)</FormLabel>
                      <NumericInput {...field} id={field.name} value={field.value ?? ''} maxLength={4}
                                    placeholder="2550" onChange={field.onChange}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                </div>
              </div>
            )}

            {/* ── Armamento ───────────────────────────────────────────── */}
            {watchedProdType === 'arma' && (
              <div className="rounded-lg border border-red-100 bg-red-50/20 p-4 space-y-3">
                <p className="text-xs font-semibold text-red-700 uppercase tracking-wider">
                  Armamento — Dados do Tipo
                </p>
                <p className="text-xs text-gray-400">
                  Números de série (nSerie, nCano) são preenchidos por unidade na emissão da NF-e.
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormField control={form.control} name="arma_tp_arma" render={({field}) => (
                    <FormItem>
                      <FormLabel>Tipo de arma *</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                                     options={[
                                       {value: '0', label: '0 – Uso permitido'},
                                       {value: '1', label: '1 – Uso restrito'},
                                     ]} placeholder="Tipo"/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                  <FormField control={form.control} name="arma_descr" render={({field}) => (
                    <FormItem>
                      <FormLabel>Descrição padrão *</FormLabel>
                      <Input {...field} id={field.name} value={field.value ?? ''}
                             placeholder="Ex: Pistola .380, marca XYZ, capacidade 15 balas" maxLength={256}/>
                      <FormMessage/>
                    </FormItem>
                  )}/>
                </div>
              </div>
            )}
          </div>
        )}

        {/* ── Submit ────────────────────────────────────────────────────── */}
        <div className="flex justify-end">
          <Button type="submit" disabled={loading} className="min-w-36">
            {loading ? 'Salvando...' : initialData ? 'Salvar alterações' : 'Criar produto'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
/**
 * Origem do combustível (comb/origComb): de onde veio e em que proporção. É do
 * produto porque a mistura não muda por venda — muda quando o posto troca de
 * fornecedor, e aí o cadastro é atualizado uma vez.
 */
function CombOrigFields({value, onChange}: {
  value: NonNullable<ProductFormData['comb_orig']>
  onChange: (v: NonNullable<ProductFormData['comb_orig']>) => void
}) {
  const patch = (i: number, p: Partial<CombOrigIn>) =>
    onChange(value.map((o, k) => (k === i ? {...o, ...p} : o)))

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium text-gray-600">Origem do combustível (até 30)</p>
        <Button type="button" variant="ghost" size="xs" disabled={value.length >= 30}
                onClick={() => onChange([...value, {ind_import: '0', c_uf_orig: '', p_orig: ''}])}>
          + Origem
        </Button>
      </div>
      {value.map((o, i) => (
        <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
          <div className="flex flex-col gap-1">
            <Label htmlFor={`orig-imp-${i}`} className="text-xs font-medium text-gray-600">Procedência</Label>
            <OptionsSelect id={`orig-imp-${i}`} value={o.ind_import}
                           onValueChange={(v: string) => patch(i, {ind_import: v as CombOrigIn['ind_import']})}
                           options={[
                             {value: '0', label: '0 – Nacional'},
                             {value: '1', label: '1 – Importado'},
                           ]}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`orig-uf-${i}`} className="text-xs font-medium text-gray-600">UF de origem</Label>
            <OptionsSelect id={`orig-uf-${i}`} value={o.c_uf_orig} options={UF_IBGE_OPTIONS}
                           onValueChange={(v: string) => patch(i, {c_uf_orig: v})} placeholder="UF"/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`orig-p-${i}`} className="text-xs font-medium text-gray-600">% da mistura</Label>
            <NumericInput id={`orig-p-${i}`} value={o.p_orig} decimal integerPlaces={3} decimalPlaces={4}
                          className="w-full" placeholder="70.0000" onChange={(v) => patch(i, {p_orig: v})}/>
          </div>
          <Button type="button" variant="ghost" size="xs"
                  onClick={() => onChange(value.filter((_, k) => k !== i))}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}
