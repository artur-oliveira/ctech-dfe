import {afterAll, afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {doRefresh, revokeToken, close} from '@/lib/auth/oauth'

const fetchMock = vi.fn()

function setAuthHintCookie() {
  document.cookie = 'ctech_auth=1; path=/'
}

function clearAuthHintCookie() {
  document.cookie = 'ctech_auth=; Max-Age=0; path=/'
}

describe('auth state gate (revoked / no hint → skip /token)', () => {
  beforeEach(() => {
    sessionStorage.clear()
    clearAuthHintCookie()
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => vi.unstubAllGlobals())
  afterAll(() => {
    close()
  })

  it('doRefresh skips the /token fetch entirely without the ctech_auth hint cookie', async () => {
    // The common case this guards: a first visit with no session at all —
    // firing the request anyway would be a guaranteed 400 against the shared
    // IdP rate limit.
    const result = await doRefresh()

    expect(result).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('doRefresh skips the /token fetch once revoked, even with the hint cookie present', async () => {
    setAuthHintCookie()
    await revokeToken() // marks revoked locally; /revoke fetch is best-effort
    fetchMock.mockClear()

    const result = await doRefresh()

    expect(result).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('a rejected refresh marks the session revoked (subsequent doRefresh short-circuits)', async () => {
    setAuthHintCookie()
    fetchMock.mockResolvedValue({ok: false, status: 400})

    expect(await doRefresh()).toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(1)

    fetchMock.mockClear()
    expect(await doRefresh()).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('a successful refresh returns the access token', async () => {
    setAuthHintCookie()
    fetchMock.mockResolvedValue({ok: true, json: async () => ({access_token: 'at'})})

    const result = await doRefresh()

    expect(result).toEqual({accessToken: 'at'})
  })

  it('a network error during refresh does not mark revoked (may be transient)', async () => {
    setAuthHintCookie()
    fetchMock.mockRejectedValueOnce(new Error('network'))
    await expect(doRefresh()).rejects.toMatchObject({name: 'OAuthTransientError'})

    fetchMock.mockResolvedValueOnce({ok: true, json: async () => ({access_token: 'at2'})})
    expect(await doRefresh()).toEqual({accessToken: 'at2'})
  })
})
