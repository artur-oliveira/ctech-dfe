import {describe, it, expect, vi, beforeEach} from 'vitest'
import {renderHook, waitFor} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import type {ReactNode} from 'react'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {apiClient, ApiError} from '@/lib/api/client'

function wrapper({children}: { children: ReactNode }) {
  const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useFiscalConfig', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('treats a 404 as "not configured" instead of a query error', async () => {
    vi.spyOn(apiClient, 'getNFeConfig').mockRejectedValue(new ApiError(404, 'not found'))

    const {result} = renderHook(() => useFiscalConfig('nfe', 'CNPJ_TEST'), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.isMissing).toBe(true)
    expect(result.current.config).toBeNull()
    expect(result.current.error).toBeNull()
  })

  it('returns the config when it exists', async () => {
    vi.spyOn(apiClient, 'getNFeConfig').mockResolvedValue({environment: 2} as never)

    const {result} = renderHook(() => useFiscalConfig('nfe', 'CNPJ_TEST'), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.isMissing).toBe(false)
    expect(result.current.config).toEqual({environment: 2})
  })

  it('propagates non-404 errors instead of treating them as unconfigured', async () => {
    vi.spyOn(apiClient, 'getNFeConfig').mockRejectedValue(new ApiError(500, 'server error'))

    const {result} = renderHook(() => useFiscalConfig('nfe', 'CNPJ_TEST'), {wrapper})

    await waitFor(() => expect(result.current.isPending).toBe(false))
    expect(result.current.isMissing).toBe(false)
    expect(result.current.error).toBeTruthy()
  })
})
