import {describe, expect, it} from 'vitest'
import {generateEntityCode} from '@/lib/utils/code'

describe('generateEntityCode', () => {
  it('gera 16 caracteres do alfabeto aceito pelo cadastro (A–Z, 0–9)', () => {
    const code = generateEntityCode()
    expect(code).toHaveLength(16)
    expect(code).toMatch(/^[0-9A-HJKMNP-TV-Z]{16}$/)
  })

  it('não repete entre chamadas', () => {
    const codes = new Set(Array.from({length: 200}, generateEntityCode))
    expect(codes.size).toBe(200)
  })
})
