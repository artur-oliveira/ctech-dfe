import {z} from 'zod'

// Valores monetários e alíquotas são string decimal com ponto — mesmo contrato
// do backend (api/internal/api/v1/dto.go ServiceBody) e do XML assinado.
const money = z.string().regex(/^\d+(\.\d{1,2})?$/, 'Use ponto decimal, ex: 1000.00')
const percent = z.string().regex(/^\d{1,3}(\.\d{1,4})?$/, 'Alíquota inválida')

const serviceIssSchema = z.object({
  // 1 operação tributável | 2 imunidade | 3 exportação de serviço | 4 não incidência
  trib_issqn: z.enum(['1', '2', '3', '4']),
  // Só a operação tributável (1) tem alíquota; nos demais casos o formulário
  // esconde o campo e envia 0 (ServiceForm.toApiPayload).
  tax_rate: percent.optional().or(z.literal('')),
  // 1 não retido | 2 retido pelo tomador | 3 retido pelo intermediário
  tp_ret_issqn: z.enum(['1', '2', '3']).optional().or(z.literal('')),
  tp_imunidade: z.enum(['0', '1', '2', '3', '4', '5']).optional().or(z.literal('')),
  c_pais_resultado: z.string().length(2, 'Código de país tem 2 letras').optional().or(z.literal('')),
}).superRefine((v, ctx) => {
  if (v.trib_issqn === '1' && !v.tax_rate) {
    ctx.addIssue({code: z.ZodIssueCode.custom, path: ['tax_rate'], message: 'Alíquota obrigatória'})
  }
  if (v.trib_issqn !== '2' && v.tp_imunidade) {
    ctx.addIssue({
      code: z.ZodIssueCode.custom,
      path: ['tp_imunidade'],
      message: 'Só se aplica quando trib_issqn é imunidade (2)',
    })
  }
})

const serviceFederalSchema = z.object({
  cst_pis_cofins: z.string().length(2, 'CST tem 2 dígitos').optional().or(z.literal('')),
  aliq_pis: percent.optional().or(z.literal('')),
  aliq_cofins: percent.optional().or(z.literal('')),
  tp_ret_pis_cofins: z.enum(['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']).optional().or(z.literal('')),
  v_ret_cp: money.optional().or(z.literal('')),
  v_ret_irrf: money.optional().or(z.literal('')),
  v_ret_csll: money.optional().or(z.literal('')),
})

const serviceIbsCbsSchema = z.object({
  c_ind_op: z.string().optional().or(z.literal('')),
  cst: z.string().length(3, 'CST tem 3 dígitos').optional().or(z.literal('')),
  c_class_trib: z.string().max(6).optional().or(z.literal('')),
  // 0 destinatário é o próprio tomador | 1 destinatário diferente do tomador
  ind_dest: z.enum(['0', '1']).optional().or(z.literal('')),
  tp_oper: z.enum(['1', '2', '3', '4', '5']).optional().or(z.literal('')),
})

const serviceTotTribSchema = z.object({
  // Valor fixo — Decreto 8.264/2014 veda estimar tributos na NFS-e.
  ind_tot_trib: z.literal('0'),
  p_tot_trib_sn: percent.optional().or(z.literal('')),
})

export const serviceSchema = z.object({
  code: z.string().min(1, 'Código obrigatório').max(60),
  description: z.string().min(2, 'Descrição obrigatória').max(2000),
  trib_nacional_code: z.string().regex(/^\d{6}$/, 'Código de tributação nacional tem 6 dígitos'),
  trib_municipal_code: z.string().max(20).optional().or(z.literal('')),
  nbs_code: z.string().regex(/^\d{9}$/, 'NBS tem 9 dígitos').optional().or(z.literal('')),
  cnae: z.string().regex(/^\d{7}$/, 'CNAE tem 7 dígitos').optional().or(z.literal('')),
  unit: z.string().min(1, 'Unidade obrigatória'),
  value: money,
  iss: serviceIssSchema,
  federal: serviceFederalSchema.optional(),
  ibs_cbs: serviceIbsCbsSchema.optional(),
  tot_trib: serviceTotTribSchema.optional(),
})

export type ServiceFormData = z.infer<typeof serviceSchema>
