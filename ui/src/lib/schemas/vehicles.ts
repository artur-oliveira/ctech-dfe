import {z} from 'zod'
import {UF_LIST} from '@/lib/schemas/entity'

export const WHEELSET_OPTIONS = [
  { value: '01', label: 'Truck' },
  { value: '02', label: 'Toco' },
  { value: '03', label: 'Cavalo Mecânico' },
  { value: '04', label: 'VAN' },
  { value: '05', label: 'Utilitário' },
  { value: '99', label: 'Outros' },
]

export const BODYWORK_OPTIONS = [
  { value: '00', label: 'Não aplicável' },
  { value: '01', label: 'Aberto' },
  { value: '02', label: 'Fechado/Baú' },
  { value: '03', label: 'Graneleiro' },
  { value: '04', label: 'Porta Container' },
  { value: '05', label: 'Sider' },
]

export const OWNER_TYPE_OPTIONS = [
  { value: 'TAC', label: 'TAC – Transportador Autônomo' },
  { value: 'ETC', label: 'ETC – Empresa de Transporte' },
  { value: 'CTC', label: 'CTC – Cooperativa de Transporte' },
]

export {UF_OPTIONS} from '@/lib/schemas/entity'

const ownerSchema = z.object({
  cpf_cnpj: z.string().min(11, 'CPF/CNPJ obrigatório').max(14),
  rntrc: z.string().regex(/^\d{8,12}$/, 'RNTRC deve ter 8–12 dígitos'),
  name: z.string().min(2, 'Mínimo 2 caracteres').max(255),
  type: z.enum(['TAC', 'ETC', 'CTC'], { error: 'Tipo inválido' }),
})

const trailerSchema = z.object({
  plate: z
    .string()
    .regex(/^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/, 'Placa Mercosul inválida (ex: ABC1D23)'),
  plate_uf: z.enum(UF_LIST, { error: 'UF inválida' }),
  wheelset: z.string().min(1, 'Tipo de eixo obrigatório'),
  bodywork: z.string().min(1, 'Carroceria obrigatória'),
  renavam: z.string().regex(/^\d{9,11}$/, 'RENAVAM deve ter 9–11 dígitos'),
  weight: z.string().regex(/^\d+$/, 'Tara deve ser um número inteiro positivo'),
  owner: ownerSchema,
})

export const vehicleSchema = z.object({
  plate: z
    .string()
    .regex(/^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/, 'Placa Mercosul inválida (ex: ABC1D23)'),
  plate_uf: z.enum(UF_LIST, { error: 'UF inválida' }),
  wheelset: z.string().min(1, 'Tipo de eixo obrigatório'),
  bodywork: z.string().min(1, 'Carroceria obrigatória'),
  renavam: z.string().regex(/^\d{9,11}$/, 'RENAVAM deve ter 9–11 dígitos'),
  weight: z.string().regex(/^\d+$/, 'Tara deve ser um número inteiro positivo'),
  owner: ownerSchema,
  trailers: z.array(trailerSchema),
})

export type VehicleFormData = z.infer<typeof vehicleSchema>
export type TrailerFormData = z.infer<typeof trailerSchema>
export type OwnerFormData = z.infer<typeof ownerSchema>
