import {describe, it, expect} from 'vitest'
import {stripNulls, isStrippableBody} from '@/lib/utils/strip-nulls'

describe('stripNulls', () => {
  it('drops null and undefined when dropNull=true', () => {
    expect(stripNulls({a: 1, b: null, c: undefined, d: ''}, true)).toEqual({a: 1, d: ''})
  })
  it('keeps null but drops undefined when dropNull=false', () => {
    expect(stripNulls({a: 1, b: null, c: undefined}, false)).toEqual({a: 1, b: null})
  })
  it('recurses into nested objects and arrays', () => {
    expect(stripNulls({a: {b: null, c: 2}, list: [{x: null, y: 3}]}, true))
      .toEqual({a: {c: 2}, list: [{y: 3}]})
  })
  it('preserves falsy non-null values', () => {
    expect(stripNulls({a: 0, b: false, c: ''}, true)).toEqual({a: 0, b: false, c: ''})
  })
})

describe('request payload stripping policy', () => {
  it('POST drops nulls', () => {
    expect(stripNulls({cest: null, name: 'x'}, true)).toEqual({name: 'x'})
  })
  it('PUT keeps explicit null (field clear)', () => {
    expect(stripNulls({cest: null, name: 'x'}, false)).toEqual({cest: null, name: 'x'})
  })
})

describe('isStrippableBody', () => {
  it('accepts plain objects and arrays', () => {
    expect(isStrippableBody({a: 1})).toBe(true)
    expect(isStrippableBody([{a: 1}])).toBe(true)
  })
  it('rejects FormData (would be flattened to {}) and other non-plain bodies', () => {
    expect(isStrippableBody(new FormData())).toBe(false)
    expect(isStrippableBody(new URLSearchParams())).toBe(false)
    expect(isStrippableBody('raw')).toBe(false)
    expect(isStrippableBody(null)).toBe(false)
    expect(isStrippableBody(undefined)).toBe(false)
  })
})
