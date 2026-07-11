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
}

export const nfeConfigSchema = z.object({ ...baseFields })
export const cteConfigSchema = nfeConfigSchema
export const mdfeConfigSchema = z.object({ ...baseFields })

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

export type DocVariant = 'nfe' | 'nfce' | 'cte' | 'mdfe'
