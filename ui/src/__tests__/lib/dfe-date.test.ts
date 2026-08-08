import {describe, expect, it} from 'vitest'
import {formatISODateBR} from '@/lib/utils/dfe'

describe('formatISODateBR', () => {
  it('formata a data civil sem aplicar conversão de timezone', () => {
    expect(formatISODateBR('2026-08-01')).toBe('01/08/2026')
  })

  it('preserva valores que não são datas ISO', () => {
    expect(formatISODateBR('')).toBe('')
    expect(formatISODateBR('01/08/2026')).toBe('01/08/2026')
  })
})
