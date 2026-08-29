import {describe, expect, it} from 'vitest'
import {CFOP_SUFFIXES, cfopSuffixOptions, getAllCfopOptions} from '@/lib/data/cfop'
import {UF_IBGE_OPTIONS} from '@/lib/data/cities'
import {LC116_SERVICE_OPTIONS} from '@/lib/data/nfse_trib_nacional'

describe('cfopSuffixOptions', () => {
  const options = cfopSuffixOptions()

  it('lista só sufixos de 3 dígitos, sem repetir', () => {
    expect(options.length).toBeGreaterThan(50)
    expect(options.every((o) => /^\d{3}$/.test(o.value))).toBe(true)
    expect(new Set(options.map((o) => o.value)).size).toBe(options.length)
  })

  it('traz a natureza de venda (102) com descrição', () => {
    const venda = options.find((o) => o.value === '102')
    expect(venda?.label).toMatch(/102 - .+/)
  })

  it('CFOP_SUFFIXES cobre exatamente as opções e rejeita um sufixo inventado', () => {
    expect(CFOP_SUFFIXES.size).toBe(options.length)
    expect(CFOP_SUFFIXES.has('102')).toBe(true)
    expect(CFOP_SUFFIXES.has('999')).toBe(false)
  })

  it('todo sufixo vem de um CFOP de saída existente', () => {
    const outgoing = new Set(getAllCfopOptions().map((o) => o.value).filter((c) => '567'.includes(c[0])))
    for (const {value} of options) {
      expect([...outgoing].some((c) => c.slice(1) === value)).toBe(true)
    }
  })
})

describe('UF_IBGE_OPTIONS', () => {
  it('tem as 27 unidades federativas, com código de 2 dígitos', () => {
    expect(UF_IBGE_OPTIONS).toHaveLength(27)
    expect(UF_IBGE_OPTIONS.every((o) => /^\d{2}$/.test(o.value))).toBe(true)
  })

  it('mapeia São Paulo para 35', () => {
    expect(UF_IBGE_OPTIONS.find((o) => o.value === '35')?.label).toBe('SP (35)')
  })
})

describe('LC116_SERVICE_OPTIONS', () => {
  it('usa o formato NN.NN do cListServ, sem repetir', () => {
    expect(LC116_SERVICE_OPTIONS.length).toBeGreaterThan(100)
    expect(LC116_SERVICE_OPTIONS.every((o) => /^\d{2}\.\d{2}$/.test(o.value))).toBe(true)
    expect(new Set(LC116_SERVICE_OPTIONS.map((o) => o.value)).size).toBe(LC116_SERVICE_OPTIONS.length)
  })

  it('traz análise e desenvolvimento de sistemas como 01.01', () => {
    expect(LC116_SERVICE_OPTIONS[0].value).toBe('01.01')
  })
})
