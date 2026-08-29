import {afterEach, describe, expect, it, vi} from 'vitest'
import {datetimeLocalToOffset, localTimezoneOffset} from '@/lib/utils/datetime'

/** Finge o fuso do navegador trocando getTimezoneOffset (minutos até o UTC). */
function withOffsetMinutes(minutesToUtc: number) {
  const spy = vi.spyOn(Date.prototype, 'getTimezoneOffset')
  spy.mockReturnValue(minutesToUtc)
  return spy
}

afterEach(() => vi.restoreAllMocks())

describe('localTimezoneOffset', () => {
  it('formata o fuso de Brasília', () => {
    withOffsetMinutes(180)
    expect(localTimezoneOffset()).toBe('-03:00')
  })

  it('formata o fuso do Acre, que não é -03:00', () => {
    withOffsetMinutes(300)
    expect(localTimezoneOffset()).toBe('-05:00')
  })

  it('formata fuso a leste de Greenwich', () => {
    withOffsetMinutes(-120)
    expect(localTimezoneOffset()).toBe('+02:00')
  })

  it('formata fuso com minutos quebrados', () => {
    withOffsetMinutes(-330)
    expect(localTimezoneOffset()).toBe('+05:30')
  })
})

describe('datetimeLocalToOffset', () => {
  it('acrescenta segundos e fuso ao valor do datetime-local', () => {
    withOffsetMinutes(180)
    expect(datetimeLocalToOffset('2026-09-11T10:00')).toBe('2026-09-11T10:00:00-03:00')
  })

  it('preserva os segundos quando o input já os traz', () => {
    withOffsetMinutes(180)
    expect(datetimeLocalToOffset('2026-09-11T10:00:30')).toBe('2026-09-11T10:00:30-03:00')
  })

  it('campo em branco vira string vazia — a tag simplesmente não sai', () => {
    expect(datetimeLocalToOffset('')).toBe('')
  })

  it('valor inválido não inventa data', () => {
    expect(datetimeLocalToOffset('não é data')).toBe('')
  })
})
