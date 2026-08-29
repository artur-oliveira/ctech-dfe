import {describe, expect, it} from 'vitest'
import {CFOP_SUFFIXES, cfopSuffixOptions, getAllCfopOptions} from '@/lib/data/cfop'
import {UF_IBGE_OPTIONS} from '@/lib/data/cities'
import {LC116_SERVICE_CODES, LC116_SERVICE_OPTIONS} from '@/lib/data/lc116_services'
import {BACEN_COUNTRIES, BACEN_COUNTRY_BRAZIL, BACEN_COUNTRY_CODES} from '@/lib/data/bacen_countries'
import {cEnqOptionsForCst, IPI_CENQ, IPI_CENQ_DEFAULT} from '@/lib/data/ipi_cenq'
import {PACKING_GROUP_OPTIONS, packingGroupApplies, RISK_CLASS_OPTIONS} from '@/lib/data/dangerous_goods'

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
  it('traz os 200 itens oficiais no formato NN.NN, sem repetir', () => {
    expect(LC116_SERVICE_OPTIONS).toHaveLength(200)
    expect(LC116_SERVICE_OPTIONS.every((o) => /^\d{2}\.\d{2}$/.test(o.value))).toBe(true)
    expect(new Set(LC116_SERVICE_OPTIONS.map((o) => o.value)).size).toBe(200)
  })

  it('traz análise e desenvolvimento de sistemas como 01.01', () => {
    expect(LC116_SERVICE_OPTIONS[0].value).toBe('01.01')
  })

  it('não inclui o 99.01 da tabela da NFS-e, que o cListServ não aceita', () => {
    expect(LC116_SERVICE_CODES.has('99.01')).toBe(false)
  })
})

describe('BACEN_COUNTRIES', () => {
  it('traz os 249 países vigentes da tabela oficial, sem repetir', () => {
    expect(BACEN_COUNTRIES).toHaveLength(249)
    expect(new Set(BACEN_COUNTRIES.map((c) => c.code)).size).toBe(249)
    expect(BACEN_COUNTRIES.every((c) => /^\d{4}$/.test(c.code))).toBe(true)
  })

  it('mapeia códigos conhecidos', () => {
    const by = new Map(BACEN_COUNTRIES.map((c) => [c.code, c.description]))
    expect(by.get(BACEN_COUNTRY_BRAZIL)).toBe('Brasil')
    expect(by.get('2496')).toBe('Estados Unidos')
    expect(by.get('0639')).toBe('Argentina')
  })

  it('não inclui os códigos encerrados em 2018', () => {
    for (const dead of ['0200', '0477', '1504', '1508', '1511', '3599', '8737']) {
      expect(BACEN_COUNTRY_CODES.has(dead)).toBe(false)
    }
  })
})

describe('IPI_CENQ', () => {
  it('traz os 132 códigos da NT 2020.002, sem repetir', () => {
    expect(IPI_CENQ).toHaveLength(132)
    expect(new Set(IPI_CENQ.map((e) => e.code)).size).toBe(132)
    expect(IPI_CENQ.every((e) => /^\d{3}$/.test(e.code))).toBe(true)
  })

  it('restringe o enquadramento à faixa que o CST do IPI aceita', () => {
    // RV W16-10: CST 04/54 é imunidade (001–099), 05/55 suspensão (101–199),
    // 02/52 isenção (301–399), e os demais 999 ou 601–608.
    expect(cEnqOptionsForCst('04').every((o) => o.value < '100')).toBe(true)
    expect(cEnqOptionsForCst('05').every((o) => o.value >= '101' && o.value <= '199')).toBe(true)
    expect(cEnqOptionsForCst('02').every((o) => o.value >= '301' && o.value <= '399')).toBe(true)
    const outros = cEnqOptionsForCst('00').map((o) => o.value)
    expect(outros).toContain(IPI_CENQ_DEFAULT)
    expect(outros.every((v) => v === '999' || (v >= '601' && v <= '608'))).toBe(true)
  })

  it('sem CST, oferece a tabela inteira', () => {
    expect(cEnqOptionsForCst()).toHaveLength(132)
  })
})

describe('classificação de produto perigoso', () => {
  it('oferece só subclasses selecionáveis, sem as classes-pai', () => {
    const values = RISK_CLASS_OPTIONS.map((o) => o.value)
    expect(values).toContain('1.1')
    expect(values).toContain('3')
    expect(values).not.toContain('1')
    expect(values).not.toContain('2')
  })

  it('sabe quais classes não recebem grupo de embalagem', () => {
    for (const semGrupo of ['1.1', '2.1', '5.2', '6.2', '7']) {
      expect(packingGroupApplies(semGrupo)).toBe(false)
    }
    for (const comGrupo of ['3', '4.1', '5.1', '6.1', '8', '9']) {
      expect(packingGroupApplies(comGrupo)).toBe(true)
    }
    expect(packingGroupApplies(null)).toBe(false)
  })

  it('tem exatamente os três grupos de embalagem da ANTT', () => {
    expect(PACKING_GROUP_OPTIONS.map((o) => o.value)).toEqual(['I', 'II', 'III'])
  })
})
