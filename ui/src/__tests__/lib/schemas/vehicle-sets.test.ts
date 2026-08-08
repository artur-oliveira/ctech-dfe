import {describe, expect, it} from 'vitest'
import {MAX_TRAILERS, vehicleSetSchema} from '@/lib/schemas/vehicle-sets'

describe('vehicleSetSchema', () => {
  const valid = {
    name: 'Carreta 1',
    tractor_sk: 'VEHICLE_ABC1D23',
    trailer_sks: ['VEHICLE_XYZ9K88'],
    driver_docs: ['11144477735'],
    rntrc: '12345678',
    ciot: '',
  }

  it('aceita uma composição completa', () => {
    expect(vehicleSetSchema.safeParse(valid).success).toBe(true)
  })

  it('exige veículo de tração', () => {
    expect(vehicleSetSchema.safeParse({...valid, tractor_sk: ''}).success).toBe(false)
  })

  it(`recusa mais de ${MAX_TRAILERS} reboques`, () => {
    const trailers = Array.from({length: MAX_TRAILERS + 1}, (_, i) => `VEHICLE_T${i}`)
    expect(vehicleSetSchema.safeParse({...valid, trailer_sks: trailers}).success).toBe(false)
  })

  it('aceita RNTRC vazio mas recusa formato inválido', () => {
    expect(vehicleSetSchema.safeParse({...valid, rntrc: ''}).success).toBe(true)
    expect(vehicleSetSchema.safeParse({...valid, rntrc: '123'}).success).toBe(false)
  })
})
