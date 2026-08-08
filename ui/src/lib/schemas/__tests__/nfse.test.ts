import {describe, expect, it} from 'vitest'
import {nfseCancelSchema, nfseEmitSchema, nfseEventSchema} from '@/lib/schemas/nfse'
import {serviceSchema} from '@/lib/schemas/services'
import {nfseConfigSchema} from '@/lib/schemas/fiscal-configs'

const emitBase = {
  tp_emit: '1' as const,
  competence: '2026-08-01',
  service: {service_id: 'SERVICE_x'},
}

describe('nfseEmitSchema', () => {
  it('aceita o caso mínimo (tp_emit=1, sem provider_person_id)', () => {
    expect(nfseEmitSchema.safeParse(emitBase).success).toBe(true)
  })

  it('rejeita valor de serviço com vírgula decimal', () => {
    const r = nfseEmitSchema.safeParse({...emitBase, service: {...emitBase.service, value: '1000,00'}})
    expect(r.success).toBe(false)
  })

  it('exige provider_person_id e motivo_emis_ti quando tp_emit é 2', () => {
    const r = nfseEmitSchema.safeParse({...emitBase, tp_emit: '2'})
    expect(r.success).toBe(false)
    if (!r.success) {
      const paths = r.error.issues.map((i) => i.path.join('.'))
      expect(paths).toContain('provider_person_id')
      expect(paths).toContain('motivo_emis_ti')
    }
  })

  it('aceita tp_emit 2 com provider_person_id e motivo_emis_ti', () => {
    const r = nfseEmitSchema.safeParse({
      ...emitBase, tp_emit: '2', provider_person_id: 'CNPJ_123', motivo_emis_ti: '1',
    })
    expect(r.success).toBe(true)
  })

  it('aceita apenas competência ISO com uma data civil válida', () => {
    expect(nfseEmitSchema.safeParse({...emitBase, competence: '01/08/2026'}).success).toBe(false)
    expect(nfseEmitSchema.safeParse({...emitBase, competence: '2026-02-31'}).success).toBe(false)
  })

  it('exige substitutes_reason quando substitutes_access_key é informado', () => {
    const r = nfseEmitSchema.safeParse({...emitBase, substitutes_access_key: '1'.repeat(50)})
    expect(r.success).toBe(false)
  })
})

describe('nfseEventSchema', () => {
  it('exige reason_code no cancelamento', () => {
    expect(nfseEventSchema.safeParse({event_type: '101101'}).success).toBe(false)
    expect(nfseEventSchema.safeParse({event_type: '101101', reason_code: '2', reason_description: 'Erro de emissão'}).success).toBe(true)
  })

  it('rejeita evento privativo do fisco', () => {
    expect(nfseEventSchema.safeParse({event_type: '105104'}).success).toBe(false)
  })

  it('rejeita 105102, que é substituição e não evento', () => {
    expect(nfseEventSchema.safeParse({event_type: '105102'}).success).toBe(false)
  })

  it('confirmação do prestador exige reason_code mas não reason_description', () => {
    expect(nfseEventSchema.safeParse({event_type: '202205'}).success).toBe(false)
    expect(nfseEventSchema.safeParse({event_type: '202205', reason_code: '1'}).success).toBe(true)
  })
})

describe('nfseCancelSchema', () => {
  it('exige reason_code e reason_description', () => {
    expect(nfseCancelSchema.safeParse({}).success).toBe(false)
    expect(nfseCancelSchema.safeParse({reason_code: '2', reason_description: 'Erro de valor'}).success).toBe(true)
  })
})

describe('serviceSchema', () => {
  const ibsCbs = {
    c_ind_op: '100301', cst: '000', c_class_trib: '000001',
    ind_dest: '0', tp_oper: '', fin_nfse: '0',
  }

  it('exige código de tributação nacional de 6 dígitos', () => {
    expect(serviceSchema.safeParse({
      code: 'S1', description: 'Consultoria', trib_nacional_code: '10101',
      unit: 'UN', value: '100.00', iss: {trib_issqn: '1', tax_rate: '2.00'}, ibs_cbs: ibsCbs,
    }).success).toBe(false)
    expect(serviceSchema.safeParse({
      code: 'S1', description: 'Consultoria', trib_nacional_code: '010101',
      unit: 'UN', value: '100.00', iss: {trib_issqn: '1', tax_rate: '2.00'}, ibs_cbs: ibsCbs,
    }).success).toBe(true)
  })

  it('rejeita tp_imunidade quando trib_issqn não é imunidade', () => {
    const r = serviceSchema.safeParse({
      code: 'S1', description: 'Consultoria', trib_nacional_code: '010101',
      unit: 'UN', value: '100.00', iss: {trib_issqn: '1', tax_rate: '2.00', tp_imunidade: '1'},
      ibs_cbs: ibsCbs,
    })
    expect(r.success).toBe(false)
  })
})

describe('nfseConfigSchema', () => {
  const base = {
    provider: 'nacional' as const, environment: '2' as const,
    timezone: 'America/Fortaleza' as const,
    c_loc_emi: '2211001', serie: '1', prod_current_number: '0', hom_current_number: '0',
  }

  it('aceita provider nacional sem campos ABRASF', () => {
    expect(nfseConfigSchema.safeParse(base).success).toBe(true)
  })

  it('exige endpoint/wsdl/município quando provider é abrasf204', () => {
    const r = nfseConfigSchema.safeParse({...base, provider: 'abrasf204'})
    expect(r.success).toBe(false)
    if (!r.success) {
      const paths = r.error.issues.map((i) => i.path.join('.'))
      expect(paths).toContain('abrasf_endpoint_url')
      expect(paths).toContain('abrasf_wsdl_version')
      expect(paths).toContain('abrasf_municipality_code')
    }
  })
})
