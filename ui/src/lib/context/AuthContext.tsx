'use client'

import {createContext, ReactNode, useCallback, useEffect, useRef, useState} from 'react'
import {apiClient, ApiError, registerRefreshFn} from '@/lib/api/client'
import type {MeResponse, UserOrganization} from '@/lib/types/api'
import {SESSION_KEY_REFRESH, STORAGE_KEY_USER, STORAGE_KEY_ORG} from '@/lib/constants/storage'
import {decodeIdToken, doRefresh, IdTokenClaims, revokeToken, startOAuthFlow} from '@/lib/auth/oauth'

interface AuthContextType {
  user: MeResponse | null
  loading: boolean
  selectedOrg: UserOrganization | null
  setSelectedOrg: (org: UserOrganization | null) => void
  login: () => void
  logout: () => void
  refreshUser: () => Promise<boolean>
  handleCallback: (accessToken: string, refreshToken: string, idToken: string | null) => Promise<void>
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined)

/** Reads the name claims persisted in the cached user, or null if none is cached. */
function cachedNameClaims(): IdTokenClaims | null {
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
function applyNameClaims(data: MeResponse, claims?: IdTokenClaims | null): MeResponse {
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
  const refreshTokenRef = useRef<string | null>(null)

  const setSelectedOrg = useCallback((org: UserOrganization | null) => {
    setSelectedOrgState(org)
    if (org) {
      localStorage.setItem(STORAGE_KEY_ORG, JSON.stringify(org))
    } else {
      localStorage.removeItem(STORAGE_KEY_ORG)
    }
  }, [])

  const refreshUser = useCallback(async (nameClaims?: IdTokenClaims | null): Promise<boolean> => {
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
      return true
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUser(null)
        localStorage.removeItem(STORAGE_KEY_USER)
      }
      return false
    }
  }, [setSelectedOrg])

  const tryRefresh = useCallback(async (): Promise<string | null> => {
    const rt = refreshTokenRef.current ?? sessionStorage.getItem(SESSION_KEY_REFRESH)
    if (!rt) return null

    const result = await doRefresh(rt)
    if (!result) {
      refreshTokenRef.current = null
      sessionStorage.removeItem(SESSION_KEY_REFRESH)
      return null
    }

    refreshTokenRef.current = result.refreshToken
    sessionStorage.setItem(SESSION_KEY_REFRESH, result.refreshToken)
    apiClient.setToken(result.accessToken)
    return result.accessToken
  }, [])

  // Register the refresh function so client.ts can call it on 401.
  useEffect(() => {
    registerRefreshFn(tryRefresh)
  }, [tryRefresh])

  const login = useCallback(() => {
    void startOAuthFlow(window.location.pathname)
  }, [])

  const logout = useCallback(() => {
    const rt = refreshTokenRef.current
    refreshTokenRef.current = null
    sessionStorage.removeItem(SESSION_KEY_REFRESH)
    apiClient.setToken(null)
    setUser(null)
    setSelectedOrgState(null)
    localStorage.removeItem(STORAGE_KEY_USER)
    localStorage.removeItem(STORAGE_KEY_ORG)
    if (rt) void revokeToken(rt)
  }, [])

  const handleCallback = useCallback(async (accessToken: string, refreshToken: string, idToken: string | null) => {
    refreshTokenRef.current = refreshToken
    sessionStorage.setItem(SESSION_KEY_REFRESH, refreshToken)
    apiClient.setToken(accessToken)
    const nameClaims = idToken ? decodeIdToken(idToken) : null
    const ok = await refreshUser(nameClaims)
    if (!ok) throw new Error('Falha ao obter dados do usuário após autenticação. Verifique a conexão com o servidor.')
  }, [refreshUser])

  // On mount: attempt silent refresh from stored refresh token.
  useEffect(() => {
    void (async () => {
      const rt = sessionStorage.getItem(SESSION_KEY_REFRESH)
      if (!rt) {
        // Check for cached user — if present, we had a session before; attempt refresh.
        const cached = localStorage.getItem(STORAGE_KEY_USER)
        if (cached) {
          localStorage.removeItem(STORAGE_KEY_USER)
        }
        setLoading(false)
        return
      }

      const newToken = await (async () => {
        const result = await doRefresh(rt)
        if (!result) return null
        refreshTokenRef.current = result.refreshToken
        sessionStorage.setItem(SESSION_KEY_REFRESH, result.refreshToken)
        apiClient.setToken(result.accessToken)
        return result.accessToken
      })()

      if (newToken) {
        const cached = localStorage.getItem(STORAGE_KEY_USER)
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
      } else {
        refreshTokenRef.current = null
        sessionStorage.removeItem(SESSION_KEY_REFRESH)
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
