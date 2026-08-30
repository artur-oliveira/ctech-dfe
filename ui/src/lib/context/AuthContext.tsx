'use client'

import {createContext, ReactNode, useCallback, useEffect, useState} from 'react'
import {apiClient, ApiError, registerRefreshFn} from '@/lib/api/client'
import type {MeResponse, UserOrganization} from '@/lib/types/api'
import {STORAGE_KEY_USER, STORAGE_KEY_ORG} from '@/lib/constants/storage'
import {currentReturnTo, decodeIdToken, doRefresh, endSessionRedirect, UnverifiedIdTokenClaims, revokeToken, startOAuthFlow} from '@/lib/auth/oauth'
import {MOCK_ENABLED} from '@/lib/mock/env'
import {mockDoRefresh} from '@/lib/mock/auth'

interface AuthContextType {
  user: MeResponse | null
  loading: boolean
  selectedOrg: UserOrganization | null
  setSelectedOrg: (org: UserOrganization | null) => void
  login: () => void
  logout: (returnTo?: string) => void
  refreshUser: () => Promise<MeResponse | null>
  handleCallback: (accessToken: string, idToken: string | null) => Promise<void>
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined)

/** Reads the name claims persisted in the cached user, or null if none is cached. */
function cachedNameClaims(): UnverifiedIdTokenClaims | null {
  const cached = localStorage.getItem(STORAGE_KEY_USER)
  if (!cached) return null
  try {
    const parsed: MeResponse = JSON.parse(cached)
    return {username: parsed.username, first_name: parsed.first_name, last_name: parsed.last_name}
  } catch {
    return null
  }
}

/**
 * Overrides the name fields of a /auth/me response.
 * Priority: fresh id_token claims -> previously cached name -> backend fallback (data as-is).
 */
function applyNameClaims(data: MeResponse, claims?: UnverifiedIdTokenClaims | null): MeResponse {
  const src = claims ?? cachedNameClaims()
  if (!src) return data
  return {
    ...data,
    first_name: src.first_name ?? data.first_name,
    last_name: src.last_name ?? data.last_name,
    username: src.username ?? data.username,
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null)
  const [selectedOrg, setSelectedOrgState] = useState<UserOrganization | null>(null)
  const [loading, setLoading] = useState(true)

  const setSelectedOrg = useCallback((org: UserOrganization | null) => {
    setSelectedOrgState(org)
    if (org) {
      localStorage.setItem(STORAGE_KEY_ORG, JSON.stringify(org))
    } else {
      localStorage.removeItem(STORAGE_KEY_ORG)
    }
  }, [])

  const refreshUser = useCallback(async (nameClaims?: UnverifiedIdTokenClaims | null): Promise<MeResponse | null> => {
    try {
      const data = applyNameClaims(await apiClient.me(), nameClaims)
      setUser(data)
      localStorage.setItem(STORAGE_KEY_USER, JSON.stringify(data))

      const savedOrg = localStorage.getItem(STORAGE_KEY_ORG)
      if (savedOrg) {
        const org: UserOrganization = JSON.parse(savedOrg)
        const stillMember = data.organizations.find(o => o.pk === org.pk)
        setSelectedOrg(stillMember ?? data.organizations[0] ?? null)
      } else if (data.organizations.length > 0) {
        setSelectedOrg(data.organizations[0])
      }
      return data
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUser(null)
        localStorage.removeItem(STORAGE_KEY_USER)
      }
      return null
    }
  }, [setSelectedOrg])

  const tryRefresh = useCallback(async (): Promise<string | null> => {
    const result = await doRefresh()
    if (!result) return null
    apiClient.setToken(result.accessToken)
    return result.accessToken
  }, [])

  // Register the refresh function so client.ts can call it on 401.
  useEffect(() => {
    registerRefreshFn(tryRefresh)
  }, [tryRefresh])

  const login = useCallback(() => {
    void startOAuthFlow(currentReturnTo())
  }, [])

  const logout = useCallback(async (returnTo = '/'): Promise<void> => {
    // Clear local session immediately; UI reflects logged-out state right away.
    apiClient.setToken(null)
    setUser(null)
    setSelectedOrgState(null)
    localStorage.removeItem(STORAGE_KEY_USER)
    localStorage.removeItem(STORAGE_KEY_ORG)
    // Await /revoke so the end-session redirect below fires only after the
    // request settles — navigating while it's in flight starves/cancels end-session.
    await revokeToken()
    endSessionRedirect(returnTo)
  }, [])

  const handleCallback = useCallback(async (accessToken: string, idToken: string | null) => {
    apiClient.setToken(accessToken)
    const nameClaims = idToken ? decodeIdToken(idToken) : null
    const ok = await refreshUser(nameClaims)
    if (!ok) throw new Error('Falha ao obter dados do usuário após autenticação. Verifique a conexão com o servidor.')
  }, [refreshUser])

  // On mount: attempt a silent refresh from the HttpOnly ctech_rt cookie (M2).
  // The cookie is the source of truth — no refresh token is held in JS.
  useEffect(() => {
    void (async () => {
      const cached = localStorage.getItem(STORAGE_KEY_USER)
      const result = MOCK_ENABLED ? await mockDoRefresh() : await doRefresh()
      if (!result) {
        if (cached) localStorage.removeItem(STORAGE_KEY_USER)
        setLoading(false)
        return
      }

      apiClient.setToken(result.accessToken)
      if (cached) {
        const parsed: MeResponse = JSON.parse(cached)
        setUser(parsed)
        const savedOrg = localStorage.getItem(STORAGE_KEY_ORG)
        if (savedOrg) {
          setSelectedOrgState(JSON.parse(savedOrg))
        } else if (parsed.organizations.length > 0) {
          setSelectedOrg(parsed.organizations[0])
        }
        setLoading(false)
        void refreshUser()
      } else {
        await refreshUser()
        setLoading(false)
      }
    })()
  }, [refreshUser, setSelectedOrg])

  return (
    <AuthContext.Provider value={{ user, loading, selectedOrg, setSelectedOrg, login, logout, refreshUser, handleCallback }}>
      {children}
    </AuthContext.Provider>
  )
}
