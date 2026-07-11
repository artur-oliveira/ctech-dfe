import {describe, it, expect} from 'vitest'
import {productSchema} from '@/lib/schemas/products'

const valueResale = productSchema.shape.value_resale

describe('productSchema.value_resale', () => {
  it('aceita vazio (opcional)', () => {
    expect(valueResale.safeParse('').success).toBe(true)
    expect(valueResale.safeParse(undefined).success).toBe(true)
  })

  it('aceita valor monetário válido', () => {
    expect(valueResale.safeParse('99.90').success).toBe(true)
    expect(valueResale.safeParse('8').success).toBe(true)
  })

  it('rejeita valor inválido', () => {
    expect(valueResale.safeParse('abc').success).toBe(false)
    expect(valueResale.safeParse('1.234567').success).toBe(false)
  })
})
