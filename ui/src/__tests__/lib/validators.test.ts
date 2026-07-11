import { describe, it, expect } from 'vitest'
import { validateCPF, validateCNPJ } from '@/lib/utils/validators'

describe('validateCPF', () => {
  it('aceita CPF válido', () => {
    expect(validateCPF('529.982.247-25')).toBe(true)
    expect(validateCPF('52998224725')).toBe(true)
  })

  it('rejeita CPF com dígito verificador errado', () => {
    expect(validateCPF('529.982.247-26')).toBe(false)
    expect(validateCPF('52998224726')).toBe(false)
  })

  it('rejeita sequência de dígitos iguais', () => {
    expect(validateCPF('000.000.000-00')).toBe(false)
    expect(validateCPF('111.111.111-11')).toBe(false)
    expect(validateCPF('99999999999')).toBe(false)
  })

  it('rejeita CPF com tamanho incorreto', () => {
    expect(validateCPF('123.456.789')).toBe(false)
    expect(validateCPF('1234567890123')).toBe(false)
    expect(validateCPF('')).toBe(false)
  })

  it('ignora formatação ao validar', () => {
    expect(validateCPF('529.982.247-25')).toBe(true)
    expect(validateCPF('529 982 247 25')).toBe(true)
  })
})

describe('validateCNPJ', () => {
  it('aceita CNPJ numérico válido', () => {
    expect(validateCNPJ('11.222.333/0001-81')).toBe(true)
    expect(validateCNPJ('11222333000181')).toBe(true)
  })

  it('aceita CNPJ alfanumérico válido (IN RFB 2229/2024)', () => {
    // 12ABC34501DE: d1=4 (326%11=7 → 11-7=4), d2=5 (314%11=6 → 11-6=5)
    expect(validateCNPJ('12ABC34501DE45')).toBe(true)
  })

  it('rejeita CNPJ com dígito verificador errado', () => {
    expect(validateCNPJ('11.222.333/0001-82')).toBe(false)
    expect(validateCNPJ('11222333000182')).toBe(false)
  })

  it('rejeita todos os caracteres iguais', () => {
    expect(validateCNPJ('00000000000000')).toBe(false)
    expect(validateCNPJ('11111111111111')).toBe(false)
  })

  it('rejeita CNPJ com tamanho incorreto', () => {
    expect(validateCNPJ('1122233300018')).toBe(false)
    expect(validateCNPJ('112223330001812')).toBe(false)
    expect(validateCNPJ('')).toBe(false)
  })

  it('rejeita CNPJ com dígitos verificadores não numéricos', () => {
    expect(validateCNPJ('11222333000AAB')).toBe(false)
  })
})
