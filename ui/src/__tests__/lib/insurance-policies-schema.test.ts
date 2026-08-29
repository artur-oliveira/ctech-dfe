import {describe, expect, it} from 'vitest'
import {insurancePolicySchema} from '@/lib/schemas/insurance-policies'

const base = {name: 'Apólice frota', resp_seg: '1' as const, cnpj: '', cpf: '', x_seg: '', cnpj_seg: '', n_apol: ''}

describe('insurancePolicySchema', () => {
  it('aceita apólice do emitente sem documento nem seguradora', () => {
    expect(insurancePolicySchema.safeParse(base).success).toBe(true)
  })

  it('exige documento quando o responsável é o contratante', () => {
    const r = insurancePolicySchema.safeParse({...base, resp_seg: '2'})
    expect(r.success).toBe(false)
    expect(r.error?.issues.some((i) => i.path[0] === 'cnpj')).toBe(true)
  })

  it('recusa CNPJ e CPF juntos — infResp é um choice no XSD', () => {
    const r = insurancePolicySchema.safeParse({...base, cnpj: '11222333000181', cpf: '52998224725'})
    expect(r.success).toBe(false)
  })

  it('recusa meia seguradora: nome sem CNPJ', () => {
    const r = insurancePolicySchema.safeParse({...base, x_seg: 'Seguradora X'})
    expect(r.success).toBe(false)
    expect(r.error?.issues.some((i) => i.path[0] === 'cnpj_seg')).toBe(true)
  })

  it('aceita seguradora completa', () => {
    expect(insurancePolicySchema.safeParse({...base, x_seg: 'Seguradora X', cnpj_seg: '11222333000181'}).success).toBe(true)
  })
})
