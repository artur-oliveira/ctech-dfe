import {describe, expect, it} from 'vitest'
import {
  AGRO_TP_GUIA_OPTIONS,
  CANA_DIA_OPTIONS,
  canaRefOptions,
  MAX_AGRO_RECEITUARIOS,
  MAX_CANA_DEDUCOES,
  MAX_CANA_DELIVERIES,
} from '@/lib/data/nfe_niche'

describe('CANA_DIA_OPTIONS', () => {
  it('cobre 1 a 31 sem zero à esquerda (padrão do XSD)', () => {
    expect(CANA_DIA_OPTIONS).toHaveLength(MAX_CANA_DELIVERIES)
    expect(CANA_DIA_OPTIONS[0].value).toBe('1')
    expect(CANA_DIA_OPTIONS[30].value).toBe('31')
    for (const opt of CANA_DIA_OPTIONS) {
      expect(opt.value).toMatch(/^([1-9]|[12]\d|3[01])$/)
    }
  })
})

describe('canaRefOptions', () => {
  it('gera 13 meses no formato MM/AAAA, do atual para trás', () => {
    const opts = canaRefOptions(new Date(2026, 8, 15)) // setembro/2026
    expect(opts).toHaveLength(13)
    expect(opts[0].value).toBe('09/2026')
    expect(opts[0].label).toBe('Setembro/2026')
    expect(opts[12].value).toBe('09/2025')
    for (const opt of opts) {
      expect(opt.value).toMatch(/^(0[1-9]|1[0-2])\/2\d{3}$/)
    }
  })

  it('atravessa a virada do ano', () => {
    const opts = canaRefOptions(new Date(2026, 0, 5)) // janeiro/2026
    expect(opts[0].value).toBe('01/2026')
    expect(opts[1].value).toBe('12/2025')
  })
})

describe('AGRO_TP_GUIA_OPTIONS', () => {
  it('cobre exatamente os sete tipos enumerados no XSD', () => {
    expect(AGRO_TP_GUIA_OPTIONS.map((o) => o.value)).toEqual(['1', '2', '3', '4', '5', '6', '7'])
  })
})

describe('limites do leiaute', () => {
  it('espelha os maxOccurs do XSD', () => {
    expect(MAX_CANA_DELIVERIES).toBe(31)
    expect(MAX_CANA_DEDUCOES).toBe(10)
    expect(MAX_AGRO_RECEITUARIOS).toBe(20)
  })
})
