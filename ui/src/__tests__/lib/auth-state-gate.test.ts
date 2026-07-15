import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {doRefresh, isRevoked, revokeToken} from '@/lib/auth/oauth'
import {AUTH_STATE_ACTIVE, SESSION_KEY_AUTH_STATE} from '@/lib/constants/storage'

const fetchMock = vi.fn()

describe('auth state gate (revoked → skip /token)', () => {
  beforeEach(() => {
    sessionStorage.clear()
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('doRefresh skips the /token fetch once revoked', async () => {
    await revokeToken() // sets revoked; fetch to /revoke is best-effort
    expect(isRevoked()).toBe(true)
    fetchMock.mockClear()

    const result = await doRefresh()

    expect(result).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled() // no /token 400 storm
  })

  it('a rejected refresh marks the session revoked', async () => {
    fetchMock.mockResolvedValue({ok: false, status: 400})

    expect(await doRefresh()).toBeNull()
    expect(isRevoked()).toBe(true)
  })

  it('a successful refresh clears the revoked flag', async () => {
    fetchMock.mockResolvedValue({ok: true, json: async () => ({access_token: 'at'})})

    const result = await doRefresh()

    expect(result).toEqual({accessToken: 'at'})
    expect(sessionStorage.getItem(SESSION_KEY_AUTH_STATE)).toBe(AUTH_STATE_ACTIVE)
    expect(isRevoked()).toBe(false)
  })

  it('a network error during refresh does NOT mark revoked (may be transient)', async () => {
    fetchMock.mockRejectedValue(new Error('network'))

    expect(await doRefresh()).toBeNull()
    expect(isRevoked()).toBe(false)
  })
})
