import {describe, it, expect} from 'vitest'
import {cfopConfigSchema, productSchema} from '@/lib/schemas/products'

const valueResale = productSchema.shape.value_resale

describe('cfopConfigSchema — uf_overrides e IBS/CBS opcional', () => {
  const base = {
    cfop: '5102', pis: '01', cofins: '01',
    ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
  }

  it('aceita IBS/CBS totalmente vazio', () => {
    expect(cfopConfigSchema.safeParse(base).success).toBe(true)
  })

  it('aceita uf_overrides com UF válida', () => {
    const result = cfopConfigSchema.safeParse({
      ...base,
      uf_overrides: [{ufs: ['SP', 'RJ'], overrides: {icms_aliq_override: '12.0000'}}],
    })
    expect(result.success).toBe(true)
  })

  it('rejeita uf_overrides sem nenhuma UF', () => {
    const result = cfopConfigSchema.safeParse({
      ...base,
      uf_overrides: [{ufs: [], overrides: {}}],
    })
    expect(result.success).toBe(false)
  })
})

describe('productSchema.value_resale', () => {
  it('aceita vazio (opcional)', () => {
    expect(valueResale.safeParse('').success).toBe(true)
    expect(valueResale.safeParse(undefined).success).toBe(true)
  })

  it('aceita valor monetário válido', () => {
    expect(valueResale.safeParse('99.90').success).toBe(true)
    expect(valueResale.safeParse('8').success).toBe(true)
  })

  it('rejeita valor inválido', () => {
    expect(valueResale.safeParse('abc').success).toBe(false)
    expect(valueResale.safeParse('1.234567').success).toBe(false)
  })
})

describe('productSchema — regras cruzadas do leiaute', () => {
  const base = {
    code: 'PROD1',
    description: 'Produto de teste',
    ncm: '84713012',
    origin: '0',
    unit: 'UN',
    value: '10.00',
    ind_tot: '1' as const,
    cfop_nfce: '5102',
    cfop_config: [],
    conversion_factors: [],
  }

  const pathsOf = (data: Record<string, unknown>): string[] => {
    const result = productSchema.safeParse(data)
    return result.success ? [] : result.error.issues.map((i) => i.path.join('.'))
  }

  it('aceita o produto genérico mínimo', () => {
    expect(productSchema.safeParse(base).success).toBe(true)
  })

  it('combustível exige cProdANP, descrição ANP e UF de consumo', () => {
    expect(pathsOf({...base, prod_type: 'comb'})).toEqual(
      expect.arrayContaining(['comb_c_prod_anp', 'comb_desc_anp', 'comb_uf_cons']),
    )
  })

  it('veículo novo exige todo o grupo veicProd, incluindo cor, CMT, distância e pesos', () => {
    const paths = pathsOf({...base, prod_type: 'veiculo'})
    expect(paths).toEqual(expect.arrayContaining([
      'veic_tp_op', 'veic_c_cor', 'veic_x_cor', 'veic_cmt', 'veic_dist', 'net_weight', 'gross_weight',
    ]))
  })

  it('ANVISA ISENTO exige motivo, e motivo sem ISENTO é recusado', () => {
    expect(pathsOf({...base, med_c_prod_anvisa: 'ISENTO'})).toContain('med_x_motivo_isencao')
    expect(pathsOf({...base, med_c_prod_anvisa: '1234567890123', med_x_motivo_isencao: 'qualquer'}))
      .toContain('med_x_motivo_isencao')
  })

  it('produção fora de escala exige o CNPJ do fabricante', () => {
    expect(pathsOf({...base, ind_escala: 'N'})).toContain('cnpj_fab')
  })

  it('selo do IPI é um grupo: código e quantidade andam juntos', () => {
    expect(pathsOf({...base, ipi_c_selo: 'ABC'})).toContain('ipi_q_selo')
    expect(pathsOf({...base, ipi_q_selo: '10'})).toContain('ipi_c_selo')
    expect(productSchema.safeParse({...base, ipi_c_selo: 'ABC', ipi_q_selo: '10'}).success).toBe(true)
  })

  it('número ONU arrasta nome e classe de risco', () => {
    // O grupo de embalagem só é cobrado depois que a classe é conhecida: há
    // classes da ANTT que não recebem grupo nenhum.
    expect(pathsOf({...base, peri_n_onu: '1203'})).toEqual(
      expect.arrayContaining(['peri_x_nome_ae', 'peri_x_cla_risco']),
    )
  })

  it('origem do combustível tem que somar 100%', () => {
    const orig = [
      {ind_import: '0' as const, c_uf_orig: '35', p_orig: '60'},
      {ind_import: '0' as const, c_uf_orig: '33', p_orig: '30'},
    ]
    expect(pathsOf({...base, comb_orig: orig})).toContain('comb_orig')
    const fechado = [...orig.slice(0, 1), {...orig[1], p_orig: '40'}]
    expect(pathsOf({...base, comb_orig: fechado})).not.toContain('comb_orig')
  })

  it('peso bruto não fica abaixo do líquido', () => {
    expect(pathsOf({...base, net_weight: '10.000', gross_weight: '9.000'})).toContain('gross_weight')
    expect(productSchema.safeParse({...base, net_weight: '10.000', gross_weight: '10.500'}).success).toBe(true)
  })

  it('recusa GTIN com dígito verificador errado e aceita o correto', () => {
    expect(pathsOf({...base, cean: '7891234567890'})).toContain('cean')
    expect(productSchema.safeParse({...base, cean: '7891000315507'}).success).toBe(true)
    expect(productSchema.safeParse({...base, cean: 'SEM GTIN'}).success).toBe(true)
  })
})

