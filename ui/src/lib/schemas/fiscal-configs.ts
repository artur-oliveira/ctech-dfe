import {z} from 'zod'

const serieField = z.string().regex(/^\d{1,3}$/, 'Série deve ter entre 1 e 3 dígitos')
const numberField = z.string().regex(/^\d+$/, 'Número inválido')

export const BRAZIL_TIMEZONES = [
  'America/Sao_Paulo',
  'America/Belem',
  'America/Fortaleza',
  'America/Recife',
  'America/Maceio',
  'America/Bahia',
  'America/Manaus',
  'America/Cuiaba',
  'America/Porto_Velho',
  'America/Boa_Vista',
  'America/Rio_Branco',
  'America/Noronha',
] as const

export type BrazilTimezone = typeof BRAZIL_TIMEZONES[number]

export const TIMEZONE_LABELS: Record<BrazilTimezone, string> = {
  'America/Sao_Paulo':   'Brasília / São Paulo (UTC-3)',
  'America/Belem':       'Belém / Macapá (UTC-3)',
  'America/Fortaleza':   'Fortaleza / Natal (UTC-3)',
  'America/Recife':      'Recife (UTC-3)',
  'America/Maceio':      'Maceió / Aracaju (UTC-3)',
  'America/Bahia':       'Salvador (UTC-3)',
  'America/Manaus':      'Manaus / Boa Vista (UTC-4)',
  'America/Cuiaba':      'Cuiabá / Campo Grande (UTC-4)',
  'America/Porto_Velho': 'Porto Velho (UTC-4)',
  'America/Boa_Vista':   'Boa Vista (UTC-4)',
  'America/Rio_Branco':  'Rio Branco / Eirunepé (UTC-5)',
  'America/Noronha':     'Fernando de Noronha (UTC-2)',
}

const baseFields = {
  timezone: z.enum(BRAZIL_TIMEZONES),
  environment: z.enum(['1', '2']),
  prod_current_serie: serieField,
  prod_current_number: numberField,
  hom_current_serie: serieField,
  hom_current_number: numberField,
  /**
   * CSRT do responsável técnico (NT 2018.005). Segredo: a API nunca o devolve,
   * então o campo volta vazio ao reabrir a tela — em branco significa "manter",
   * não "apagar".
   */
  csrt_id: z.string().regex(/^\d{1,2}$/, 'ID do CSRT: 1 ou 2 dígitos').optional().or(z.literal('')),
  csrt: z.string().length(36, 'O CSRT tem 36 caracteres').optional().or(z.literal('')),
}

export const nfeConfigSchema = z.object({ ...baseFields })
export const cteConfigSchema = nfeConfigSchema
// O MDF-e acrescenta três campos que só existem no leiaute dele e recorrem em
// toda emissão da organização — a observação da viagem continua na emissão.
export const mdfeConfigSchema = z.object({
  ...baseFields,
  ind_canal_verde: z.boolean(),
  ind_carrega_posterior: z.boolean(),
  inf_ad_fisco: z.string().max(2000).optional().or(z.literal('')),
})

export const nfceConfigSchema = z.object({
  ...baseFields,
  prod_csc: z.string().min(1, 'CSC obrigatório').max(36, 'Máximo 36 caracteres'),
  prod_csc_id: z.string().regex(/^\d+$/, 'ID do CSC inválido'),
  hom_csc: z.string().min(1, 'CSC obrigatório').max(36, 'Máximo 36 caracteres'),
  hom_csc_id: z.string().regex(/^\d+$/, 'ID do CSC inválido'),
})

export type NFeConfigFormData  = z.infer<typeof nfeConfigSchema>
export type NFCeConfigFormData = z.infer<typeof nfceConfigSchema>
export type CTeConfigFormData  = z.infer<typeof cteConfigSchema>
export type MDFeConfigFormData = z.infer<typeof mdfeConfigSchema>

// NFS-e não embeda baseFields: uma única serie (não prod/hom), e o formato
// do provider nacional troca
// prod_current_serie por um município emissor. Ver api/internal/api/v1/dto.go
// NfseConfigBody e docs/specs/2026-08-04-nfse-design.md §3.3.
export const nfseConfigSchema = z.object({
  provider: z.enum(['nacional', 'abrasf204']),
  environment: z.enum(['1', '2']),
  timezone: z.enum(BRAZIL_TIMEZONES),
  c_loc_emi: z.string().regex(/^\d{7}$/, 'Código IBGE deve ter 7 dígitos'),
  serie: serieField,
  prod_current_number: numberField,
  hom_current_number: numberField,
  certificate_sk: z.string().optional().or(z.literal('')),
  abrasf_endpoint_url: z.string().url('URL inválida').optional().or(z.literal('')),
  abrasf_wsdl_version: z.string().max(10).optional().or(z.literal('')),
  abrasf_municipality_code: z.string().regex(/^\d{7}$/, 'Código IBGE deve ter 7 dígitos').optional().or(z.literal('')),
  abrasf_synchronous: z.boolean().optional(),
}).superRefine((v, ctx) => {
  if (v.provider !== 'abrasf204') return
  if (!v.abrasf_endpoint_url) {
    ctx.addIssue({code: z.ZodIssueCode.custom, path: ['abrasf_endpoint_url'], message: 'Endpoint obrigatório para ABRASF 2.04'})
  }
  if (!v.abrasf_wsdl_version) {
    ctx.addIssue({code: z.ZodIssueCode.custom, path: ['abrasf_wsdl_version'], message: 'Versão do WSDL obrigatória para ABRASF 2.04'})
  }
  if (!v.abrasf_municipality_code) {
    ctx.addIssue({code: z.ZodIssueCode.custom, path: ['abrasf_municipality_code'], message: 'Código do município obrigatório para ABRASF 2.04'})
  }
})

export type NfseConfigFormData = z.infer<typeof nfseConfigSchema>

export type DocVariant = 'nfe' | 'nfce' | 'cte' | 'mdfe' | 'nfse'
