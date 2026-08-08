import {describe, expect, it} from 'vitest'
import {serviceSchema} from '@/lib/schemas/services'

const base = {
  code: 'SVC001',
  description: 'Consultoria em TI',
  trib_nacional_code: '010101',
  trib_municipal_code: '',
  nbs_code: '',
  cnae: '',
  unit: 'UN',
  value: '1000.00',
  iss: {trib_issqn: '1', tax_rate: '5.00', tp_ret_issqn: '', tp_imunidade: '', c_pais_resultado: ''},
  ibs_cbs: {c_ind_op: '100301', cst: '000', c_class_trib: '000001', ind_dest: '0', tp_oper: '', fin_nfse: '0'},
}

describe('serviceSchema — alíquota de ISSQN', () => {
  it('exige alíquota na operação tributável', () => {
    const r = serviceSchema.safeParse({...base, iss: {...base.iss, tax_rate: ''}})
    expect(r.success).toBe(false)
  })

  it('dispensa alíquota quando não há tributação (imunidade)', () => {
    const r = serviceSchema.safeParse({...base, iss: {...base.iss, trib_issqn: '2', tax_rate: ''}})
    expect(r.success).toBe(true)
  })

  it('normaliza espaços no código e na descrição', () => {
    const parsed = serviceSchema.parse({...base, code: '  SVC001  ', description: '  Consultoria em TI  '})

    expect(parsed.code).toBe('SVC001')
    expect(parsed.description).toBe('Consultoria em TI')
  })

  it('exige a classificação IBS/CBS', () => {
    expect(serviceSchema.safeParse({...base, ibs_cbs: undefined}).success).toBe(false)
  })
})
