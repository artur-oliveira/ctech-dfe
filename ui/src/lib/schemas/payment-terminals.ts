/**
 * Terminal de captura (POS). Espelha PaymentTerminalBody
 * (api/internal/api/v1/dto.go): CNPJ recebedor e identificador do terminal são
 * invariantes por maquininha, então a emissão só aponta o terminal.
 */
import {z} from 'zod'
import {validateCNPJ} from '@/lib/utils/validators'

const cnpj = z.string().refine((v) => validateCNPJ(v.replace(/\D/g, '')), 'CNPJ inválido')

export const paymentTerminalSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  cnpj_receb: cnpj,
  id_term_pag: z.string().min(1, 'Identificador obrigatório').max(40),
  cnpj_pag: z.string().optional().or(z.literal('')),
  uf_pag: z.string().optional().or(z.literal('')),
  t_band: z.string().max(2).optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  // UFPag só é válido acompanhado de CNPJPag — o XSD trata os dois como um par.
  if (v.uf_pag && !v.cnpj_pag) {
    ctx.addIssue({code: 'custom', path: ['cnpj_pag'], message: 'UF do pagador exige o CNPJ do pagador'})
  }
  if (v.cnpj_pag && !validateCNPJ(v.cnpj_pag.replace(/\D/g, ''))) {
    ctx.addIssue({code: 'custom', path: ['cnpj_pag'], message: 'CNPJ inválido'})
  }
})

export type PaymentTerminalFormData = z.infer<typeof paymentTerminalSchema>
