import {afterAll, describe, it, expect} from 'vitest'
import {decodeIdToken, close} from '@/lib/auth/oauth'

// Builds a fake JWT (header.payload.signature) whose payload is base64url(JSON(claims)).
function makeToken(claims: Record<string, unknown>): string {
  const bytes = new TextEncoder().encode(JSON.stringify(claims))
  let bin = ''
  bytes.forEach((b) => (bin += String.fromCharCode(b)))
  const payload = btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
  return `header.${payload}.signature`
}

describe('decodeIdToken', () => {
  afterAll(() => {
    close()
  })
  it('maps given_name/family_name/preferred_username to name fields', () => {
    const token = makeToken({
      sub: 'u1',
      preferred_username: 'joao.silva',
      given_name: 'João',
      family_name: 'Silva',
    })
    expect(decodeIdToken(token)).toEqual({
      username: 'joao.silva',
      first_name: 'João',
      last_name: 'Silva',
    })
  })

  it('decodes accented UTF-8 names correctly', () => {
    const token = makeToken({given_name: 'José', family_name: 'Conceição'})
    const claims = decodeIdToken(token)
    expect(claims?.first_name).toBe('José')
    expect(claims?.last_name).toBe('Conceição')
  })

  it('returns undefined for absent claims but keeps present ones', () => {
    const token = makeToken({given_name: 'Ana'})
    expect(decodeIdToken(token)).toEqual({
      username: undefined,
      first_name: 'Ana',
      last_name: undefined,
    })
  })

  it('returns null when no name claim is present', () => {
    expect(decodeIdToken(makeToken({sub: 'u1', email: 'a@b.com'}))).toBeNull()
  })

  it('returns null for a malformed token (no payload segment)', () => {
    expect(decodeIdToken('notajwt')).toBeNull()
  })

  it('returns null when the payload is not valid base64/JSON', () => {
    expect(decodeIdToken('header.!!!not-base64!!!.sig')).toBeNull()
  })
})
