import {describe, expect, it} from 'vitest'
import {getDfeThemeFromPath} from '@/lib/theme/dfe-theme'

describe('getDfeThemeFromPath', () => {
  it('distingue /nfse de /nfce', () => {
    expect(getDfeThemeFromPath('/nfse')).toBe('nfse')
    expect(getDfeThemeFromPath('/nfse/emit')).toBe('nfse')
    expect(getDfeThemeFromPath('/nfce')).toBe('nfce')
    expect(getDfeThemeFromPath('/nfce/detail')).toBe('nfce')
  })

  it('mantém NF-e como tema padrão', () => {
    expect(getDfeThemeFromPath('/nfe')).toBe('nfe')
    expect(getDfeThemeFromPath('/dashboard')).toBe('nfe')
  })
})
