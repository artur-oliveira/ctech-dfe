import {z} from 'zod'
import {packingGroupApplies, RISK_CLASSES} from '@/lib/data/dangerous_goods'
import {IPI_CENQ} from '@/lib/data/ipi_cenq'

export const conversionFactorSchema = z.object({
  origin_unit: z
    .string()
    .min(1, 'Unidade obrigatória')
    .max(6)
    .regex(/^[A-Z]{1,6}$/, 'Apenas A–Z'),
  target_unit: z
    .string()
    .min(1, 'Unidade obrigatória')
    .max(6)
    .regex(/^[A-Z]{1,6}$/, 'Apenas A–Z'),
  factor: z
    .string()
    .regex(/^\d+(\.\d+)?$/, 'Fator inválido (ex: 20 ou 0.5)')
    .refine((v) => parseFloat(v) > 0, 'Deve ser maior que zero'),
})

const _ibsCbsCstRegex = /^(000|010|011|200|220|221|222|400|410|510|515|550|620|800|810|811|820|830)$/
const _ibsCbsClassRegex = /^\d{6}$/
const _ibsCbsAliqRegex = /^\d{1,3}(\.\d{1,4})?$/
const _percentRegex = /^\d{1,3}(\.\d{1,4})?$/

/** CSTs de IPI tributado — exigem pIPI ou vUnid (leiauteNFe_v4.00, grupo IPITrib). */
const IPI_TAXED_CSTS = new Set(['00', '49', '50', '99'])

/** modBC por pauta (1) e por lista negociada (2) exigem o valor da pauta. */
const ICMS_MOD_BC_PAUTA = new Set(['1', '2'])

const optionalStr = z.string().optional().or(z.literal(''))
const optionalPercent = z.string().regex(_percentRegex, '% inválido').optional().or(z.literal(''))

export const ufTaxOverrideSchema = z.object({
  ufs: z.array(z.string().regex(/^[A-Z]{2}$/, 'UF inválida')).min(1, 'Escolha ao menos uma UF'),
  overrides: z.record(z.string(), z.unknown()),
})

