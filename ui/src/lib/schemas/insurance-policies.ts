/**
 * Apólice de seguro da carga (MDF-e infMDFe/seg). Espelha InsurancePolicyBody
 * (api/internal/api/v1/dto.go). A apólice e a seguradora recorrem entre
 * viagens; por viagem muda só a averbação, que vai no corpo da emissão.
 */
import {z} from 'zod'
import {validateCNPJ, validateCPF} from '@/lib/utils/validators'

/** respSeg — quem responde pelo seguro da carga. */
export const RESP_SEG_OPTIONS = [
  {value: '1', label: '1 – Emitente do MDF-e'},
  {value: '2', label: '2 – Contratante do serviço de transporte'},
]

/** Só o responsável que não é o emitente precisa se identificar. */
export const RESP_SEG_CONTRATANTE = '2'

export const insurancePolicySchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  resp_seg: z.enum(['1', '2']),
  cnpj: z.string().optional().or(z.literal('')),
  cpf: z.string().optional().or(z.literal('')),
  x_seg: z.string().max(30).optional().or(z.literal('')),
  cnpj_seg: z.string().optional().or(z.literal('')),
  n_apol: z.string().max(20).optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  // CNPJ e CPF são um choice no XSD: no máximo um responsável.
  if (v.cnpj && v.cpf) {
    ctx.addIssue({code: 'custom', path: ['cpf'], message: 'Informe CNPJ ou CPF, nunca os dois'})
  }
  // Mesma regra do backend: o responsável que não é o emitente se identifica.
  if (v.resp_seg === RESP_SEG_CONTRATANTE && !v.cnpj && !v.cpf) {
    ctx.addIssue({code: 'custom', path: ['cnpj'], message: 'Informe o CNPJ ou o CPF do contratante'})
  }
  if (v.cnpj && !validateCNPJ(v.cnpj.replace(/\D/g, ''))) {
    ctx.addIssue({code: 'custom', path: ['cnpj'], message: 'CNPJ inválido'})
  }
  if (v.cpf && !validateCPF(v.cpf.replace(/\D/g, ''))) {
    ctx.addIssue({code: 'custom', path: ['cpf'], message: 'CPF inválido'})
  }
  // Meia seguradora o XSD recusa: nome e CNPJ andam juntos.
  if (!!v.x_seg !== !!v.cnpj_seg) {
    ctx.addIssue({code: 'custom', path: ['cnpj_seg'], message: 'Informe nome e CNPJ da seguradora juntos'})
  }
  if (v.cnpj_seg && !validateCNPJ(v.cnpj_seg.replace(/\D/g, ''))) {
    ctx.addIssue({code: 'custom', path: ['cnpj_seg'], message: 'CNPJ inválido'})
  }
})

export type InsurancePolicyFormData = z.infer<typeof insurancePolicySchema>
