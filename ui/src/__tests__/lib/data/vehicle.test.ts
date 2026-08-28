import {describe, expect, it} from 'vitest'
import {
  VEIC_COR_DENATRAN_OPTIONS,
  VEIC_ESP_VEIC_OPTIONS,
  VEIC_TP_PINT_OPTIONS,
  VEIC_TP_VEIC_OPTIONS,
  vehicleYearOptions,
} from '@/lib/data/vehicle'

describe('tabelas RENAVAM de veicProd', () => {
  it('tipo de veículo cobre 01 a 26, sempre com dois dígitos', () => {
    expect(VEIC_TP_VEIC_OPTIONS).toHaveLength(26)
    expect(VEIC_TP_VEIC_OPTIONS[0].value).toBe('01')
    expect(VEIC_TP_VEIC_OPTIONS[25].value).toBe('26')
    for (const opt of VEIC_TP_VEIC_OPTIONS) {
      expect(opt.value).toMatch(/^\d{2}$/)
    }
  })

  it('espécie cobre 1 a 7 com um dígito — o XSD aceita só um', () => {
    expect(VEIC_ESP_VEIC_OPTIONS.map((o) => o.value)).toEqual(['1', '2', '3', '4', '5', '6', '7'])
  })

  it('cor DENATRAN cobre as 16 cores da tabela', () => {
    expect(VEIC_COR_DENATRAN_OPTIONS).toHaveLength(16)
    expect(VEIC_COR_DENATRAN_OPTIONS[15].value).toBe('16')
  })

  it('tipo de pintura é um caractere só', () => {
    for (const opt of VEIC_TP_PINT_OPTIONS) {
      expect(opt.value).toHaveLength(1)
    }
  })
})

describe('vehicleYearOptions', () => {
  it('vai do ano seguinte até cinco anos atrás, do mais novo para o mais velho', () => {
    const opts = vehicleYearOptions(new Date(2026, 5, 1))
    expect(opts.map((o) => o.value)).toEqual(
      ['2027', '2026', '2025', '2024', '2023', '2022', '2021'],
    )
  })
})
