/**
 * Natureza de operação — os valores que sempre andam juntos por cenário de
 * negócio. Espelha OperationBody (api/internal/api/v1/dto.go).
 */
import {z} from 'zod'
import {CFOP_SUFFIXES} from '@/lib/data/cfop'

export const DOC_TYPE_OPTIONS = [
  {value: 'nfe', label: 'NF-e'},
  {value: 'nfce', label: 'NFC-e'},
  {value: 'cte', label: 'CT-e'},
  {value: 'mdfe', label: 'MDF-e'},
] as const

/** Placeholders aceitos em inf_ad_fisco/inf_cpl — espelha services.AllPlaceholders. */
export const OPERATION_PLACEHOLDERS = [
  {key: 'v_nf', label: 'Valor total da nota'},
  {key: 'v_icms_st', label: 'Valor do ICMS ST'},
  {key: 'cliente', label: 'Nome do cliente'},
  {key: 'nat_op', label: 'Natureza da operação'},
  {key: 'competencia', label: 'Competência'},
] as const

const KNOWN_KEYS = new Set(OPERATION_PLACEHOLDERS.map((p) => p.key as string))
const PLACEHOLDER_RE = /\{\{\s*([a-z_]+)\s*\}\}/g

/** Primeira chave desconhecida do texto, ou null. Espelha services.ValidatePlaceholders. */
export function unknownPlaceholder(template: string): string | null {
  for (const match of template.matchAll(PLACEHOLDER_RE)) {
    if (!KNOWN_KEYS.has(match[1])) return match[1]
  }
  return null
}

const fiscalText = (max: number) => z.string().max(max).optional().or(z.literal(''))
  .superRefine((v, ctx) => {
    const unknown = v ? unknownPlaceholder(v) : null
    if (unknown) {
      ctx.addIssue({code: 'custom', message: `Placeholder desconhecido: {{${unknown}}}`})
    }
  })

/** Par campo/texto de infAdic (obsCont ou obsFisco). */
export const obsSchema = z.object({
  x_campo: z.string().min(1, 'Campo obrigatório').max(20),
  x_texto: z.string().min(1, 'Texto obrigatório').max(60),
})

