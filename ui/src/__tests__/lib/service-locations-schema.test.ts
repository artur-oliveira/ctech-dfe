import {describe, expect, it} from 'vitest'
import {serviceLocationSchema} from '@/lib/schemas/service-locations'

const national = {
  name: 'Obra Centro',
  roles: ['work'] as const,
  address_scope: 'national' as const,
  street: 'Rua C', number: '300', complement: '', neighborhood: 'Centro',
  postal_code: '64000000', city_ibge_code: '2211001',
  foreign_postal_code: '', foreign_city: '', foreign_region: '',
  insc_imob_fisc: '', c_obra: 'CNO-123', cib: '', id_atv_evt: '',
}

describe('serviceLocationSchema', () => {
  it('aceita um local nacional com um papel', () => {
    expect(serviceLocationSchema.safeParse(national).success).toBe(true)
  })

  it('exige ao menos um papel', () => {
    const result = serviceLocationSchema.safeParse({...national, roles: []})
    expect(result.success).toBe(false)
  })

  it('recusa código da obra e CIB juntos', () => {
    // serv/obra é a escolha cObra|cCIB|end: com os dois, a emissão decidiria sozinha.
    const result = serviceLocationSchema.safeParse({...national, cib: '12345678'})
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[0] === 'cib')).toBe(true)
  })

  it('exige CEP e município quando o local é no Brasil', () => {
    const result = serviceLocationSchema.safeParse({...national, postal_code: '', city_ibge_code: ''})
    expect(result.success).toBe(false)
    const fields = result.error?.issues.map((i) => i.path[0])
    expect(fields).toContain('postal_code')
    expect(fields).toContain('city_ibge_code')
  })

  it('recusa registros fiscais brasileiros num local no exterior', () => {
    const result = serviceLocationSchema.safeParse({
      ...national,
      address_scope: 'foreign',
      postal_code: '', city_ibge_code: '',
      foreign_postal_code: '10001', foreign_city: 'New York', foreign_region: 'NY',
    })
    expect(result.success).toBe(false)
    expect(result.error?.issues.some((i) => i.path[0] === 'c_obra')).toBe(true)
  })

  it('aceita um local no exterior sem registros brasileiros', () => {
    const result = serviceLocationSchema.safeParse({
      ...national,
      address_scope: 'foreign',
      postal_code: '', city_ibge_code: '', c_obra: '',
      foreign_postal_code: '10001', foreign_city: 'New York', foreign_region: 'NY',
    })
    expect(result.success).toBe(true)
  })
})
