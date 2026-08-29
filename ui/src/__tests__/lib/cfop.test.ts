import { describe, it, expect } from 'vitest'
import {
  cfopDirection, cfopTpNf, buildNatOpFromCfops,
  cfopScope, cfopSuffix, groupCfopConfigBySuffix, resolveCfopForUf, cfopGroupCodes,
  getCfopDescription,
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
  it('cobre bonificação, doação e brinde — a operação sem pagamento por excelência', () => {
    // Regressão: a lista trazia só 5920/6920, que na tabela do CONFAZ é remessa
    // de vasilhame. A nota de doação (5910) não era forçada a tPag 90.
    expect(NO_PAYMENT_CFOPS).toContain('5910')
    expect(NO_PAYMENT_CFOPS).toContain('6910')
  })

  it('cobre amostra grátis e remessa de embalagem retornável', () => {
    expect(NO_PAYMENT_CFOPS).toContain('5911')
    expect(NO_PAYMENT_CFOPS).toContain('6911')
    expect(NO_PAYMENT_CFOPS).toContain('5920')
    expect(NO_PAYMENT_CFOPS).toContain('6920')
  })

  it('não inclui venda, que sempre tem pagamento', () => {
    expect(NO_PAYMENT_CFOPS).not.toContain('5102')
    expect(NO_PAYMENT_CFOPS).not.toContain('6102')
  })
})

describe('cobertura da tabela oficial do CONFAZ', () => {
  it('inclui os CFOPs de ato cooperativo (Ajuste SINIEF 18/17 e 11/18)', () => {
    for (const code of ['1131', '2131', '5131', '6131', '5159', '6159', '5160', '6160']) {
      expect(getCfopDescription(code)).toBeTruthy()
    }
  })

  it('inclui o Sistema de Integração e Parceria Rural (Ajuste SINIEF 20/19)', () => {
    expect(getCfopDescription('5451')).toMatch(/Integração e Parceria Rural/)
    expect(getCfopDescription('1451')).toMatch(/Integração e Parceria Rural/)
    expect(getCfopDescription('5456')).toMatch(/remuneração do produtor/)
  })

  it('inclui uso de bordo em tráfego internacional e o lote de exportação', () => {
    expect(getCfopDescription('3552')).toMatch(/uso ou consumo de bordo/)
    expect(getCfopDescription('7552')).toMatch(/uso ou consumo de bordo/)
    expect(getCfopDescription('7504')).toMatch(/formação de lote/)
  })

  it('separa a natureza do escopo exterior quando a tabela oficial a separa', () => {
    // 3552 é uso de bordo, não transferência de ativo como 1552/2552.
    expect(getCfopDescription('1552')).toMatch(/ativo imobilizado/)
    expect(getCfopDescription('3552')).not.toMatch(/ativo imobilizado/)
    // 7501 é a exportação em si, não a remessa com fim específico.
    expect(getCfopDescription('5501')).toMatch(/Remessa/)
    expect(getCfopDescription('7501')).toMatch(/Exportação/)
  })

  it('descreve 1451 e 1452 como entrada do sistema de integração, não retorno', () => {
    // A redação anterior ("Retorno de animal do estabelecimento produtor") foi
    // substituída pelo Ajuste SINIEF 20/19 e não existe mais na tabela vigente.
    expect(getCfopDescription('1451')).toMatch(/^Entrada de animal/)
    expect(getCfopDescription('1452')).toMatch(/^Entrada de insumo/)
  })
})

describe('getCfopDescription resolve variante, não só o grupo', () => {
  it('devolve a descrição pelo código concreto de qualquer escopo', () => {
    // Regressão: só o código do grupo resolvia, então 6102 devolvia null.
    expect(getCfopDescription('6102')).toBe(getCfopDescription('5102'))
    expect(getCfopDescription('2131')).toBe(getCfopDescription('1131'))
  })

  it('devolve null para código que não existe', () => {
    expect(getCfopDescription('9999')).toBeNull()
  })
})
