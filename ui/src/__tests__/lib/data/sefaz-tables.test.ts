import {describe, expect, it} from 'vitest'
import {ANP_CODE_SET, ANP_MONO_FUELS, anpMonoFuel, ANP_OPTIONS} from '@/lib/data/anp'
import {benefitOptionsForUf, CBENEF_UFS, isKnownBenefit, SEM_CBENEF, ufHasBenefitTable} from '@/lib/data/cbenef'
import {especieOptionsForTipo, isValidVehicleTypePair, VEHICLE_TYPE_PAIRS} from '@/lib/data/vehicle_type_pairs'

describe('tabela da ANP', () => {
  it('traz os 1031 códigos oficiais, todos com 9 dígitos', () => {
    expect(ANP_CODE_SET.size).toBe(1031)
    expect([...ANP_CODE_SET].every((c) => /^\d{9}$/.test(c))).toBe(true)
  })

  it('todo monofásico está na tabela geral e aparece antes no seletor', () => {
    expect(ANP_MONO_FUELS.every((f) => ANP_CODE_SET.has(f.code))).toBe(true)
    expect(ANP_OPTIONS).toHaveLength(1031)
    expect(ANP_OPTIONS[0].label).toContain(' - ')
  })

  it('devolve os dados publicados do GLP e nada para código sem monofasia', () => {
    const glp = anpMonoFuel('210203001')
    expect(glp?.description).toBe('GLP')
    expect(glp?.taxableUnit).toBe('KG')
    expect(glp?.adRemIcms).toBe('1.47')
    expect(anpMonoFuel('110203073')).toBeNull()
    expect(anpMonoFuel(null)).toBeNull()
  })
})

describe('tabela de benefício fiscal por UF', () => {
  it('cobre as UFs que publicaram tabela', () => {
    expect(CBENEF_UFS).toEqual(expect.arrayContaining(['RS', 'SP', 'RJ', 'PR', 'ES']))
    expect(ufHasBenefitTable('SP')).toBe(true)
    expect(ufHasBenefitTable('AC')).toBe(false)
  })

  it('oferece SEM CBENEF primeiro e só códigos da UF pedida', () => {
    const sp = benefitOptionsForUf('SP')
    expect(sp[0].value).toBe(SEM_CBENEF)
    expect(sp.slice(1).every((o) => o.value.startsWith('SP'))).toBe(true)
    expect(benefitOptionsForUf('AC')).toHaveLength(0)
  })

  it('filtra pelo CST quando a UF publica o par código × CST', () => {
    const todos = benefitOptionsForUf('RS')
    const doCst40 = benefitOptionsForUf('RS', '40')
    expect(doCst40.length).toBeGreaterThan(1)
    expect(doCst40.length).toBeLessThan(todos.length)
  })

  it('reconhece um código da UF e recusa o de outra', () => {
    expect(isKnownBenefit('SP', 'SP010010')).toBe(true)
    expect(isKnownBenefit('SP', SEM_CBENEF)).toBe(true)
    expect(isKnownBenefit('SP', 'RJ801001')).toBe(false)
  })
})

describe('pares de tipo e espécie de veículo', () => {
  it('não é o produto cartesiano dos dois selects', () => {
    const tipos = new Set(VEHICLE_TYPE_PAIRS.map((p) => p.tpVeic))
    const especies = new Set(VEHICLE_TYPE_PAIRS.map((p) => p.espVeic))
    expect(VEHICLE_TYPE_PAIRS.length).toBeLessThan(tipos.size * especies.size)
  })

  it('motoneta aceita passageiro e carga', () => {
    expect(especieOptionsForTipo('3').map((o) => o.value)).toEqual(expect.arrayContaining(['1', '2']))
    expect(especieOptionsForTipo(null)).toHaveLength(0)
  })

  it('recusa o par que a tabela oficial não publica', () => {
    expect(isValidVehicleTypePair('2', '1')).toBe(true)
    expect(isValidVehicleTypePair('2', '9')).toBe(false)
    // Campo vazio não é erro de par — é o superRefine de obrigatoriedade que cobra.
    expect(isValidVehicleTypePair('', '')).toBe(true)
  })
})
