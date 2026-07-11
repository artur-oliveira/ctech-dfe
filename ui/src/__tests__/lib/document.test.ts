import { describe, it, expect } from 'vitest'
import { formatCpfCnpj, unformatCpfCnpj, docLabel } from '@/lib/utils/document'

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
