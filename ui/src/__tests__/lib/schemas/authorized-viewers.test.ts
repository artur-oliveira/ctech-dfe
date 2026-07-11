import {describe, expect, it} from 'vitest'
import {authorizedViewerSchema, hasDuplicateViewer} from '@/lib/schemas/authorized-viewers'

describe('authorizedViewerSchema', () => {
  it('rejects invalid CPF', () => {
    expect(authorizedViewerSchema.safeParse({cpf_or_cnpj: '11111111111', name: 'X'}).success).toBe(false)
  })

  it('accepts a valid CPF', () => {
    expect(authorizedViewerSchema.safeParse({cpf_or_cnpj: '05213732399', name: 'Contador'}).success).toBe(true)
  })

  it('accepts a valid CNPJ', () => {
    expect(authorizedViewerSchema.safeParse({cpf_or_cnpj: '11222333000181', name: 'Escritório Contábil'}).success).toBe(true)
  })

  it('rejects name shorter than 2 characters', () => {
    expect(authorizedViewerSchema.safeParse({cpf_or_cnpj: '05213732399', name: 'X'}).success).toBe(false)
  })
})

describe('hasDuplicateViewer', () => {
  it('detects duplicate ignoring formatting', () => {
    const existing = [{cpf_cnpj: '11122233344'}]
    expect(hasDuplicateViewer(existing, '111.222.333-44')).toBe(true)
    expect(hasDuplicateViewer(existing, '99988877766')).toBe(false)
  })

  it('returns false for empty list', () => {
    expect(hasDuplicateViewer([], '11122233344')).toBe(false)
  })
})
