import type {ReactNode} from 'react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {render, screen, waitFor} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import NfseEmitPage from '@/app/nfse/emit/page'
import {NfseServicePicker} from '@/components/nfse/NfseServicePicker'
import {apiClient} from '@/lib/api/client'

let searchParams = new URLSearchParams()

vi.mock('next/navigation', () => ({
  useSearchParams: () => searchParams,
}))

vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({children}: { children: ReactNode }) => <>{children}</>,
}))

vi.mock('@/components/layout/RootLayout', () => ({
  RootLayout: ({children}: { children: ReactNode }) => <>{children}</>,
}))

vi.mock('@/components/nfse/NfseEmitForm', () => ({
  NfseEmitForm: ({mode, sourceIdDps}: { mode: string; sourceIdDps?: string }) => (
    <div data-testid="nfse-form" data-mode={mode} data-source={sourceIdDps}/>
  ),
}))

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_TEST'}}),
}))

describe('emissão de NFS-e', () => {
  beforeEach(() => {
    searchParams = new URLSearchParams()
    vi.restoreAllMocks()
  })

  it('renderiza o modo de substituição a partir da query string', () => {
    searchParams = new URLSearchParams('substitute=DPS_TESTE')

    render(<NfseEmitPage/>)

    expect(screen.getByRole('heading', {name: 'Substituir NFS-e'})).toBeInTheDocument()
    expect(screen.getByTestId('nfse-form')).toHaveAttribute('data-mode', 'substitute')
    expect(screen.getByTestId('nfse-form')).toHaveAttribute('data-source', 'DPS_TESTE')
  })

  it('renderiza o modo de duplicação a partir da query string', () => {
    searchParams = new URLSearchParams('duplicate=DPS_ORIGINAL')

    render(<NfseEmitPage/>)

    expect(screen.getByRole('heading', {name: 'Duplicar NFS-e'})).toBeInTheDocument()
    expect(screen.getByTestId('nfse-form')).toHaveAttribute('data-mode', 'duplicate')
    expect(screen.getByTestId('nfse-form')).toHaveAttribute('data-source', 'DPS_ORIGINAL')
  })

  it('isola e pagina o catálogo do picker pela organização ativa', async () => {
    const getServices = vi.spyOn(apiClient, 'getServices')
      .mockResolvedValueOnce({
        items: [],
        next_cursor: 'PAGE_2',
        previous_cursor: null,
        has_next: true,
        has_previous: false,
      })
      .mockResolvedValueOnce({
        items: [],
        next_cursor: null,
        previous_cursor: null,
        has_next: false,
        has_previous: true,
      })
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}})

    render(
      <QueryClientProvider client={queryClient}>
        <NfseServicePicker onSelect={vi.fn()} onClear={vi.fn()}/>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(getServices).toHaveBeenCalledWith({limit: 100, cursor: undefined}))
    await waitFor(() => expect(getServices).toHaveBeenCalledWith({limit: 100, cursor: 'PAGE_2'}))
    expect(queryClient.getQueryState(['services', 'CNPJ_TEST'])).toBeDefined()
    expect(queryClient.getQueryState(['services', undefined])).toBeUndefined()
  })
})
