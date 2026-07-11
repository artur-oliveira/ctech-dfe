import {describe, it, expect} from 'vitest'
import {vehicleSchema} from '@/lib/schemas/vehicles'

describe('vehicleSchema', () => {
  it('aceita apenas placa, UF e role (mínimo)', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP', role: 'tractor'})
    expect(result.success).toBe(true)
  })

  it('rejeita sem role', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP'})
    expect(result.success).toBe(false)
  })

  it('rejeita role inválido', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP', role: 'carro'})
    expect(result.success).toBe(false)
  })

  it('aceita campos avançados quando presentes', () => {
    const result = vehicleSchema.safeParse({
      plate: 'ABC1D23', plate_uf: 'SP', role: 'tractor',
      wheelset: '01', bodywork: '00', renavam: '123456789', weight: '8000',
    })
    expect(result.success).toBe(true)
  })

  it('não exige mais o bloco owner', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP', role: 'trailer'})
    expect(result.success).toBe(true)
  })
})