export const cfopConfigBase = z.object({
  cfop: z.string().regex(/^\d{4}$/, 'CFOP deve ter 4 dígitos'),
  // csosn/icms são validados manualmente em addCfop de acordo com o CRT da org
  csosn: optionalStr,
  icms: optionalStr,
  // ICMS alíquotas e modalidade
  icms_mod_bc: optionalStr,
  icms_aliq_override: optionalPercent,
  icms_fcp_override: optionalPercent,
  icms_sn_cred_aliq: optionalPercent,
  icms_ind_deduz_deson: optionalStr,
  // ICMS ST
  icms_st_mod_bc: optionalStr,
  icms_st_mva: optionalPercent,
  icms_st_red_bc: optionalPercent,
  icms_st_aliq: optionalPercent,
  icms_st_fcp_aliq: optionalPercent,
  // Campos condicionais ICMS (Regime Normal)
  icms_p_red_bc: optionalPercent,
  icms_mot_des: optionalStr,
  icms_p_dif: optionalPercent,
  icms_pauta_valor: optionalStr,
  // ICMS monofásico combustíveis (CST 02/15/53/61)
  icms_ad_rem: optionalPercent,
  icms_ad_rem_reten: optionalPercent,
  icms_p_red_ad_rem: optionalPercent,
  icms_mot_red_ad_rem: optionalStr,
  icms_p_dif_mono: optionalPercent,
  // ICMS60 — ST retida anteriormente (opcional)
  icms_v_bc_st_ret: optionalStr,
  icms_v_icms_st_ret: optionalStr,
  icms_p_st: optionalPercent,
  icms_fcp_v_bc_st_ret: optionalStr,
  icms_fcp_st_ret_aliq: optionalPercent,
  // ICMSST (CST 41) — repasse da ST já retida na operação interestadual
  icms_v_bc_st_dest: optionalStr,
  icms_v_icms_st_dest: optionalStr,
  // ICMS efetivo — ICMS60, ICMSST e ICMSSN500
  icms_p_red_bc_efet: optionalPercent,
  icms_p_icms_efet: optionalPercent,
  // ICMSPart — partilha entre UF de origem e destino (não tem CST próprio)
  icms_part_p_bc_op: optionalPercent,
  icms_part_uf_st: optionalStr,
  // ST desonerada (ICMS10/70/90) e FCP diferido (ICMS51/90)
  icms_mot_des_st: optionalStr,
  icms_p_fcp_dif: optionalPercent,
  // IPI por unidade — vUnid presente troca vBC+pIPI por qUnid+vUnid
  ipi_v_unid: optionalStr,
  // Observação fiscal do item (det/obsItem)
  obs_item_x_campo: optionalStr,
  obs_item_x_texto: optionalStr,
  // PIS/COFINS
  pis: z.string().regex(/^\d{2}$/, 'PIS deve ter 2 dígitos'),
  cofins: z.string().regex(/^\d{2}$/, 'COFINS deve ter 2 dígitos'),
  pis_aliq: optionalPercent,
  cofins_aliq: optionalPercent,
  pis_aliq_unid: optionalPercent,
  cofins_aliq_unid: optionalPercent,
  // PIS/COFINS-ST — substituição tributária (grupo opcional)
  pis_st_aliq: optionalPercent,
  cofins_st_aliq: optionalPercent,
  pis_st_v_bc: optionalStr,
  cofins_st_v_bc: optionalStr,
  // IPI
  ipi_cst: optionalStr,
  ipi_aliq: optionalPercent,
  // IS — Imposto Seletivo (NT 2024.001)
  is_cst: optionalStr,
  is_aliq: optionalPercent,
  is_class_trib: z.string().regex(/^\d{6}$/, 'Classificação deve ter 6 dígitos').optional().or(z.literal('')),
  is_aliq_espec: optionalPercent,
  is_unid_trib: optionalStr,
  // IBS/CBS — opcional, tudo-ou-nada (o backend valida a regra; aqui cada
  // campo só precisa ter formato válido quando preenchido)
  ibs_cbs_cst: z.string().regex(_ibsCbsCstRegex, 'CST IBS/CBS inválido').optional().or(z.literal('')),
  ibs_cbs_class_trib: z.string().regex(_ibsCbsClassRegex, 'Código de classificação deve ter 6 dígitos').optional().or(z.literal('')),
  ibs_uf_aliq: z.string().regex(_ibsCbsAliqRegex, 'Alíquota IBS Estadual inválida (ex: 8.0000)').optional().or(z.literal('')),
  ibs_mun_aliq: z.string().regex(_ibsCbsAliqRegex, 'Alíquota IBS Municipal inválida (ex: 1.0000)').optional().or(z.literal('')),
  cbs_aliq: z.string().regex(_ibsCbsAliqRegex, 'Alíquota CBS inválida (ex: 9.0000)').optional().or(z.literal('')),
  // IBS/CBS redução e diferimento (Reforma Tributária)
  ibs_uf_p_red: optionalPercent,
  ibs_mun_p_red: optionalPercent,
  cbs_p_red: optionalPercent,
  ibs_uf_p_dif: optionalPercent,
  ibs_mun_p_dif: optionalPercent,
  cbs_p_dif: optionalPercent,
  // indDoacao (TIndDoacao) enumera um valor só: 1. Na UI é um checkbox.
  ibs_ind_doacao: z.enum(['1']).optional().or(z.literal('')),
  // Monofasia (gIBSCBSMono): alíquota específica por unidade em cada sub-grupo.
  ibs_ad_rem: optionalPercent,
  cbs_ad_rem: optionalPercent,
  ibs_ad_rem_reten: optionalPercent,
  cbs_ad_rem_reten: optionalPercent,
  ibs_ad_rem_ret: optionalPercent,
  cbs_ad_rem_ret: optionalPercent,
  ibs_p_dif_mono: optionalPercent,
  cbs_p_dif_mono: optionalPercent,
  // Devolução de tributo ao adquirente: um percentual só, nas três esferas.
  ibs_cbs_p_dev_trib: optionalPercent,
  // Tributação de referência (gTribRegular) e de compra governamental
  // (gTribCompraGov) — quanto o item pagaria fora do benefício.
  ibs_reg_cst: z.string().regex(_ibsCbsCstRegex, 'CST IBS/CBS inválido').optional().or(z.literal('')),
  ibs_reg_class_trib: z.string().regex(_ibsCbsClassRegex, 'Código de classificação deve ter 6 dígitos').optional().or(z.literal('')),
  ibs_reg_uf_aliq: optionalPercent,
  ibs_reg_mun_aliq: optionalPercent,
  cbs_reg_aliq: optionalPercent,
  ibs_gov_uf_aliq: optionalPercent,
  ibs_gov_mun_aliq: optionalPercent,
  cbs_gov_aliq: optionalPercent,
  // Crédito presumido da operação e o da ZFM (choice: o da operação vence).
  ibs_cbs_c_cred_pres: z.string().regex(/^\d{2}$/, 'Código do crédito presumido tem 2 dígitos').optional().or(z.literal('')),
  ibs_p_cred_pres: optionalPercent,
  cbs_p_cred_pres: optionalPercent,
  ibs_cbs_cred_pres_cond_sus: z.enum(['1']).optional().or(z.literal('')),
  ibs_zfm_p_cred_pres: optionalPercent,
  // Alíquota zero da CBS em ALC/ZFM (gALCZFMCBS).
  alc_zfm_tp_cbs: z.enum(['1', '2']).optional().or(z.literal('')),
  alc_zfm_n_proc_suframa: z.string().regex(/^.{8,12}$/, 'Processo Suframa tem 8 a 12 caracteres').optional().or(z.literal('')),
  // ISSQN — Imposto Sobre Serviços (LC 116/2003)
  issqn_ind_iss: optionalStr,
  issqn_c_list_serv: optionalStr,
  issqn_c_mun_fg: z.string().regex(/^\d{7}$/, 'Código deve ter 7 dígitos').optional().or(z.literal('')),
  issqn_aliq: optionalPercent,
  issqn_v_deducao: optionalStr,
  issqn_v_outro: optionalStr,
  issqn_v_desc_incond: optionalStr,
  issqn_v_desc_cond: optionalStr,
  issqn_c_servico: optionalStr,
  issqn_c_mun: optionalStr,
  issqn_c_pais: z.string().regex(/^\d{4}$/, 'Código de país tem 4 dígitos').optional().or(z.literal('')),
  issqn_n_processo: optionalStr,
  issqn_ind_incentivo: optionalStr,
  issqn_v_iss_ret: optionalStr,
  // Overrides por UF de destino — só preenche o que diverge para aquelas UFs
  uf_overrides: z.array(ufTaxOverrideSchema).optional(),
})

