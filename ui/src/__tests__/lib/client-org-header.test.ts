import {describe, expect, it} from 'vitest'
import {orgHeaderValue} from '@/lib/api/client'

// The one that breaks everything, so it gets its own test rather than being
// covered incidentally by the formatter's.
//
// This value goes on every request as Dfe-Organization-Pk. The API validates it
// with IsCompanyKey — lowercase hex, hyphens required — so a browser that
// reshapes it here makes the whole product answer "organização inválida", with
// the cause three layers from the symptom.
describe('the organization header', () => {
  it('sends a company id verbatim', () => {
    const id = '0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70'
    expect(orgHeaderValue({pk: id})).toBe(id)
  })

  // The legacy shape keeps travelling as a bare document: that is what the
  // server's ParseOrgPK re-prefixes today, and the old partitions are the
  // migration's rollback.
  it('still sends a legacy key as a bare document', () => {
    expect(orgHeaderValue({pk: 'CNPJ_11222333000181'})).toBe('11222333000181')
    expect(orgHeaderValue({pk: 'CPF_52998224725'})).toBe('52998224725')
  })

  it('sends nothing for an organization without a key', () => {
    expect(orgHeaderValue({pk: ''})).toBe('')
  })
})
