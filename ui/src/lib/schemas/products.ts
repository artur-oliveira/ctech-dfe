import {z} from 'zod'

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

const optionalStr = z.string().optional().or(z.literal(''))
const optionalPercent = z.string().regex(_percentRegex, '% inválido').optional().or(z.literal(''))

export const ufTaxOverrideSchema = z.object({
  ufs: z.array(z.string().regex(/^[A-Z]{2}$/, 'UF inválida')).min(1, 'Escolha ao menos uma UF'),
  overrides: z.record(z.string(), z.unknown()),
})

export const cfopConfigSchema = z.object({
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
  ibs_ind_doacao: optionalStr,
  ibs_ad_rem: optionalPercent,
  cbs_ad_rem: optionalPercent,
  ibs_cbs_p_dev_trib: optionalPercent,
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
  issqn_c_pais: optionalStr,
  issqn_n_processo: optionalStr,
  issqn_ind_incentivo: optionalStr,
  issqn_v_iss_ret: optionalStr,
  // Overrides por UF de destino — só preenche o que diverge para aquelas UFs
  uf_overrides: z.array(ufTaxOverrideSchema).optional(),
})

const nullableStr = (schema: z.ZodString) =>
  schema.or(z.literal('')).optional()

export const productSchema = z.object({
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

export type ProductFormData = z.infer<typeof productSchema>
export type CfopConfigFormData = z.infer<typeof cfopConfigSchema>
export type ConversionFactorFormData = z.infer<typeof conversionFactorSchema>