/**
 * Regras de grupo do tratamento tributário. Ficam numa função para valerem tanto
 * na linha de CFOP do produto quanto no perfil fiscal, que é a mesma tabela sem
 * o CFOP — e porque um schema com refinement não aceita `.omit()`.
 */
export function applyTaxGroupRules(
  data: Record<string, unknown>,
  ctx: z.RefinementCtx,
): void {
  const filled = (v: unknown): boolean => typeof v === 'string' && v.trim() !== ''
  /** Grupo do leiaute: ou vêm todos os campos, ou nenhum. */
  const requireTogether = (fields: string[], message: string) => {
    const present = fields.filter((f) => filled(data[f]))
    if (present.length === 0 || present.length === fields.length) return
    for (const field of fields) {
      if (!filled(data[field])) {
        ctx.addIssue({code: 'custom', path: [field], message})
      }
    }
  }

  // IPI tributado é por alíquota OU por unidade — nunca nenhum dos dois.
  if (IPI_TAXED_CSTS.has(String(data.ipi_cst ?? '')) && !filled(data.ipi_aliq) && !filled(data.ipi_v_unid)) {
    ctx.addIssue({
      code: 'custom',
      path: ['ipi_aliq'],
      message: 'CST de IPI tributado exige alíquota ou valor por unidade',
    })
  }

  // ICMSPart: a partilha precisa do percentual da operação própria e da UF do ST.
  requireTogether(
    ['icms_part_p_bc_op', 'icms_part_uf_st'],
    'A partilha do ICMS exige o percentual da operação própria e a UF do ST',
  )

  // modBC por pauta ou por lista negociada exige o valor da pauta.
  if (ICMS_MOD_BC_PAUTA.has(String(data.icms_mod_bc ?? '')) && !filled(data.icms_pauta_valor)) {
    ctx.addIssue({
      code: 'custom',
      path: ['icms_pauta_valor'],
      message: 'Esta modalidade de base de cálculo exige o valor da pauta',
    })
  }

  requireTogether(
    ['alc_zfm_tp_cbs', 'alc_zfm_n_proc_suframa'],
    'A alíquota zero da CBS em ALC/ZFM exige o tipo e o processo SUFRAMA',
  )

  requireTogether(
    ['obs_item_x_campo', 'obs_item_x_texto'],
    'A observação do item exige campo e texto',
  )
}

