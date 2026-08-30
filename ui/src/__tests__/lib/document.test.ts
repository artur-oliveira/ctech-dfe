import { describe, it, expect } from 'vitest'
import { formatCpfCnpj, unformatCpfCnpj, docLabel, orgTaxId, orgIsPJ, personTaxId } from '@/lib/utils/document'

describe('unformatCpfCnpj', () => {
  it('remove prefixo CPF_', () => {
    expect(unformatCpfCnpj('CPF_52998224725')).toBe('52998224725')
  })

  it('remove prefixo CNPJ_', () => {
    expect(unformatCpfCnpj('CNPJ_11222333000181')).toBe('11222333000181')
  })

  it('remove formatação além do prefixo', () => {
    expect(unformatCpfCnpj('CPF_529.982.247-25')).toBe('52998224725')
    expect(unformatCpfCnpj('CNPJ_11.222.333/0001-81')).toBe('11222333000181')
  })

  it('converte para maiúsculas', () => {
    expect(unformatCpfCnpj('CNPJ_12abc34501de35')).toBe('12ABC34501DE35')
  })

  it('funciona sem prefixo', () => {
    expect(unformatCpfCnpj('52998224725')).toBe('52998224725')
  })
})

describe('formatCpfCnpj', () => {
  it('formata CPF a partir de pk com prefixo', () => {
    expect(formatCpfCnpj('CPF_52998224725')).toBe('529.982.247-25')
  })

  it('formata CNPJ a partir de pk com prefixo', () => {
    expect(formatCpfCnpj('CNPJ_11222333000181')).toBe('11.222.333/0001-81')
  })

  it('formata CNPJ alfanumérico', () => {
    expect(formatCpfCnpj('CNPJ_12ABC34501DE35')).toBe('12.ABC.345/01DE-35')
  })
})

describe('docLabel', () => {
  it('retorna CPF para pk com prefixo CPF_', () => {
    expect(docLabel('CPF_52998224725')).toBe('CPF')
  })

  it('retorna CNPJ para pk com prefixo CNPJ_', () => {
    expect(docLabel('CNPJ_11222333000181')).toBe('CNPJ')
  })

  it('retorna CNPJ para qualquer pk sem prefixo CPF_', () => {
    expect(docLabel('11222333000181')).toBe('CNPJ')
  })
})

// The regression the company re-key exists for. These helpers are document
// formatters, and a company id is not a document: it must come back untouched
// rather than silently reshaped into a value the API refuses.
describe('a key that is not a document', () => {
  const companyId = '0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70'

  it('unformatCpfCnpj leaves a company id exactly as it is', () => {
    // Stripping its hyphens and uppercasing its hex produced
    // 0199F3A18C427C319D5E6A2B4C8E1F70, which the API's IsCompanyKey rejects —
    // so every screen in the product would have answered "organização inválida".
    expect(unformatCpfCnpj(companyId)).toBe(companyId)
  })

  it('formatCpfCnpj leaves a company id exactly as it is', () => {
    expect(formatCpfCnpj(companyId)).toBe(companyId)
  })

  it('docLabel calls a company id neither CPF nor CNPJ', () => {
    // Labelling it "CNPJ" would print a wrong word next to a value that is not
    // one. Empty is the honest answer; the caller decides whether to render it.
    expect(docLabel(companyId)).toBe('')
  })

  it('still recognizes uppercase hex as a company id', () => {
    // The canonical form is lowercase, but a value that round-tripped through
    // an uppercasing helper must not then be treated as a document.
    expect(unformatCpfCnpj('0199F3A1-8C42-7C31-9D5E-6A2B4C8E1F70'))
      .toBe('0199F3A1-8C42-7C31-9D5E-6A2B4C8E1F70')
  })

  it('does not mistake a real document for a key', () => {
    expect(unformatCpfCnpj('CNPJ_11222333000181')).toBe('11222333000181')
    expect(docLabel('CPF_52998224725')).toBe('CPF')
    expect(docLabel('CNPJ_11222333000181')).toBe('CNPJ')
  })
})

// Mirrors services.IssuerDoc in the API. The two must agree on both eras, or
// the screen shows one document and the XML carries another.
describe('orgTaxId / orgIsPJ', () => {
  const companyId = '0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70'

  it('reads the record after the migration', () => {
    const org = {pk: companyId, tax_id: '11222333000181', tax_id_kind: 'cnpj' as const}
    expect(orgTaxId(org)).toBe('11222333000181')
    expect(orgIsPJ(org)).toBe(true)
  })

  it('reads cpf_or_cnpj while compatibility records are still present', () => {
    const org = {pk: companyId, cpf_or_cnpj: '11.222.333/0001-81'}
    expect(orgTaxId(org)).toBe('11222333000181')
    expect(orgIsPJ(org)).toBe(true)
  })

  it('falls back to the legacy key before it', () => {
    expect(orgTaxId({pk: 'CNPJ_11222333000181'})).toBe('11222333000181')
    expect(orgIsPJ({pk: 'CNPJ_11222333000181'})).toBe(true)
    expect(orgTaxId({pk: 'CPF_52998224725'})).toBe('52998224725')
    expect(orgIsPJ({pk: 'CPF_52998224725'})).toBe(false)
  })

  // The whole point: a company id with no record behind it yields nothing, not
  // the id. Returning the id is what put a UUID where a CNPJ belonged.
  it('never returns the company id as a document', () => {
    expect(orgTaxId({pk: companyId})).toBe('')
  })

  it('keeps a natural person natural under a company id', () => {
    const org = {pk: companyId, tax_id: '52998224725', tax_id_kind: 'cpf' as const}
    expect(orgIsPJ(org)).toBe(false)
  })
})

describe('personTaxId', () => {
  it('uses the explicit document when the item is the organization itself', () => {
    expect(personTaxId({
      sk: '01a04fc3-b6f7-7bb9-8cfe-6e19b66019f6',
      cpf_or_cnpj: '11222333000181',
    })).toBe('11222333000181')
  })

  it('keeps using the sort key for regular people', () => {
    expect(personTaxId({sk: 'CPF_52998224725'})).toBe('52998224725')
  })
})
