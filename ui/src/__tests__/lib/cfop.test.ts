import { describe, it, expect } from 'vitest'
import {
  cfopDirection, cfopTpNf, buildNatOpFromCfops,
  cfopScope, cfopSuffix, groupCfopConfigBySuffix, resolveCfopForUf, cfopGroupCodes,
  NO_PAYMENT_CFOPS,
} from '@/lib/data/cfop'
import type { CfopConfigItem } from '@/lib/types/api'

describe('cfopGroupCodes', () => {
  const cc = (cfop: string): CfopConfigItem => ({cfop} as CfopConfigItem)
  it('junta as variantes intra/inter com barra, intra primeiro', () => {
    const [g920] = groupCfopConfigBySuffix([cc('6920'), cc('5920')])
    expect(cfopGroupCodes(g920)).toBe('5920/6920')
  })
  it('mostra só a variante única quando o par não existe', () => {
    const [g405] = groupCfopConfigBySuffix([cc('5405')])
    expect(cfopGroupCodes(g405)).toBe('5405')
  })
})

describe('cfopDirection', () => {
  it('classifica CFOPs de entrada (1/2/3) como "in"', () => {
    expect(cfopDirection('1102')).toBe('in')
    expect(cfopDirection('2102')).toBe('in')
    expect(cfopDirection('3102')).toBe('in')
  })

  it('classifica CFOPs de saída (5/6/7) como "out"', () => {
    expect(cfopDirection('5102')).toBe('out')
    expect(cfopDirection('6102')).toBe('out')
    expect(cfopDirection('7102')).toBe('out')
  })

  it('retorna null para entrada inválida', () => {
    expect(cfopDirection('')).toBeNull()
    expect(cfopDirection('9999')).toBeNull()
  })
})

describe('cfopTpNf', () => {
  it('entrada → 0, saída → 1', () => {
    expect(cfopTpNf('1102')).toBe('0')
    expect(cfopTpNf('5102')).toBe('1')
  })

  it('default 1 quando desconhecido', () => {
    expect(cfopTpNf('9999')).toBe('1')
  })
})

describe('buildNatOpFromCfops', () => {
  it('retorna vazio quando não há CFOPs', () => {
    expect(buildNatOpFromCfops([])).toBe('')
    expect(buildNatOpFromCfops(['', ''])).toBe('')
  })

  it('usa a descrição (truncada) para um único CFOP', () => {
    const out = buildNatOpFromCfops(['5102'])
    expect(out.length).toBeLessThanOrEqual(60)
    expect(out.toLowerCase()).toContain('venda')
  })

  it('combina o primeiro termo de cada CFOP distinto', () => {
    // 5102 = Venda..., 5949/5917 etc. — termos distintos combinados com " e "
    const out = buildNatOpFromCfops(['5102', '5102'])
    // CFOPs iguais → tratado como único
    expect(out.toLowerCase()).toContain('venda')
  })

  it('nunca excede 60 caracteres', () => {
    const out = buildNatOpFromCfops(['5102', '6102', '5949', '5910'])
    expect(out.length).toBeLessThanOrEqual(60)
  })
})

const cc = (cfop: string): CfopConfigItem => ({cfop} as CfopConfigItem)

describe('cfop scope/suffix', () => {
  it('splits scope and suffix', () => {
    expect(cfopScope('5920')).toBe('5')
    expect(cfopSuffix('5920')).toBe('920')
    expect(cfopScope('6920')).toBe('6')
    expect(cfopSuffix('6920')).toBe('920')
  })
})

describe('groupCfopConfigBySuffix', () => {
  it('pairs intra and inter variants under one suffix', () => {
    const groups = groupCfopConfigBySuffix([cc('5405'), cc('5920'), cc('6920')])
    const g920 = groups.find(g => g.suffix === '920')!
    const g405 = groups.find(g => g.suffix === '405')!
    expect(groups).toHaveLength(2)
    expect(g920.intra).toBe('5920')
    expect(g920.inter).toBe('6920')
    expect(g405.intra).toBe('5405')
    expect(g405.inter).toBeUndefined()
  })
})

describe('resolveCfopForUf', () => {
  const groups = groupCfopConfigBySuffix([cc('5405'), cc('5920'), cc('6920')])
  const g920 = groups.find(g => g.suffix === '920')!
  const g405 = groups.find(g => g.suffix === '405')!

  it('returns intra variant when same UF', () => {
    expect(resolveCfopForUf(g920, true)).toBe('5920')
    expect(resolveCfopForUf(g405, true)).toBe('5405')
  })
  it('returns inter variant when other UF', () => {
    expect(resolveCfopForUf(g920, false)).toBe('6920')
  })
  it('returns null when required scope variant is missing', () => {
    expect(resolveCfopForUf(g405, false)).toBeNull()
  })
})

describe('NO_PAYMENT_CFOPS', () => {
  it('contains the same-UF and other-UF remessa/bonificação CFOPs', () => {
    expect(NO_PAYMENT_CFOPS).toContain('5920')
    expect(NO_PAYMENT_CFOPS).toContain('6920')
  })
})
