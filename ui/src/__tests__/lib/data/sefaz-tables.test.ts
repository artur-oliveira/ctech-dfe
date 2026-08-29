import {describe, expect, it} from 'vitest'
import {ANP_CODE_SET, ANP_MONO_FUELS, anpMonoFuel, ANP_OPTIONS} from '@/lib/data/anp'
import {benefitOptionsForUf, CBENEF_UFS, isKnownBenefit, SEM_CBENEF, ufHasBenefitTable} from '@/lib/data/cbenef'
import {especieOptionsForTipo, isValidVehicleTypePair, VEHICLE_TYPE_PAIRS} from '@/lib/data/vehicle_type_pairs'
import {CARD_PAYMENT_TYPES, isPixPaymentType, TBAND_OPTIONS, TPAG_LABELS, TPAG_TABLE} from '@/lib/data/payment-tables'
import {IBS_CBS_CLASS_BY_CST, IBS_CBS_CLASS_CODES, IBS_CBS_CST} from '@/lib/data/ibs_cbs_cst'
import {taxableUnitForNcm} from '@/lib/data/ncm_taxable_unit'

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

describe('tabelas de pagamento', () => {
  it('traz os 23 meios de pagamento e as 28 bandeiras vigentes', () => {
    expect(TPAG_TABLE).toHaveLength(23)
    expect(TBAND_OPTIONS).toHaveLength(28)
  })

  it('classifica como PIX só 17, 20 e 23', () => {
    // Regressão: 12 e 13 são Vale Presente e Vale Combustível, e eram tratados
    // como PIX — abriam campos de transação PIX no meio de pagamento errado.
    expect(isPixPaymentType('17')).toBe(true)
    expect(isPixPaymentType('20')).toBe(true)
    expect(isPixPaymentType('23')).toBe(true)
    expect(isPixPaymentType('12')).toBe(false)
    expect(isPixPaymentType('13')).toBe(false)
  })

  it('rotula 12 e 13 como vale, conforme a tabela oficial', () => {
    expect(TPAG_LABELS['12']).toBe('Vale Presente')
    expect(TPAG_LABELS['13']).toBe('Vale Combustível')
    expect(TPAG_LABELS['20']).toContain('Estático')
  })

  it('pede dados de transação nos meios que os têm', () => {
    expect(CARD_PAYMENT_TYPES.has('03')).toBe(true)
    expect(CARD_PAYMENT_TYPES.has('20')).toBe(true)
    expect(CARD_PAYMENT_TYPES.has('01')).toBe(false)
    expect(CARD_PAYMENT_TYPES.has('90')).toBe(false)
  })
})

describe('tabela de IBS/CBS', () => {
  it('traz as 164 classificações publicadas nos 18 CSTs', () => {
    expect(IBS_CBS_CST).toHaveLength(18)
    expect(IBS_CBS_CLASS_CODES.size).toBe(164)
    expect([...IBS_CBS_CLASS_CODES].every((c) => /^\d{6}$/.test(c))).toBe(true)
  })

  it('cada classificação pertence a um CST só', () => {
    const all = IBS_CBS_CST.flatMap((e) => e.classCodes.map((c) => c.code))
    expect(new Set(all).size).toBe(all.length)
  })

  it('mantém a classificação da tributação integral e traz as que faltavam', () => {
    const integral = IBS_CBS_CLASS_BY_CST['000'].map((o) => o.value)
    expect(integral).toContain('000001')
    // 200002 é uma das 101 classificações ausentes antes desta tabela.
    expect(IBS_CBS_CLASS_CODES.has('200002')).toBe(true)
  })
})

describe('unidade tributável por NCM', () => {
  it('devolve a unidade publicada, com ou sem pontuação', () => {
    // 0101.21.00 (cavalos reprodutores) é tributado em UN, não no KG default.
    expect(taxableUnitForNcm('01012100')).toBe('UN')
    expect(taxableUnitForNcm('0101.21.00')).toBe('UN')
  })

  it('cai no default para NCM sem exceção publicada', () => {
    // 0201.10.00 (carcaças bovinas) usa o KG default; 8471.30.12 é UN.
    expect(taxableUnitForNcm('02011000')).toBe('KG')
    expect(taxableUnitForNcm('84713012')).toBe('UN')
  })

  it('devolve null para entrada que não é NCM', () => {
    expect(taxableUnitForNcm('')).toBeNull()
    expect(taxableUnitForNcm(null)).toBeNull()
    expect(taxableUnitForNcm('123')).toBeNull()
  })
})
