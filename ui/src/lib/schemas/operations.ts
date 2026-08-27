/**
 * Natureza de operação — os valores que sempre andam juntos por cenário de
 * negócio. Espelha OperationBody (api/internal/api/v1/dto.go).
 */
import {z} from 'zod'

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
  requires_receiver: z.boolean(),
  is_default: z.boolean(),
})

export type OperationFormData = z.infer<typeof operationSchema>
