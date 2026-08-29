import {describe, expect, it} from 'vitest'
import {fuelPumpSchema} from '@/lib/schemas/fuel-pumps'

const base = {name: 'Bico 1 — Gasolina', n_bico: '1', n_bomba: '2', n_tanque: '3'}

describe('fuelPumpSchema', () => {
  it('aceita a bomba completa', () => {
    expect(fuelPumpSchema.safeParse(base).success).toBe(true)
  })

  it('exige o número do bico', () => {
    const r = fuelPumpSchema.safeParse({...base, n_bico: ''})
    expect(r.success).toBe(false)
    expect(r.error?.issues.some((i) => i.path[0] === 'n_bico')).toBe(true)
  })

  it('aceita bomba e tanque em branco — só o bico é obrigatório no leiaute', () => {
    expect(fuelPumpSchema.safeParse({...base, n_bomba: '', n_tanque: ''}).success).toBe(true)
  })

  it('recusa número com mais de 3 dígitos', () => {
    expect(fuelPumpSchema.safeParse({...base, n_bomba: '1234'}).success).toBe(false)
  })

  it('não aceita a leitura do encerrante — ela é escrita pela emissão', () => {
    const parsed = fuelPumpSchema.parse({...base, last_v_enc_fin: '1000.000'})
    expect('last_v_enc_fin' in parsed).toBe(false)
  })
})
