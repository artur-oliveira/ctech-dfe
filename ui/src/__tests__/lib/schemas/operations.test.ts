import {describe, expect, it} from 'vitest'
import {operationSchema, unknownPlaceholder} from '@/lib/schemas/operations'

const base = {
  name: 'Venda para revenda',
  doc_types: ['nfe' as const],
  nat_op: 'Venda de mercadoria',
  tp_nf: '1' as const,
  fin_nfe: '1' as const,
  ind_final: '1' as const,
  ind_pres: '1' as const,
  cfop_suffix: '102',
  tax_profile_id: '',
  payment_term_id: '',
  mod_frete: '' as const,
  inf_ad_fisco: '',
  inf_cpl: '',
  obs_cont: [],
  obs_fisco: [],
  requires_receiver: true,
  is_default: false,
}

describe('natureza de operação', () => {
  it('aceita uma operação completa', () => {
    expect(operationSchema.safeParse(base).success).toBe(true)
  })

  // A natureza fiscal são só os 3 últimos dígitos: o escopo (5/6/7) é resolvido
  // na emissão pelas UFs, não escolhido no cadastro.
  it('recusa CFOP completo no lugar da natureza fiscal', () => {
    expect(operationSchema.safeParse({...base, cfop_suffix: '5102'}).success).toBe(false)
  })

  it('aceita operação sem natureza fiscal', () => {
    expect(operationSchema.safeParse({...base, cfop_suffix: ''}).success).toBe(true)
  })

  it('exige nome com ao menos 2 caracteres', () => {
    expect(operationSchema.safeParse({...base, name: 'V'}).success).toBe(false)
  })
})

describe('placeholders das mensagens fiscais', () => {
  it('aceita todas as chaves conhecidas', () => {
    const text = 'NF {{v_nf}} · ST {{v_icms_st}} · {{cliente}} · {{nat_op}} · {{competencia}}'
    expect(unknownPlaceholder(text)).toBeNull()
    expect(operationSchema.safeParse({...base, inf_cpl: text}).success).toBe(true)
  })

  it('tolera espaços dentro das chaves', () => {
    expect(unknownPlaceholder('{{ v_nf }}')).toBeNull()
  })

  // Chave desconhecida tem que falhar no cadastro; deixar passar viraria um
  // buraco silencioso no XML.
  it('recusa chave desconhecida, nomeando qual é', () => {
    expect(unknownPlaceholder('Total {{v_iss}}')).toBe('v_iss')
    const parsed = operationSchema.safeParse({...base, inf_cpl: 'Total {{v_iss}}'})
    expect(parsed.success).toBe(false)
    if (!parsed.success) {
      expect(parsed.error.issues[0].message).toContain('v_iss')
    }
  })

  it('texto sem placeholder nenhum é válido', () => {
    expect(unknownPlaceholder('Documento emitido em regime especial.')).toBeNull()
  })
})
