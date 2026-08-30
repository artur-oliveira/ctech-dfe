/**
 * Local de prestação de serviço da NFS-e. Espelha ServiceLocationBody
 * (api/internal/api/v1/dto.go).
 *
 * Os papéis são combináveis porque o XSD repete o mesmo endereço em
 * `serv/obra`, `serv/atvEvento` e `IBSCBS/imovel`: um canteiro que também é o
 * endereço do imóvel tributado seria dois cadastros idênticos se fossem
 * exclusivos.
 */
import {z} from 'zod'

export const SERVICE_LOCATION_ROLES = [
  {value: 'work', label: 'Obra'},
  {value: 'property', label: 'Imóvel'},
  {value: 'event_venue', label: 'Local de evento'},
] as const

/** Escopo do endereço: nacional pede CEP e município; exterior, cidade e região. */
export const ADDRESS_SCOPES = [
  {value: 'national', label: 'Brasil'},
  {value: 'foreign', label: 'Exterior'},
] as const

export const serviceLocationSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  roles: z.array(z.enum(['work', 'property', 'event_venue'])).min(1, 'Escolha ao menos um papel'),
  address_scope: z.enum(['national', 'foreign']),
  street: z.string().min(1, 'Logradouro obrigatório').max(255),
  number: z.string().min(1, 'Número obrigatório').max(60),
  complement: z.string().max(156).optional().or(z.literal('')),
  neighborhood: z.string().min(1, 'Bairro obrigatório').max(60),
  postal_code: z.string().optional().or(z.literal('')),
  city_ibge_code: z.string().optional().or(z.literal('')),
  foreign_postal_code: z.string().max(11).optional().or(z.literal('')),
  foreign_city: z.string().max(60).optional().or(z.literal('')),
  foreign_region: z.string().max(60).optional().or(z.literal('')),
  insc_imob_fisc: z.string().max(30).optional().or(z.literal('')),
  c_obra: z.string().max(30).optional().or(z.literal('')),
  cib: z.string().optional().or(z.literal('')),
  id_atv_evt: z.string().max(30).optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  if (v.address_scope === 'national') {
    if (!/^\d{8}$/.test(v.postal_code ?? '')) {
      ctx.addIssue({code: 'custom', path: ['postal_code'], message: 'CEP deve ter 8 dígitos'})
    }
    if (!/^\d{7}$/.test(v.city_ibge_code ?? '')) {
      ctx.addIssue({code: 'custom', path: ['city_ibge_code'], message: 'Escolha o município'})
    }
    return
  }
  if (!v.foreign_postal_code) {
    ctx.addIssue({code: 'custom', path: ['foreign_postal_code'], message: 'Código postal obrigatório'})
  }
  if (!v.foreign_city) {
    ctx.addIssue({code: 'custom', path: ['foreign_city'], message: 'Cidade obrigatória'})
  }
  if (!v.foreign_region) {
    ctx.addIssue({code: 'custom', path: ['foreign_region'], message: 'Estado/província/região obrigatório'})
  }
  // Mesma regra do backend: CNO, CIB e inscrição imobiliária são registros
  // brasileiros e não existem num endereço no exterior.
  for (const [field, value] of [
    ['c_obra', v.c_obra], ['cib', v.cib], ['insc_imob_fisc', v.insc_imob_fisc],
  ] as const) {
    if (value) {
      ctx.addIssue({code: 'custom', path: [field], message: 'Não se aplica a um local no exterior'})
    }
  }
}).superRefine((v, ctx) => {
  // serv/obra é a escolha cObra|cCIB|end: guardar os dois deixaria a emissão
  // decidir em silêncio qual ramo gerar.
  if (v.c_obra && v.cib) {
    ctx.addIssue({code: 'custom', path: ['cib'], message: 'Informe o código da obra OU o CIB, não os dois'})
  }
  if (v.cib && v.cib.length !== 8) {
    ctx.addIssue({code: 'custom', path: ['cib'], message: 'CIB tem 8 caracteres'})
  }
})

export type ServiceLocationFormData = z.infer<typeof serviceLocationSchema>
