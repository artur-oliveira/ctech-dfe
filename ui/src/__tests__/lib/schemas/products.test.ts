import {describe, it, expect} from 'vitest'
import {cfopConfigSchema, productSchema} from '@/lib/schemas/products'

const valueResale = productSchema.shape.value_resale

describe('cfopConfigSchema — uf_overrides e IBS/CBS opcional', () => {
  const base = {
    cfop: '5102', pis: '01', cofins: '01',
    ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
  }

  it('aceita IBS/CBS totalmente vazio', () => {
    expect(cfopConfigSchema.safeParse(base).success).toBe(true)
  })

  it('aceita uf_overrides com UF válida', () => {
    const result = cfopConfigSchema.safeParse({
      ...base,
      uf_overrides: [{ufs: ['SP', 'RJ'], overrides: {icms_aliq_override: '12.0000'}}],
    })
    expect(result.success).toBe(true)
  })

  it('rejeita uf_overrides sem nenhuma UF', () => {
    const result = cfopConfigSchema.safeParse({
      ...base,
      uf_overrides: [{ufs: [], overrides: {}}],
    })
    expect(result.success).toBe(false)
  })
})

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
