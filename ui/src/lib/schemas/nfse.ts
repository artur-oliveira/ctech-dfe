import {z} from 'zod'

const money = z.string().regex(/^\d+(\.\d{1,2})?$/, 'Use ponto decimal, ex: 1000.00')
const percent = z.string().regex(/^\d{1,3}(\.\d{1,4})?$/, 'Alíquota inválida')
// api/internal/validation/validators.go: "datebr" = ^\d{2}/\d{2}/\d{4}$
const dateBr = z.string().regex(/^\d{2}\/\d{2}\/\d{4}$/, 'Use DD/MM/AAAA')

/**
 * Eventos que o contribuinte pode oferecer no seletor da UI. Espelha
 * nfse.ContribuinteEvents (go-dfe/nfse/constants.go) MENOS 105102: esse é
 * gerado pelo fisco na substituição — api/internal/services/nfses/events.go
 * rejeita 105102 em POST /events com 400 pedindo POST /substitute.
 */
export const CONTRIBUINTE_EVENTS = [
  '101101', '101103', '202201', '203202', '204203', '202205', '203206', '204207', '205208',
] as const

export const EVENT_LABELS: Record<string, string> = {
  '101101': 'Cancelamento',
  '101103': 'Solicitação de análise fiscal de cancelamento',
  '105102': 'Cancelamento por substituição',
  '202201': 'Rejeição do prestador',
  '203202': 'Rejeição do tomador',
  '204203': 'Rejeição do intermediário',
  '202205': 'Confirmação do prestador',
  '203206': 'Confirmação do tomador',
  '204207': 'Confirmação do intermediário',
  '205208': 'Anulação de rejeição',
}

// go-dfe/nfse/constants.go EventsRequiringMotivo, menos 105102 (não é
// oferecido pela UI — ver CONTRIBUINTE_EVENTS acima).
const EVENTS_REQUIRING_REASON_CODE = new Set(['101101', '101103', '202205', '203206', '204207'])
// go-dfe/nfse/constants.go EventsRequiringXMotivo
const EVENTS_REQUIRING_REASON_DESCRIPTION = new Set(['101101', '101103'])

const nfseServiceItemSchema = z.object({
  service_id: z.string().min(1, 'Selecione um serviço do catálogo'),
  description: z.string().max(2000).optional().or(z.literal('')),
  value: money.optional().or(z.literal('')),
  tax_rate: percent.optional().or(z.literal('')),
  c_trib_mun: z.string().max(20).optional().or(z.literal('')),
})

export const nfseEmitSchema = z
  .object({
    tp_emit: z.enum(['1', '2', '3']),
    motivo_emis_ti: z.enum(['1', '2', '3', '4']).optional().or(z.literal('')),
    ch_nfse_rej: z.string().length(50, 'Chave de acesso tem 50 dígitos').regex(/^\d+$/).optional().or(z.literal('')),
    competence: dateBr,
    provider_person_id: z.string().optional().or(z.literal('')),
    customer_id: z.string().optional().or(z.literal('')),
    intermediary_id: z.string().optional().or(z.literal('')),
    service: nfseServiceItemSchema,
    substitutes_access_key: z.string().length(50).regex(/^\d+$/).optional().or(z.literal('')),
    substitutes_reason: z.string().max(2).optional().or(z.literal('')),
    additional_info: z.string().max(2000).optional().or(z.literal('')),
  })
  .superRefine((v, ctx) => {
    if ((v.tp_emit === '2' || v.tp_emit === '3') && !v.provider_person_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['provider_person_id'],
        message: 'Obrigatório quando a emissão não é pelo próprio prestador',
      })
    }
    if ((v.tp_emit === '2' || v.tp_emit === '3') && !v.motivo_emis_ti) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['motivo_emis_ti'],
        message: 'Obrigatório quando a emissão é por tomador ou intermediário',
      })
    }
    if (v.substitutes_access_key && !v.substitutes_reason) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['substitutes_reason'],
        message: 'Motivo obrigatório na substituição',
      })
    }
  })

export type NfseEmitFormData = z.infer<typeof nfseEmitSchema>

export const nfseEventSchema = z
  .object({
    event_type: z.enum(CONTRIBUINTE_EVENTS),
    sequence_number: z.number().int().min(1).max(999).optional(),
    reason_code: z.string().max(2).optional().or(z.literal('')),
    reason_description: z.string().max(255).optional().or(z.literal('')),
  })
  .superRefine((v, ctx) => {
    if (EVENTS_REQUIRING_REASON_CODE.has(v.event_type) && !v.reason_code) {
      ctx.addIssue({code: z.ZodIssueCode.custom, path: ['reason_code'], message: 'Código do motivo obrigatório'})
    }
    if (EVENTS_REQUIRING_REASON_DESCRIPTION.has(v.event_type) && !v.reason_description) {
      ctx.addIssue({code: z.ZodIssueCode.custom, path: ['reason_description'], message: 'Descrição do motivo obrigatória'})
    }
  })

export type NfseEventFormData = z.infer<typeof nfseEventSchema>

// POST /nfses/{id}/cancel tem corpo próprio (nfses.go), não NfseEventBody:
// reason_code e reason_description são sempre obrigatórios aqui.
export const nfseCancelSchema = z.object({
  reason_code: z.string().min(1, 'Código do motivo obrigatório').max(2),
  reason_description: z.string().min(1, 'Descrição do motivo obrigatória').max(255),
  sequence_number: z.number().int().min(1).max(999).optional(),
})

export type NfseCancelFormData = z.infer<typeof nfseCancelSchema>
