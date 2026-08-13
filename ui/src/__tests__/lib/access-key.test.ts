import { describe, it, expect } from 'vitest'
import { validateAccessKey } from '@/lib/utils/access-key'

const VALID = '35250512345678000195550010000000011000000015'

describe('validateAccessKey', () => {
  it('aceita uma chave numérica válida', () => {
    expect(validateAccessKey(VALID)).toEqual({ valid: true })
  })

  it('rejeita tamanho incorreto', () => {
    expect(validateAccessKey(VALID.slice(0, 43))).toEqual({ valid: false, error: 'length' })
  })

  it('rejeita cUF inexistente', () => {
    expect(validateAccessKey('99' + VALID.slice(2))).toEqual({ valid: false, error: 'cUF' })
  })

  it('rejeita mês 13 em AAMM', () => {
    expect(validateAccessKey(VALID.slice(0, 2) + '2513' + VALID.slice(6))).toEqual({ valid: false, error: 'AAMM' })
  })

  it('rejeita mod diferente de 55 (NFC-e fora de escopo)', () => {
    expect(validateAccessKey(VALID.slice(0, 20) + '65' + VALID.slice(22))).toEqual({ valid: false, error: 'mod' })
  })

  it('rejeita tpEmis=9 (exclusivo de NFC-e)', () => {
    expect(validateAccessKey(VALID.slice(0, 34) + '9' + VALID.slice(35))).toEqual({ valid: false, error: 'tpEmis' })
  })

  it('rejeita cDV incorreto', () => {
    const lastDigit = VALID[43]
    const bad = lastDigit === '0' ? '1' : '0'
    expect(validateAccessKey(VALID.slice(0, 43) + bad)).toEqual({ valid: false, error: 'cDV' })
  })

  it('aceita CPF com prefixo 000 e rejeita DV de CPF inválido', () => {
    const base = VALID.slice(0, 6) + '00052998224725' + VALID.slice(20, 43)
    const ok = base + computeExpectedDV(base)
    expect(validateAccessKey(ok)).toEqual({ valid: true })

    const badCpfBase = VALID.slice(0, 6) + '00052998224724' + VALID.slice(20, 43)
    const badCpf = badCpfBase + computeExpectedDV(badCpfBase)
    expect(validateAccessKey(badCpf)).toEqual({ valid: false, error: 'doc' })
  })
})

// Test-local reimplementation of the cDV algorithm, kept intentionally
// separate from the production calcAccessKeyDV so this test doesn't silently
// pass if both were wrong in the same way.
function computeExpectedDV(key43: string): string {
  const weights = [2, 3, 4, 5, 6, 7, 8, 9]
  let sum = 0
  for (let i = key43.length - 1, wi = 0; i >= 0; i--, wi++) {
    sum += (key43.charCodeAt(i) - 48) * weights[wi % 8]
  }
  const rem = sum % 11
  return String(rem < 2 ? 0 : 11 - rem)
}
