import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {render, screen, waitFor} from '@testing-library/react'
import {describe, expect, it, vi, beforeEach} from 'vitest'
import type {ReactNode} from 'react'
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig'
import {apiClient, ApiError} from '@/lib/api/client'

const replaceFn = vi.fn()
vi.mock('next/navigation', () => ({
  useRouter: () => ({replace: replaceFn}),
}))

let selectedOrg: { pk: string } | null = {pk: 'CNPJ_TEST'}
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg})}))

function renderWithClient(children: ReactNode) {
  const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return render(<QueryClientProvider client={qc}>{children}</QueryClientProvider>)
}

describe('RequireFiscalConfig', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    replaceFn.mockClear()
    selectedOrg = {pk: 'CNPJ_TEST'}
  })

  it('renders children once the config is confirmed present', async () => {
    vi.spyOn(apiClient, 'getNFeConfig').mockResolvedValue({environment: 1} as never)

    renderWithClient(
      <RequireFiscalConfig variant="nfe">
        <div data-testid="emit-form"/>
      </RequireFiscalConfig>,
    )

    await waitFor(() => expect(screen.getByTestId('emit-form')).toBeInTheDocument())
    expect(replaceFn).not.toHaveBeenCalled()
  })

  it('redirects to the doc type config tab when config is missing, without rendering children', async () => {
    vi.spyOn(apiClient, 'getNFeConfig').mockRejectedValue(new ApiError(404, 'not found'))

    renderWithClient(
      <RequireFiscalConfig variant="nfe">
        <div data-testid="emit-form"/>
      </RequireFiscalConfig>,
    )

    await waitFor(() => expect(replaceFn).toHaveBeenCalledWith('/fiscal-config?tab=nfe'))
    expect(screen.queryByTestId('emit-form')).not.toBeInTheDocument()
  })

  it('shows the no-org banner instead of fetching config when no org is selected', () => {
    selectedOrg = null

    renderWithClient(
      <RequireFiscalConfig variant="nfe">
        <div data-testid="emit-form"/>
      </RequireFiscalConfig>,
    )

    expect(screen.queryByTestId('emit-form')).not.toBeInTheDocument()
    expect(replaceFn).not.toHaveBeenCalled()
  })
})
