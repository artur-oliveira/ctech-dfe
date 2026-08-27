/**
 * Fornecedora de vale-pedágio. Espelha TollProviderBody
 * (api/internal/api/v1/dto.go). O vale é obrigatório no transporte rodoviário
 * de carga (Lei 10.209); a fornecedora e o pagador são invariantes, então por
 * viagem só entram número da compra e valor.
 */
import {z} from 'zod'
import {validateCNPJ, validateCPF} from '@/lib/utils/validators'

/** tpValePed — como o vale foi adquirido. */
export const TP_VALE_PED_OPTIONS = [
  {value: '01', label: '01 – TAG'},
  {value: '02', label: '02 – Cupom'},
  {value: '03', label: '03 – Cartão'},
]

export const tollProviderSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  cnpj_forn: z.string().refine((v) => validateCNPJ(v.replace(/\D/g, '')), 'CNPJ inválido'),
  cnpj_pg: z.string().optional().or(z.literal('')),
  cpf_pg: z.string().optional().or(z.literal('')),
  tp_vale_ped: z.enum(['01', '02', '03']).optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  // CNPJPg e CPFPg são um choice no XSD: no máximo um pagador.
  if (v.cnpj_pg && v.cpf_pg) {
    ctx.addIssue({code: 'custom', path: ['cpf_pg'], message: 'Informe CNPJ ou CPF do pagador, nunca os dois'})
  }
  if (v.cnpj_pg && !validateCNPJ(v.cnpj_pg.replace(/\D/g, ''))) {
    ctx.addIssue({code: 'custom', path: ['cnpj_pg'], message: 'CNPJ inválido'})
  }
  if (v.cpf_pg && !validateCPF(v.cpf_pg.replace(/\D/g, ''))) {
    ctx.addIssue({code: 'custom', path: ['cpf_pg'], message: 'CPF inválido'})
  }
})

export type TollProviderFormData = z.infer<typeof tollProviderSchema>