export const cfopConfigSchema = cfopConfigBase.superRefine(applyTaxGroupRules)

const nullableStr = (schema: z.ZodString) =>
  schema.or(z.literal('')).optional()


/**
 * Dígito verificador GS1 (mod 10, pesos 3/1 da direita para a esquerda). A SEFAZ
 * valida o GTIN: um EAN com dígito errado é rejeição de nota, não aviso de
 * cadastro — e é conta, que é justamente o que o operador não deve fazer.
 */
export function isValidGtin(code: string): boolean {
  if (!/^\d{8}$|^\d{12,14}$/.test(code)) return false
  const digits = code.split('').map(Number)
  const check = digits.pop() as number
  let sum = 0
  for (let i = digits.length - 1, weight = 3; i >= 0; i--, weight = weight === 3 ? 1 : 3) {
    sum += digits[i] * weight
  }
  return (10 - (sum % 10)) % 10 === check
}

/** Tags de veicProd exigidas pelo leiaute — espelha `veicProdTagOrder` no builder Go. */
const VEICULO_REQUIRED: {field: keyof ProductFormData; label: string}[] = [
  {field: 'veic_tp_op', label: 'tipo de operação'},
  {field: 'veic_tp_comb', label: 'combustível'},
  {field: 'veic_tp_pint', label: 'tipo de pintura'},
  {field: 'veic_tp_veic', label: 'tipo RENAVAM'},
  {field: 'veic_esp_veic', label: 'espécie RENAVAM'},
  {field: 'veic_vin', label: 'VIN remarcado'},
  {field: 'veic_cond_veic', label: 'condição'},
  {field: 'veic_c_mod', label: 'código marca/modelo'},
  {field: 'veic_c_cor_denatran', label: 'cor DENATRAN'},
  {field: 'veic_c_cor', label: 'código da cor'},
  {field: 'veic_x_cor', label: 'descrição da cor'},
  {field: 'veic_lota', label: 'lotação'},
  {field: 'veic_tp_rest', label: 'restrição'},
  {field: 'veic_ano_mod', label: 'ano modelo'},
  {field: 'veic_ano_fab', label: 'ano de fabricação'},
  {field: 'veic_pot', label: 'potência'},
  {field: 'veic_cilin', label: 'cilindradas'},
  {field: 'veic_cmt', label: 'capacidade máxima de tração'},
  {field: 'veic_dist', label: 'distância entre eixos'},
  {field: 'net_weight', label: 'peso líquido'},
  {field: 'gross_weight', label: 'peso bruto'},
]

