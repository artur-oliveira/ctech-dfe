import {describe, expect, it} from 'vitest'
import {paymentTermSchema, previewInstallments} from '@/lib/schemas/payment-terms'

describe('paymentTermSchema', () => {
  const valid = {
    name: '30/60/90',
    payment_type: '15',
    ind_pag: '' as const,
    installments: 3,
    interval_days: 30,
    first_due_days: 30,
  }

  it('aceita uma condição completa', () => {
    expect(paymentTermSchema.safeParse(valid).success).toBe(true)
  })

  it('exige forma de pagamento', () => {
    expect(paymentTermSchema.safeParse({...valid, payment_type: ''}).success).toBe(false)
  })

  it('recusa zero parcelas', () => {
    expect(paymentTermSchema.safeParse({...valid, installments: 0}).success).toBe(false)
  })
})

describe('previewInstallments', () => {
  const issue = new Date('2026-01-10T12:00:00Z')

  it('fecha a soma com o total quando a divisão não é exata', () => {
    const out = previewInstallments({installments: 3, interval_days: 30, first_due_days: 30}, 100, issue)
    expect(out.map((i) => i.value)).toEqual(['33.33', '33.33', '33.34'])
    const sum = out.reduce((s, i) => s + Math.round(parseFloat(i.value) * 100), 0)
    expect(sum).toBe(10000)
  })

  it('escalona os vencimentos a partir da primeira parcela', () => {
    const out = previewInstallments({installments: 3, interval_days: 30, first_due_days: 30}, 100, issue)
    expect(out.map((i) => i.dueDate)).toEqual(['2026-02-09', '2026-03-11', '2026-04-10'])
  })

  it('numera as parcelas com três dígitos', () => {
    const out = previewInstallments({installments: 1, interval_days: 0, first_due_days: 0}, 50, issue)
    expect(out).toHaveLength(1)
    expect(out[0].number).toBe('001')
    expect(out[0].value).toBe('50.00')
  })
})
