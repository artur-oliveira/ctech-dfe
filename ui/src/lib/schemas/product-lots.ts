/**
 * Lote de produção (NF-e prod/rastro). Espelha ProductLotBody
 * (api/internal/api/v1/dto.go). O lote é do produto e reaparece em várias notas
 * até acabar: na emissão o item só aponta qual lote saiu, e a quantidade é
 * rateada da quantidade vendida.
 */
import {z} from 'zod'

export const productLotSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  product_id: z.string().min(1, 'Escolha o produto do lote'),
  n_lote: z.string().min(1, 'Número do lote obrigatório').max(20),
  q_lote: z.string().min(1, 'Quantidade produzida obrigatória'),
  d_fab: z.string().min(1, 'Data de fabricação obrigatória'),
  d_val: z.string().min(1, 'Data de validade obrigatória'),
  c_agreg: z.string().max(20).optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  // Mesma regra do backend: lote que vence antes de ser fabricado é digitação errada.
  if (v.d_fab && v.d_val && v.d_val < v.d_fab) {
    ctx.addIssue({code: 'custom', path: ['d_val'], message: 'A validade não pode ser anterior à fabricação'})
  }
})

export type ProductLotFormData = z.infer<typeof productLotSchema>
