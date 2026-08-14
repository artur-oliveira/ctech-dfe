import {describe, it, expect, beforeEach, vi} from 'vitest'
import {useContext} from 'react'
import {renderHook, act, waitFor} from '@testing-library/react'
import {AuthProvider, AuthContext} from '@/lib/context/AuthContext'
import {STORAGE_KEY_USER} from '@/lib/constants/storage'
import type {MeResponse} from '@/lib/types/api'

const meMock = vi.fn<() => Promise<MeResponse>>()

vi.mock('@/lib/api/client', () => ({
  apiClient: {me: () => meMock(), setToken: vi.fn()},
  registerRefreshFn: vi.fn(),
  ApiError: class ApiError extends Error {
    constructor(public readonly status: number, detail: string) {
      super(detail)
    }
  },
}))

// base64url(JSON) JWT payload with the given claims.
function makeToken(claims: Record<string, unknown>): string {
  const bytes = new TextEncoder().encode(JSON.stringify(claims))
  let bin = ''
  bytes.forEach((b) => (bin += String.fromCharCode(b)))
  const payload = btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
  return `header.${payload}.signature`
}

const backendMe = (overrides: Partial<MeResponse> = {}): MeResponse => ({
  user_id: 'u1',
  username: 'stale.user',
  email: 'a@b.com',
  first_name: 'Stale',
  last_name: 'Old',
  email_verified: true,
  is_enabled: true,
  terms_addendum_accepted: true,
  last_login_at: null,
  organizations: [],
  ...overrides,
})

function renderAuth() {
  return renderHook(() => useContext(AuthContext)!, {
    wrapper: ({children}) => <AuthProvider>{children}</AuthProvider>,
  })
}

describe('AuthContext name merge from id_token', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    meMock.mockReset()
  })

  it('overrides backend name with fresh id_token claims on login', async () => {
    meMock.mockResolvedValue(backendMe())
    const idToken = makeToken({preferred_username: 'fresh.user', given_name: 'Fresh', family_name: 'Name'})

    const {result} = renderAuth()
    await act(async () => {
      await result.current.handleCallback('at', idToken)
    })

    await waitFor(() => expect(result.current.user?.first_name).toBe('Fresh'))
    expect(result.current.user?.last_name).toBe('Name')
    expect(result.current.user?.username).toBe('fresh.user')
  })

  it('preserves the id_token name on background refresh (no new id_token)', async () => {
    meMock.mockResolvedValue(backendMe())
    const idToken = makeToken({preferred_username: 'fresh.user', given_name: 'Fresh', family_name: 'Name'})

    const {result} = renderAuth()
    await act(async () => {
      await result.current.handleCallback('at', idToken)
    })
    await waitFor(() => expect(result.current.user?.first_name).toBe('Fresh'))

    // Later /auth/me returns fresh orgs but the same stale backend name.
    meMock.mockResolvedValue(backendMe({email: 'new@b.com'}))
    await act(async () => {
      await result.current.refreshUser()
    })

    // Name stays from the id_token; other fields refresh from the backend.
    expect(result.current.user?.first_name).toBe('Fresh')
    expect(result.current.user?.username).toBe('fresh.user')
    expect(result.current.user?.email).toBe('new@b.com')
  })

  it('falls back to backend name when no id_token is present', async () => {
    meMock.mockResolvedValue(backendMe())

    const {result} = renderAuth()
    await act(async () => {
      await result.current.handleCallback('at', null)
    })

    await waitFor(() => expect(result.current.user).not.toBeNull())
    expect(result.current.user?.first_name).toBe('Stale')
    expect(result.current.user?.username).toBe('stale.user')
  })

  // Regression: after creating an org, the new-org page needs refreshUser to return
  // the fresh /auth/me so it can select the just-created org as active by pk.
  it('refreshUser resolves to the fetched user so a new org can be selected active', async () => {
    const newOrg = {pk: 'org-new', name: 'Nova', description: null, role: 'owner', permissions: [], state_federation: null}
    meMock.mockResolvedValue(backendMe({organizations: [newOrg]}))

    const {result} = renderAuth()
    let me: MeResponse | null = null
    await act(async () => {
      me = await result.current.refreshUser()
    })

    expect((me as unknown as MeResponse)?.organizations.find((o) => o.pk === 'org-new')).toBeTruthy()

    act(() => result.current.setSelectedOrg(newOrg))
    expect(result.current.selectedOrg?.pk).toBe('org-new')
  })

  it('persists the merged name to localStorage', async () => {
    meMock.mockResolvedValue(backendMe())
    const idToken = makeToken({given_name: 'Fresh', family_name: 'Name'})

    const {result} = renderAuth()
    await act(async () => {
      await result.current.handleCallback('at', idToken)
    })

    await waitFor(() => expect(result.current.user?.first_name).toBe('Fresh'))
    const stored: MeResponse = JSON.parse(localStorage.getItem(STORAGE_KEY_USER)!)
    expect(stored.first_name).toBe('Fresh')
  })
})
