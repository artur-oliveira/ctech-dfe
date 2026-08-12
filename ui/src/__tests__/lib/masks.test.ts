import { describe, it, expect } from 'vitest'
import { maskCpf, maskCnpj, maskCpfCnpj, maskCep, maskPhone, maskAccessKey } from '@/lib/utils/masks'

describe('maskCpf', () => {
  it('formata CPF completo', () => {
    expect(maskCpf('52998224725')).toBe('529.982.247-25')
  })

  it('formata parcialmente durante digitação', () => {
    expect(maskCpf('529')).toBe('529')
    expect(maskCpf('5299')).toBe('529.9')
    expect(maskCpf('529982')).toBe('529.982')
    expect(maskCpf('52998224')).toBe('529.982.24')
  })

  it('ignora caracteres não numéricos', () => {
    expect(maskCpf('529.982.247-25')).toBe('529.982.247-25')
    expect(maskCpf('abc12345678')).toBe('123.456.78')
  })

  it('limita a 11 dígitos', () => {
    expect(maskCpf('529982247251234')).toBe('529.982.247-25')
  })
})

describe('maskCnpj', () => {
  it('formata CNPJ numérico completo', () => {
    expect(maskCnpj('11222333000181')).toBe('11.222.333/0001-81')
  })

  it('formata CNPJ alfanumérico', () => {
    expect(maskCnpj('12ABC34501DE35')).toBe('12.ABC.345/01DE-35')
  })

  it('formata parcialmente durante digitação', () => {
    expect(maskCnpj('11')).toBe('11')
    expect(maskCnpj('112')).toBe('11.2')
    expect(maskCnpj('11222333')).toBe('11.222.333')
    expect(maskCnpj('112223330001')).toBe('11.222.333/0001')
  })

  it('limita a 14 caracteres', () => {
    expect(maskCnpj('112223330001812')).toBe('11.222.333/0001-81')
  })
})

describe('maskCpfCnpj', () => {
  it('aplica máscara de CPF para 11 dígitos', () => {
    expect(maskCpfCnpj('52998224725')).toBe('529.982.247-25')
  })

  it('aplica máscara de CNPJ para 14 caracteres', () => {
    expect(maskCpfCnpj('11222333000181')).toBe('11.222.333/0001-81')
  })

  it('aplica máscara de CNPJ quando contém letras', () => {
    expect(maskCpfCnpj('12ABC34501DE35')).toBe('12.ABC.345/01DE-35')
  })

  it('aceita entrada já formatada', () => {
    expect(maskCpfCnpj('529.982.247-25')).toBe('529.982.247-25')
  })
})

describe('maskCep', () => {
  it('formata CEP completo', () => {
    expect(maskCep('01310100')).toBe('01310-100')
  })

  it('formata parcialmente durante digitação', () => {
    expect(maskCep('01310')).toBe('01310')
    expect(maskCep('013101')).toBe('01310-1')
  })

  it('ignora caracteres não numéricos', () => {
    expect(maskCep('01310-100')).toBe('01310-100')
  })
})

describe('maskPhone', () => {
  it('formata celular (11 dígitos)', () => {
    expect(maskPhone('11987654321')).toBe('(11) 98765-4321')
  })

  it('formata telefone fixo (10 dígitos)', () => {
    expect(maskPhone('1133334444')).toBe('(11) 3333-4444')
  })

  it('ignora caracteres não numéricos', () => {
    expect(maskPhone('(11) 98765-4321')).toBe('(11) 98765-4321')
  })

  it('formata parcialmente durante digitação', () => {
    // DDD só é aplicado a partir de 3+ dígitos (regex precisa de (\d{2})(\d))
    expect(maskPhone('11')).toBe('11')
    expect(maskPhone('119')).toBe('(11) 9')
    expect(maskPhone('119876')).toBe('(11) 9876')
  })
})

describe('maskAccessKey', () => {
  it('agrupa em blocos de 4 caracteres', () => {
    expect(maskAccessKey('35250512345678000195550010000000011000000011'))
      .toBe('3525 0512 3456 7800 0195 5500 1000 0000 0110 0000 0011')
  })

  it('formata parcialmente durante digitação', () => {
    expect(maskAccessKey('352505')).toBe('3525 05')
  })

  it('aceita CNPJ alfanumérico em maiúsculas, ignora minúsculas convertendo', () => {
    expect(maskAccessKey('3525051234ab5678000195550010000000011000000011'))
      .toBe('3525 0512 34AB 5678 0001 9555 0010 0000 0001 1000 0000')
  })

  it('ignora caracteres não alfanuméricos e limita a 44', () => {
    expect(maskAccessKey('3525-0512.3456/7800 0195550010000000011000000011XXXX'))
      .toBe('3525 0512 3456 7800 0195 5500 1000 0000 0110 0000 0011')
  })
})