export const operationSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  doc_types: z.array(z.enum(['nfe', 'nfce', 'cte', 'mdfe'])),
  nat_op: z.string().max(60).optional().or(z.literal('')),
  tp_nf: z.enum(['0', '1']).optional().or(z.literal('')),
  fin_nfe: z.enum(['1', '2', '3', '4']).optional().or(z.literal('')),
  ind_final: z.enum(['0', '1']).optional().or(z.literal('')),
  ind_pres: z.enum(['0', '1', '2', '3', '4', '5', '9']).optional().or(z.literal('')),
  // Natureza fiscal: 3 dígitos. O escopo (5/6/7) é resolvido na emissão.
  cfop_suffix: z.string().regex(/^\d{3}$/, 'Natureza fiscal: 3 dígitos').optional().or(z.literal('')),
  tax_profile_id: z.string().optional().or(z.literal('')),
  payment_term_id: z.string().optional().or(z.literal('')),
  mod_frete: z.enum(['0', '1', '2', '3', '4', '9']).optional().or(z.literal('')),
  /** Espécie e marca padrão dos volumes de transp/vol. */
  vol_esp: z.string().max(60).optional().or(z.literal('')),
  vol_marca: z.string().max(60).optional().or(z.literal('')),
  inf_ad_fisco: fiscalText(2000),
  /** Observações de campo livre de infAdic (obsCont/obsFisco), máx 10 cada. */
  obs_cont: z.array(obsSchema).max(10),
  obs_fisco: z.array(obsSchema).max(10),
  inf_cpl: fiscalText(5000),
  /** Perfil de retenções federais (total/retTrib). Percentuais; os valores
   *  saem da base da nota na emissão. */
  ret_trib: z.object({
    p_ret_pis: z.string().optional().or(z.literal('')),
    p_ret_cofins: z.string().optional().or(z.literal('')),
    p_ret_csll: z.string().optional().or(z.literal('')),
    p_ret_irrf: z.string().optional().or(z.literal('')),
    p_ret_prev_inss: z.string().optional().or(z.literal('')),
  }),
  /** Canal de venda: marketplace/plataforma do cenário (infNFe/infIntermed) e
   *  o indicador de ide/indIntermed. Uma operação por canal. */
  intermediary_person_id: z.string().optional().or(z.literal('')),
  ind_intermed: z.enum(['0', '1']).optional().or(z.literal('')),
  /** Prazo padrão de saída da mercadoria, em dias corridos a partir da emissão
   *  (ide/dhSaiEnt). O valor explícito na emissão vence. */
  dh_sai_ent_offset_days: z.string().regex(/^\d{1,3}$/, 'Dias: até 3 dígitos').optional().or(z.literal('')),
  /** ide da reforma tributária. Todos são do cenário, não da nota. */
  c_ind_op: z.string().regex(/^\d{6}$/, 'Código do local da operação: 6 dígitos').optional().or(z.literal('')),
  c_mun_fg_ibs: z.string().regex(/^\d{7}$/, 'Código IBGE tem 7 dígitos').optional().or(z.literal('')),
  tp_nf_debito: z.enum(['01', '02', '03', '04', '05', '06', '07', '08']).optional().or(z.literal('')),
  tp_nf_credito: z.enum(['01', '02', '03', '04', '05', '06']).optional().or(z.literal('')),
  /** Compras governamentais (ide/gCompraGov). O tipo de operação decide se a
   *  nota exige chaves de documentos anteriores — a emissão valida. */
  compra_gov_tp_ente: z.enum(['1', '2', '3', '4', '5', '6']).optional().or(z.literal('')),
  compra_gov_p_redutor: z.string().optional().or(z.literal('')),
  compra_gov_tp_oper: z.enum(['1', '2', '3', '4']).optional().or(z.literal('')),
  /** Nota de empenho do cenário de venda a órgão público (compra/xNEmp).
   *  Pedido e contrato variam por nota e são pedidos na emissão. */
  compra_x_n_emp: z.string().max(22).optional().or(z.literal('')),
  /** Safra do registro de aquisição de cana (cana/safra), ex. "2025/2026". */
  cana_safra: z.string().max(9).optional().or(z.literal('')),
  /** Exportação: UF de saída do país e índice do local de despacho salvo na
   *  organização (pickup_locations) — o endereço é referenciado, não copiado. */
  export_uf_saida_pais: z.string().optional().or(z.literal('')),
  export_loc_despacho_index: z.number().int().min(0).optional(),
  requires_receiver: z.boolean(),
  is_default: z.boolean(),
}).superRefine((data, ctx) => {
  // Três dígitos que passam no regex mas não existem na tabela CFOP são o erro
  // mais caro do cadastro: ele só aparece na emissão, em toda nota da operação.
  if (data.cfop_suffix && !CFOP_SUFFIXES.has(data.cfop_suffix)) {
    ctx.addIssue({
      code: 'custom',
      path: ['cfop_suffix'],
      message: 'Natureza fiscal não existe na tabela CFOP',
    })
  }

  // gCompraGov: o tipo do ente e o redutor formam grupo com o tipo de operação.
  const govFields = ['compra_gov_tp_ente', 'compra_gov_tp_oper'] as const
  const govFilled = govFields.filter((f) => data[f])
  if (govFilled.length === 1) {
    for (const field of govFields) {
      if (!data[field]) {
        ctx.addIssue({
          code: 'custom',
          path: [field],
          message: 'Compra governamental exige o tipo do ente e o tipo da operação',
        })
      }
    }
  }
})

export type OperationFormData = z.infer<typeof operationSchema>

/**
 * Safras disponíveis no seletor, no formato "AAAA/AAAA" do leiaute. Um select
 * fecha o campo: "25/26" digitado é rejeição, e a safra é sempre o par de anos
 * consecutivos.
 */
export function safraOptions(now: Date = new Date()): { value: string; label: string }[] {
  const current = now.getFullYear()
  const out: { value: string; label: string }[] = []
  for (let y = current + 1; y >= current - 3; y--) {
    const value = `${y}/${y + 1}`
    out.push({value, label: value})
  }
  return out
}
