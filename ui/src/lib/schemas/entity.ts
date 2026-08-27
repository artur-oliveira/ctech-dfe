/**
 * Unified schema for both persons and organizations.
 * Organizations additionally use the `description` and `person.contacts` fields.
 */
import {z} from 'zod'
import {validateCNPJ, validateCPF} from '@/lib/utils/validators'
import type {NfseInfo} from '@/lib/types/api'

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
  /** Inscrição de substituto tributário nesta UF (emit/IEST). */
  ie_st: z.string().max(20).optional().or(z.literal('')),
})

// Papéis de pessoa (services.AllPersonRoles em api/internal/services/person_roles.go).
// Papel é filtro de cadastro, não regra fiscal: a emissão nunca valida papel, e
// uma pessoa acumula quantos papéis forem verdade ao mesmo tempo — transportadora
// que também é cliente é o caso normal, não a exceção.
export const PERSON_ROLES = ['customer', 'supplier', 'carrier', 'driver', 'provider', 'freight_contractor'] as const
export type PersonRole = typeof PERSON_ROLES[number]

export const PERSON_ROLE_LABELS: Record<PersonRole, string> = {
  customer: 'Cliente',
  supplier: 'Fornecedor',
  carrier: 'Transportadora',
  driver: 'Condutor',
  provider: 'Prestador',
  freight_contractor: 'Contratante de frete',
}

// Papel pré-marcado num cadastro novo de pessoa.
export const PERSON_ROLE_DEFAULT: PersonRole = 'customer'

export const PERSON_ROLE_OPTIONS = PERSON_ROLES.map((value) => ({
  value,
  label: PERSON_ROLE_LABELS[value],
}))

// Sentinel select value meaning "no CRT" for pessoa física. Lives in form state so
// the Radix Select stays controlled; converted to null on submit (CLAUDE: no magic strings).
export const CRT_NONE_VALUE = '__none__'

// Grupo `nfse` do cadastro (NfseInfoBody em api/internal/api/v1/dto.go). É o
// que a emissão de NFS-e exige e o cadastro de NF-e não tem: inscrição
// municipal e regime tributário do prestador.
export const nfseInfoSchema = z.object({
  im: z.string().regex(/^\d{1,15}$/, 'Inscrição municipal: até 15 dígitos').optional().or(z.literal('')),
  // 1 não optante | 2 optante MEI | 3 optante ME/EPP
  op_simp_nac: z.enum(['1', '2', '3']).optional().or(z.literal('')),
  // Exigido apenas quando op_simp_nac = 3.
  reg_ap_trib_sn: z.enum(['1', '2', '3']).optional().or(z.literal('')),
  reg_esp_trib: z.enum(['0', '1', '2', '3', '4', '5', '6', '9']).optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  if (v.op_simp_nac === '3' && !v.reg_ap_trib_sn) {
    ctx.addIssue({
      code: 'custom',
      message: 'Obrigatório para optante do Simples (ME/EPP)',
      path: ['reg_ap_trib_sn'],
    })
  }
})

export const OP_SIMP_NAC_OPTIONS = [
  {value: '1', label: '1 – Não optante pelo Simples Nacional'},
  {value: '2', label: '2 – Optante — MEI'},
  {value: '3', label: '3 – Optante — Microempresa ou EPP'},
]

export const REG_AP_TRIB_SN_OPTIONS = [
  {value: '1', label: '1 – Federais e municipal pelo Simples'},
  {value: '2', label: '2 – Federais pelo Simples, ISSQN por fora'},
  {value: '3', label: '3 – Federais e municipal por fora do Simples'},
]

export const REG_ESP_TRIB_OPTIONS = [
  {value: '0', label: '0 – Nenhum'},
  {value: '1', label: '1 – Ato cooperado'},
  {value: '2', label: '2 – Estimativa'},
  {value: '3', label: '3 – Microempresa municipal'},
  {value: '4', label: '4 – Notário ou registrador'},
  {value: '5', label: '5 – Profissional autônomo'},
  {value: '6', label: '6 – Sociedade de profissionais'},
  {value: '9', label: '9 – Outros'},
]

