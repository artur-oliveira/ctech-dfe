/**
 * Unified schema for both persons and organizations.
 * Organizations additionally use the `description` and `person.contacts` fields.
 */
import {z} from 'zod'
import {validateCNPJ, validateCPF} from '@/lib/utils/validators'

export const UF_LIST = [
  'AC', 'AL', 'AM', 'AP', 'BA', 'CE', 'DF', 'ES', 'GO', 'MA',
  'MG', 'MS', 'MT', 'PA', 'PB', 'PE', 'PI', 'PR', 'RJ', 'RN',
  'RO', 'RR', 'RS', 'SC', 'SE', 'SP', 'TO',
] as const

export const addressSchema = z.object({
  city_ibge_code: z.string().regex(/^\d{7}$/, 'Código IBGE deve ter 7 dígitos'),
  street: z.string().min(1, 'Logradouro obrigatório').max(255),
  neighborhood: z.string().min(1, 'Bairro obrigatório').max(120),
  number: z.string().min(1, 'Número obrigatório').max(20),
  city: z.string().min(1, 'Cidade obrigatória').max(120),
  state_federation: z.enum(UF_LIST, {error: 'UF inválida'}),
  postal_code: z.string().regex(/^\d{8}$/, 'CEP deve ter 8 dígitos'),
  complement: z.string().max(120).optional().or(z.literal('')),
})

export const stateRegistrationSchema = z.object({
  uf: z.enum(UF_LIST, {error: 'UF inválida'}),
  state_registration: z.string().min(1, 'IE obrigatória').max(20),
})

// Sentinel select value meaning "no CRT" for pessoa física. Lives in form state so
// the Radix Select stays controlled; converted to null on submit (CLAUDE: no magic strings).
export const CRT_NONE_VALUE = '__none__'

export const entitySchema = z.object({
  tipo: z.enum(['pf', 'pj']),
  cpf_or_cnpj: z.string().min(1, 'CPF/CNPJ obrigatório'),
  name: z.string().min(2, 'Mínimo 2 caracteres').max(255),
  description: z.string().max(120).optional().or(z.literal('')),
  person: z.object({
    fantasy_name: z.string().max(255).optional().or(z.literal('')),
    crt: z.enum(['1', '2', '3', '4', CRT_NONE_VALUE]).optional(),
    state_registrations: z.array(stateRegistrationSchema).default([]),
    addresses: z.array(addressSchema).min(1, 'Ao menos um endereço obrigatório'),
    contacts: z.object({
      emails: z.email('E-mail inválido').array().max(5, 'Máximo 5 e-mails'),
      phones: z.string().regex(/^\d{10,11}$/, 'Telefone inválido').array().max(5, 'Máximo 5 telefones'),
    }).default({emails: [], phones: []}),
  }),
}).superRefine((data, ctx) => {
  const raw = data.cpf_or_cnpj.replace(/[^A-Z0-9]/gi, '').toUpperCase()
  if (data.tipo === 'pf' && !validateCPF(raw)) {
    ctx.addIssue({code: 'custom', message: 'CPF inválido', path: ['cpf_or_cnpj']})
  }
  if (data.tipo === 'pj' && !validateCNPJ(raw)) {
    ctx.addIssue({code: 'custom', message: 'CNPJ inválido', path: ['cpf_or_cnpj']})
  }
  if (data.tipo === 'pj' && (!data.person.crt || data.person.crt === CRT_NONE_VALUE)) {
    ctx.addIssue({code: 'custom', message: 'CRT obrigatório', path: ['person', 'crt']})
  }
  const ufs = data.person.state_registrations.map((r) => r.uf)
  const dup = ufs.find((uf, i) => ufs.indexOf(uf) !== i)
  if (dup) {
    ctx.addIssue({code: 'custom', message: `UF duplicada: ${dup}`, path: ['person', 'state_registrations']})
  }
})

export type EntityFormData = z.infer<typeof entitySchema>
export type AddressData = z.infer<typeof addressSchema>
export type StateRegistrationData = z.infer<typeof stateRegistrationSchema>

export const UF_OPTIONS = UF_LIST.map((uf) => ({value: uf, label: uf}))

export const CRT_OPTIONS_PJ = [
  {value: '1', label: 'Simples Nacional'},
  {value: '2', label: 'Simples Nacional (excesso de sublimite)'},
  {value: '3', label: 'Regime Normal'},
  {value: '4', label: 'MEI – Microempreendedor Individual'},
]

export const CRT_OPTIONS_ORG_PF = [
  {value: '3', label: 'Regime Normal'},
  {value: '4', label: 'MEI – Microempreendedor Individual'},
]