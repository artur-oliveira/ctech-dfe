/**
 * Condição de pagamento — "30/60/90", "à vista", "boleto 28 dias".
 * Espelha PaymentTermBody (api/internal/api/v1/dto.go).
 */
import {z} from 'zod'

export const paymentTermSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  payment_type: z.string().min(1, 'Forma de pagamento obrigatória'),
  ind_pag: z.enum(['0', '1']).optional().or(z.literal('')),
  installments: z.number().int().min(1, 'Mínimo 1 parcela').max(120),
  interval_days: z.number().int().min(0).max(365),
  first_due_days: z.number().int().min(0).max(365),
})

export type PaymentTermFormData = z.infer<typeof paymentTermSchema>

export interface PreviewInstallment {
  number: string
  dueDate: string
  value: string
}

/**
 * Pré-visualização das parcelas geradas — o mesmo cálculo de
 * nfes.ExpandPaymentTerm, inclusive a regra de a última parcela absorver o
 * resíduo. Aqui é só para o usuário ver antes de salvar; o valor que vai para o
 * XML é sempre o que o backend calcula.
 */
export function previewInstallments(
  term: Pick<PaymentTermFormData, 'installments' | 'interval_days' | 'first_due_days'>,
  total: number,
  issueDate: Date,
): PreviewInstallment[] {
  const count = Math.max(1, term.installments)
  const cents = Math.round(total * 100)
  // Math.round espelha o RoundBank do backend nos casos comuns; a última
  // parcela absorve qualquer diferença, então divergir no meio é inofensivo.
  const base = Math.round(cents / count)

  const out: PreviewInstallment[] = []
  let accumulated = 0
  for (let i = 0; i < count; i++) {
    const value = i === count - 1 ? cents - accumulated : base
    accumulated += value

    const due = new Date(issueDate)
    due.setDate(due.getDate() + term.first_due_days + i * term.interval_days)

    out.push({
      number: String(i + 1).padStart(3, '0'),
      dueDate: due.toISOString().slice(0, 10),
      value: (value / 100).toFixed(2),
    })
  }
  return out
}