describe('cfopConfigSchema — grupos do tratamento tributário', () => {
  const base = {cfop: '5102', pis: '01', cofins: '01'}

  const pathsOf = (data: Record<string, unknown>): string[] => {
    const result = cfopConfigSchema.safeParse(data)
    return result.success ? [] : result.error.issues.map((i) => i.path.join('.'))
  }

  it('CST de IPI tributado exige alíquota ou valor por unidade', () => {
    expect(pathsOf({...base, ipi_cst: '00'})).toContain('ipi_aliq')
    expect(cfopConfigSchema.safeParse({...base, ipi_cst: '00', ipi_aliq: '5.00'}).success).toBe(true)
    expect(cfopConfigSchema.safeParse({...base, ipi_cst: '00', ipi_v_unid: '1.50'}).success).toBe(true)
  })

  it('partilha do ICMS exige percentual da operação própria e UF do ST juntos', () => {
    expect(pathsOf({...base, icms_part_p_bc_op: '40'})).toContain('icms_part_uf_st')
    expect(pathsOf({...base, icms_part_uf_st: 'SP'})).toContain('icms_part_p_bc_op')
  })

  it('base de cálculo por pauta exige o valor da pauta', () => {
    expect(pathsOf({...base, icms_mod_bc: '1'})).toContain('icms_pauta_valor')
    expect(cfopConfigSchema.safeParse({...base, icms_mod_bc: '3'}).success).toBe(true)
  })

  it('ALC/ZFM e observação do item são grupos completos', () => {
    expect(pathsOf({...base, alc_zfm_tp_cbs: '1'})).toContain('alc_zfm_n_proc_suframa')
    expect(pathsOf({...base, obs_item_x_campo: 'Beneficio'})).toContain('obs_item_x_texto')
  })
})

describe('productSchema — tabelas oficiais de produto perigoso e IPI', () => {
  const base = {
    code: 'PROD1', description: 'Produto de teste', ncm: '84713012', origin: '0',
    unit: 'UN', value: '10.00', ind_tot: '1' as const, cfop_nfce: '5102',
    cfop_config: [], conversion_factors: [],
  }

  const pathsOf = (data: Record<string, unknown>): string[] => {
    const result = productSchema.safeParse(data)
    return result.success ? [] : result.error.issues.map((i) => i.path.join('.'))
  }

  it('recusa classe de risco fora da tabela da ANTT', () => {
    expect(pathsOf({...base, peri_x_cla_risco: '13'})).toContain('peri_x_cla_risco')
    expect(pathsOf({...base, peri_x_cla_risco: '3'})).not.toContain('peri_x_cla_risco')
  })

  it('não cobra grupo de embalagem nas classes que não o recebem', () => {
    // Classe 2.1 (gás inflamável) não tem grupo de embalagem.
    const gas = {...base, peri_n_onu: '1978', peri_x_nome_ae: 'PROPANO', peri_x_cla_risco: '2.1'}
    expect(pathsOf(gas)).not.toContain('peri_gr_emb')
    // Classe 3 (líquido inflamável) tem.
    const liquido = {...base, peri_n_onu: '1203', peri_x_nome_ae: 'GASOLINA', peri_x_cla_risco: '3'}
    expect(pathsOf(liquido)).toContain('peri_gr_emb')
  })

  it('recusa grupo de embalagem numa classe que não o admite', () => {
    const errado = {
      ...base, peri_n_onu: '1978', peri_x_nome_ae: 'PROPANO',
      peri_x_cla_risco: '2.1', peri_gr_emb: 'II',
    }
    expect(pathsOf(errado)).toContain('peri_gr_emb')
  })

  it('recusa enquadramento de IPI fora da tabela e aceita o genérico', () => {
    expect(pathsOf({...base, ipi_c_enq: '888'})).toContain('ipi_c_enq')
    expect(pathsOf({...base, ipi_c_enq: '999'})).not.toContain('ipi_c_enq')
    expect(pathsOf({...base, ipi_c_enq: '101'})).not.toContain('ipi_c_enq')
  })
})