export const entitySchema = z.object({
  tipo: z.enum(['pf', 'pj']),
  cpf_or_cnpj: z.string().default(''),
  /** Documento de pessoa no exterior (dest/idEstrangeiro). Alternativa a
   *  cpf_or_cnpj, nunca acompanhante — o XSD de dest é um choice. */
  id_estrangeiro: z.string().max(20).optional().or(z.literal('')),
  name: z.string().min(2, 'Mínimo 2 caracteres').max(255),
  description: z.string().max(120).optional().or(z.literal('')),
  // Só a variante 'person' renderiza e envia este campo; organização o ignora.
  roles: z.array(z.enum(PERSON_ROLES)).default([]),
  person: z.object({
    fantasy_name: z.string().max(255).optional().or(z.literal('')),
    crt: z.enum(['1', '2', '3', '4', CRT_NONE_VALUE]).optional(),
    state_registrations: z.array(stateRegistrationSchema).default([]),
    addresses: z.array(addressSchema).min(1, 'Ao menos um endereço obrigatório'),
    contacts: z.object({
      emails: z.email('E-mail inválido').array().max(5, 'Máximo 5 e-mails'),
      phones: z.string().regex(/^\d{10,11}$/, 'Telefone inválido').array().max(5, 'Máximo 5 telefones'),
    }).default({emails: [], phones: []}),
    nfse: nfseInfoSchema.default({im: '', op_simp_nac: '', reg_ap_trib_sn: '', reg_esp_trib: ''}),
    /** CNAE do emitente — exigido pelo leiaute quando IM está presente. */
    cnae: z.string().regex(/^\d{7}$/, 'CNAE tem 7 dígitos').optional().or(z.literal('')),
    /** Inscrição Suframa do emitente (emit/ISUFEmit). */
    isuf_emit: z.string().regex(/^\d{1,9}$/, 'Suframa é numérico').optional().or(z.literal('')),
  }),
}).superRefine((data, ctx) => {
  // Pessoa no exterior não tem CPF/CNPJ: os dois campos são exclusivos.
  if (data.id_estrangeiro) {
    if (data.cpf_or_cnpj) {
      ctx.addIssue({code: 'custom', message: 'Informe CPF/CNPJ ou documento estrangeiro, nunca os dois', path: ['id_estrangeiro']})
    }
    return
  }
  if (!data.cpf_or_cnpj) {
    ctx.addIssue({code: 'custom', message: 'CPF/CNPJ obrigatório', path: ['cpf_or_cnpj']})
    return
  }
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

// IE is optional at cadastro time — a PJ organization may be registered without
// any state registration (backend accepts empty state_registrations). Duplicate
// UF validation still applies via entitySchema's base superRefine.
export const organizationSchema = entitySchema

export type EntityFormData = z.infer<typeof entitySchema>
export type NfseInfoData = z.infer<typeof nfseInfoSchema>
export type AddressData = z.infer<typeof addressSchema>
export type StateRegistrationData = z.infer<typeof stateRegistrationSchema>

export const UF_OPTIONS = UF_LIST.map((uf) => ({value: uf, label: uf}))

export const CRT_OPTIONS_PJ = [
  {value: '1', label: 'Simples Nacional'},
  {value: '2', label: 'Simples Nacional (excesso de sublimite)'},
  {value: '3', label: 'Regime Normal'},
  {value: '4', label: 'MEI – Microempreendedor Individual'},
]

/** Estado do formulário → grupo `nfse` da API. Devolve null quando nada foi
 *  preenchido, para não gravar um grupo vazio no cadastro. */
export function nfseInfoToApi(v: NfseInfoData | undefined): NfseInfo | null {
  if (!v || (!v.im && !v.op_simp_nac)) return null
  return {
    im: v.im || null,
    reg_trib: v.op_simp_nac
      ? {
        op_simp_nac: Number(v.op_simp_nac),
        reg_ap_trib_sn: v.reg_ap_trib_sn ? Number(v.reg_ap_trib_sn) : null,
        reg_esp_trib: Number(v.reg_esp_trib || '0'),
      }
      : null,
  }
}

/** Grupo `nfse` da API → estado do formulário (selects sempre controlados). */
export function nfseInfoFromApi(v: NfseInfo | null | undefined): NfseInfoData {
  const reg = v?.reg_trib
  return {
    im: v?.im ?? '',
    op_simp_nac: (reg?.op_simp_nac != null ? String(reg.op_simp_nac) : '') as NfseInfoData['op_simp_nac'],
    reg_ap_trib_sn: (reg?.reg_ap_trib_sn != null ? String(reg.reg_ap_trib_sn) : '') as NfseInfoData['reg_ap_trib_sn'],
    reg_esp_trib: (reg?.reg_esp_trib != null ? String(reg.reg_esp_trib) : '') as NfseInfoData['reg_esp_trib'],
  }
}

export const CRT_OPTIONS_ORG_PF = [
  {value: '3', label: 'Regime Normal'},
  {value: '4', label: 'MEI – Microempreendedor Individual'},
]