const ANVISA_ISENTO = 'ISENTO'

/** Índices das tabelas oficiais, para a validação não varrê-las por tecla. */
const RISK_CLASS_CODES = new Set(RISK_CLASSES.filter((c) => !c.parentOnly).map((c) => c.code))
const IPI_CENQ_CODES = new Set(IPI_CENQ.map((e) => e.code))
const SEM_GTIN = 'SEM GTIN'

const productSchemaBase = z.object({
  code: z
    .string()
    .min(1, 'Código é obrigatório')
    .max(60, 'Máximo 60 caracteres')
    .regex(/^[A-Z0-9._\-]+$/, 'Apenas A–Z, 0–9 e . _ -'),
  description: z
    .string()
    .min(2, 'Mínimo 2 caracteres')
    .max(255, 'Máximo 255 caracteres'),
  brand: nullableStr(z.string().max(60, 'Máximo 60 caracteres')),
  ncm: z.string().regex(/^\d{8}$/, 'NCM deve ter 8 dígitos'),
  cest: nullableStr(z.string().regex(/^\d{7}$/, 'CEST deve ter 7 dígitos')),
  origin: z.string().min(1, 'Origem obrigatória'),
  unit: z.string().min(1, 'Unidade obrigatória').max(6),
  taxable_unit: nullableStr(z.string().max(6)),
  cean: nullableStr(
    z.string().regex(
      /^(\d{8,14}|SEM GTIN)$/,
      'EAN inválido (8–14 dígitos ou "SEM GTIN")'
    )
  ),
  taxable_cean: nullableStr(
    z.string().regex(
      /^(\d{8,14}|SEM GTIN)$/,
      'EAN inválido (8–14 dígitos ou "SEM GTIN")'
    )
  ),
  value: z
    .string()
    .regex(/^\d+(\.\d{1,4})?$/, 'Valor inválido (ex: 99.90)'),
  value_resale: nullableStr(
    z.string().regex(/^\d+(\.\d{1,4})?$/, 'Valor inválido (ex: 99.90)')
  ),
  net_weight: nullableStr(
    z.string().regex(/^\d+(\.\d{1,3})?$/, 'Peso inválido (ex: 10.500)')
  ),
  gross_weight: nullableStr(
    z.string().regex(/^\d+(\.\d{1,3})?$/, 'Peso inválido (ex: 10.500)')
  ),
  // Campos fiscais do produto
  c_benef: nullableStr(z.string().regex(/^([!-ÿ]{8}|[!-ÿ]{10}|SEM CBENEF)$/, 'Código inválido (8 ou 10 chars, ou SEM CBENEF)')),
  ext_ipi: nullableStr(z.string().regex(/^\d{2,3}$/, 'EX TIPI deve ter 2 ou 3 dígitos')),
  ind_escala: z.enum(['S', 'N']).optional().or(z.literal('')),
  cnpj_fab: nullableStr(z.string().regex(/^\d{14}$/, 'CNPJ deve ter 14 dígitos')),
  ind_tot: z.enum(['0', '1']),
  icms_aliq_override: nullableStr(z.string().regex(_percentRegex, '% inválido')),
  fcp_aliq_override: nullableStr(z.string().regex(_percentRegex, '% inválido')),
  inf_ad_prod: nullableStr(z.string().max(500, 'Máximo 500 caracteres')),
  cfop_nfce: z.string().regex(/^\d{4}$/, 'CFOP NFC-e deve ter 4 dígitos'),
  cfop_config: z
    .array(cfopConfigSchema),
  conversion_factors: z.array(conversionFactorSchema),
  // Tipo específico e campos especiais
  prod_type: z.enum(['generic', 'comb', 'med', 'veiculo', 'arma']).optional().or(z.literal('')),
  comb_c_prod_anp: nullableStr(z.string().regex(/^\d{9}$/, 'ANP deve ter 9 dígitos')),
  comb_desc_anp: nullableStr(z.string().max(95, 'Máximo 95 caracteres')),
  comb_uf_cons: nullableStr(z.string().regex(/^[A-Z]{2}$/, 'UF inválida')),
  comb_codif: nullableStr(z.string().max(21, 'Máximo 21 dígitos')),
  comb_p_glp: nullableStr(z.string().regex(_percentRegex, '% inválido')),
  comb_p_gnn: nullableStr(z.string().regex(_percentRegex, '% inválido')),
  comb_p_gni: nullableStr(z.string().regex(_percentRegex, '% inválido')),
  comb_v_part: nullableStr(z.string().regex(/^\d+(\.\d{1,2})?$/, 'Valor inválido')),
  comb_cide_v_aliq_prod: z.string().optional().or(z.literal('')),
  comb_orig: z.array(z.object({
    ind_import: z.enum(['0', '1']),
    c_uf_orig: z.string().regex(/^\d{2}$/, 'Código IBGE da UF: 2 dígitos'),
    p_orig: z.string().min(1, 'Percentual obrigatório'),
  })).optional(),
  comb_p_bio: nullableStr(z.string().regex(_percentRegex, '% inválido')),
  med_c_prod_anvisa: nullableStr(z.string().min(5, 'Campo inválido')),
  med_x_motivo_isencao: nullableStr(z.string().max(255)),
  med_v_pmc: nullableStr(z.string().regex(/^\d+(\.\d{1,2})?$/, 'Valor inválido')),
  // peri — classificação de produto perigoso (MDF-e). Cadastrada uma vez aqui;
  // o MDF-e a deriva sozinho ao referenciar a NF-e que contém o item.
  // Selo de controle do IPI e enquadramento legal — nível produto
  ipi_cnpj_prod: nullableStr(z.string().regex(/^\d{14}$/, 'CNPJ tem 14 dígitos')),
  ipi_c_selo: nullableStr(z.string().max(60)),
  ipi_q_selo: nullableStr(z.string().regex(/^\d{1,12}$/, 'Quantidade inválida')),
  ipi_c_enq: nullableStr(z.string().regex(/^\d{1,3}$/, 'Enquadramento tem até 3 dígitos')),
  // NVE, FCI e códigos de barra próprios — nível produto
  nve: z.array(z.string().length(6, 'NVE tem 6 caracteres')).max(8).optional(),
  n_fci: nullableStr(z.string().regex(/^[0-9a-fA-F-]{36}$/, 'FCI inválida')),
  c_barra: nullableStr(z.string().max(30)),
  c_barra_trib: nullableStr(z.string().max(30)),
  /** RECOPI do papel imune (prod/nRECOPI) — 20 dígitos. Último ramo do choice
   *  de prod: com comb/med/veicProd/arma no item, não é emitido. */
  n_recopi: nullableStr(z.string().regex(/^\d{20}$/, 'RECOPI tem 20 dígitos')),
  /** Créditos presumidos da UF aplicados ao item (prod/gCred, máx. 4). O valor
   *  é derivado do percentual sobre o valor do item na emissão. */
  gcred: z.array(z.object({
    c_cred_presumido: z.string().regex(/^.{8}$|^.{10}$/, 'Código tem 8 ou 10 caracteres'),
    p_cred_presumido: z.string().regex(/^\d{1,3}(\.\d{1,4})?$/, 'Percentual inválido'),
  })).max(4).optional(),
  /** Classificação da subapuração do IBS na ZFM (prod/tpCredPresIBSZFM). */
  tp_cred_pres_ibs_zfm: z.enum(['0', '1', '2', '3', '4']).optional().or(z.literal('')),
  /** prod/indBemMovelUsado. O XSD enumera um valor só: 1. */
  ind_bem_movel_usado: z.enum(['1']).optional().or(z.literal('')),
  peri_n_onu: nullableStr(z.string().regex(/^\d{1,4}$/, 'ONU tem até 4 dígitos')),
  peri_x_nome_ae: nullableStr(z.string().max(150)),
  peri_x_cla_risco: nullableStr(z.string().max(40)),
  peri_gr_emb: nullableStr(z.string().max(6)),
  peri_q_vol_tipo: nullableStr(z.string().max(60)),
  // veicProd — dados do modelo
  veic_tp_op: nullableStr(z.string().regex(/^[0-3]$/, 'Valor inválido')),
  veic_tp_comb: nullableStr(z.string().max(2, 'Máximo 2 chars')),
  veic_tp_pint: nullableStr(z.string().max(1, 'Máximo 1 char')),
  veic_tp_veic: nullableStr(z.string().regex(/^\d{1,2}$/, '1-2 dígitos')),
  veic_esp_veic: nullableStr(z.string().regex(/^\d$/, '1 dígito')),
  veic_vin: nullableStr(z.string().regex(/^[RN]$/, 'R ou N')),
  veic_cond_veic: nullableStr(z.string().regex(/^[1-3]$/, '1, 2 ou 3')),
  veic_c_mod: nullableStr(z.string().regex(/^\d{1,6}$/, '1-6 dígitos')),
  veic_c_cor_denatran: nullableStr(z.string().regex(/^\d{1,2}$/, '1-2 dígitos')),
  veic_lota: nullableStr(z.string().regex(/^\d{1,3}$/, '1-3 dígitos')),
  veic_tp_rest: nullableStr(z.string().regex(/^[0-4]$|^9$/, '0-4 ou 9')),
  veic_ano_mod: nullableStr(z.string().regex(/^\d{4}$/, '4 dígitos')),
  veic_ano_fab: nullableStr(z.string().regex(/^\d{4}$/, '4 dígitos')),
  veic_pot: nullableStr(z.string().max(4, 'Máximo 4 chars')),
  veic_cilin: nullableStr(z.string().max(4, 'Máximo 4 chars')),
  veic_cmt: nullableStr(z.string().max(9, 'Máximo 9 chars')),
  veic_dist: nullableStr(z.string().max(4, 'Máximo 4 chars')),
  veic_c_cor: nullableStr(z.string().max(4, 'Máximo 4 chars')),
  veic_x_cor: nullableStr(z.string().max(40, 'Máximo 40 chars')),
  // arma
  arma_tp_arma: nullableStr(z.string().regex(/^[01]$/, '0 ou 1')),
  arma_descr: nullableStr(z.string().max(256, 'Máximo 256 chars')),
})

