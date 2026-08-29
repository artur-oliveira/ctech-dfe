import {describe, expect, it} from 'vitest'
import {productLotSchema} from '@/lib/schemas/product-lots'

const base = {
  name: 'Lote 2026/001', product_id: 'PRODUCT_1', n_lote: 'ABC1234',
  q_lote: '100.000', d_fab: '2026-01-10', d_val: '2027-01-10', c_agreg: '',
}

describe('productLotSchema', () => {
  it('aceita o lote completo', () => {
    expect(productLotSchema.safeParse(base).success).toBe(true)
  })

  it('exige o produto do lote', () => {
    const r = productLotSchema.safeParse({...base, product_id: ''})
    expect(r.success).toBe(false)
    expect(r.error?.issues.some((i) => i.path[0] === 'product_id')).toBe(true)
  })

  it('recusa validade anterior à fabricação', () => {
    const r = productLotSchema.safeParse({...base, d_val: '2025-12-31'})
    expect(r.success).toBe(false)
    expect(r.error?.issues.some((i) => i.path[0] === 'd_val')).toBe(true)
  })

  it('aceita validade igual à fabricação', () => {
    expect(productLotSchema.safeParse({...base, d_val: base.d_fab}).success).toBe(true)
  })
})
