import {describe, expect, it} from 'vitest'
import {ALL_CNAES} from '@/lib/data/cnae'

describe('tabela CNAE', () => {
  it('contém as 1.357 subclasses únicas do CSV', () => {
    expect(ALL_CNAES).toHaveLength(1357)
    expect(new Set(ALL_CNAES.map(({code}) => code)).size).toBe(1357)
  })

  it('normaliza códigos de seis dígitos com zero à esquerda', () => {
    expect(ALL_CNAES[0]).toEqual({
      code: '0111301',
      description: 'Cultivo de Arroz',
    })
  })

  it('gera apenas códigos aceitos pelo contrato de sete dígitos', () => {
    expect(ALL_CNAES.every(({code}) => /^\d{7}$/.test(code))).toBe(true)
  })
})