type ProductFormBase = z.infer<typeof productSchemaBase>

export type ProductFormData = ProductFormBase

/**
 * Regras cruzadas do leiaute. Cada uma existe hoje como texto de ajuda ao lado
 * do campo; aqui elas passam a recusar o cadastro, que é o único jeito de a
 * rejeição não chegar dias depois, na emissão, numa tela diferente.
 */
export const productSchema = productSchemaBase.superRefine((data, ctx) => {
  const filled = (v: unknown): boolean => typeof v === 'string' && v.trim() !== ''
  const require = (field: keyof ProductFormBase, message: string) => {
    if (!filled(data[field])) ctx.addIssue({code: 'custom', path: [field], message})
  }

  if (data.prod_type === 'comb') {
    require('comb_c_prod_anp', 'Obrigatório para combustível')
    require('comb_desc_anp', 'Obrigatório para combustível')
    require('comb_uf_cons', 'Obrigatório para combustível')
  }

  if (data.prod_type === 'veiculo') {
    for (const {field, label} of VEICULO_REQUIRED) {
      require(field, `Obrigatório para veículo novo (${label})`)
    }
  }

  if (data.prod_type === 'arma') {
    require('arma_tp_arma', 'Obrigatório para armamento')
  }

  // cProdANVISA = ISENTO exige o motivo; registro numérico o proíbe.
  if (data.med_c_prod_anvisa === ANVISA_ISENTO) {
    require('med_x_motivo_isencao', 'Obrigatório quando o registro é ISENTO')
  } else if (filled(data.med_c_prod_anvisa) && filled(data.med_x_motivo_isencao)) {
    ctx.addIssue({
      code: 'custom',
      path: ['med_x_motivo_isencao'],
      message: 'Só se aplica quando o registro ANVISA é ISENTO',
    })
  }

  // indEscala = N (produção fora de escala relevante) exige o fabricante.
  if (data.ind_escala === 'N') {
    require('cnpj_fab', 'Obrigatório quando a produção é fora de escala relevante')
  }

  // O selo de controle do IPI é um grupo: código e quantidade andam juntos.
  if (filled(data.ipi_c_selo) !== filled(data.ipi_q_selo)) {
    const missing = filled(data.ipi_c_selo) ? 'ipi_q_selo' : 'ipi_c_selo'
    ctx.addIssue({code: 'custom', path: [missing], message: 'Código e quantidade do selo andam juntos'})
  }

  // peri: com o número ONU, o resto do grupo é obrigatório no MDF-e que
  // referenciar esta nota.
  if (filled(data.peri_n_onu)) {
    require('peri_x_nome_ae', 'Obrigatório quando há número ONU')
    require('peri_x_cla_risco', 'Obrigatório quando há número ONU')
    // Classe 1, 2, 5.2, 6.2 e 7 não recebem grupo de embalagem (Res. ANTT 5.998/2022).
    if (packingGroupApplies(data.peri_x_cla_risco)) {
      require('peri_gr_emb', 'Obrigatório quando há número ONU')
    }
  }

  if (filled(data.peri_x_cla_risco) && !RISK_CLASS_CODES.has(data.peri_x_cla_risco as string)) {
    ctx.addIssue({code: 'custom', path: ['peri_x_cla_risco'], message: 'Classe de risco não existe na tabela da ANTT'})
  }

  if (filled(data.peri_gr_emb) && !packingGroupApplies(data.peri_x_cla_risco)) {
    ctx.addIssue({
      code: 'custom',
      path: ['peri_gr_emb'],
      message: 'Esta classe de risco não recebe grupo de embalagem',
    })
  }

  if (filled(data.ipi_c_enq) && !IPI_CENQ_CODES.has(data.ipi_c_enq as string)) {
    ctx.addIssue({code: 'custom', path: ['ipi_c_enq'], message: 'Enquadramento legal do IPI não existe na tabela'})
  }

  // Origem do combustível: o rateio tem que fechar em 100%.
  if (data.comb_orig && data.comb_orig.length > 0) {
    const total = data.comb_orig.reduce((sum, o) => sum + (parseFloat(o.p_orig) || 0), 0)
    if (Math.abs(total - 100) >= 0.01) {
      ctx.addIssue({
        code: 'custom',
        path: ['comb_orig'],
        message: `Os percentuais de origem somam ${total.toFixed(2)}% — têm que somar 100%`,
      })
    }
  }

  // Peso bruto inclui a embalagem: nunca é menor que o líquido.
  const net = parseFloat(data.net_weight ?? '')
  const gross = parseFloat(data.gross_weight ?? '')
  if (!Number.isNaN(net) && !Number.isNaN(gross) && gross < net) {
    ctx.addIssue({
      code: 'custom',
      path: ['gross_weight'],
      message: 'Peso bruto não pode ser menor que o líquido',
    })
  }

  for (const field of ['cean', 'taxable_cean'] as const) {
    const value = data[field]
    if (filled(value) && value !== SEM_GTIN && !isValidGtin(value as string)) {
      ctx.addIssue({code: 'custom', path: [field], message: 'Dígito verificador do GTIN inválido'})
    }
  }
})

export type CfopConfigFormData = z.infer<typeof cfopConfigSchema>
export type ConversionFactorFormData = z.infer<typeof conversionFactorSchema>
