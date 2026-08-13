import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {describe, expect, it, vi, beforeEach} from 'vitest'
import NfesPage from '@/app/nfe/page'
import {apiClient} from '@/lib/api/client'
import type {NFeConfigOut} from '@/lib/types/api'

const VALID_KEY = '35250512345678000195550010000000011000000015'

vi.mock('next/navigation', () => ({
  useRouter: () => ({replace: vi.fn(), push: vi.fn()}),
  useSearchParams: () => new URLSearchParams('tab=distribuicao'),
}))

vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => children}))
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => children}))
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_11222333000181'}})}))

function emptyHistory() {
  return {items: [], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false}
}

function renderPage() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return render(
    <QueryClientProvider client={client}>
      <NfesPage/>
    </QueryClientProvider>,
  )
}

describe('NfeDistributionTab — Importar NF-e', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(apiClient, 'getNFeConfig').mockResolvedValue({environment: 2} as NFeConfigOut)
    vi.spyOn(apiClient, 'listDistributions').mockResolvedValue(emptyHistory())
  })

  it('desabilita o envio enquanto a chave é inválida', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', {name: 'Importar NF-e'}))
    const input = await screen.findByLabelText('Chave de acesso')
    await user.type(input, '3525051234567800')

    expect(await screen.findByRole('button', {name: 'Importar'})).toBeDisabled()
  })

  it('mostra o campo específico que falhou', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', {name: 'Importar NF-e'}))
    const input = await screen.findByLabelText('Chave de acesso')
    await user.type(input, '99' + VALID_KEY.slice(2))

    expect(await screen.findByText('Código da UF (cUF) inválido')).toBeInTheDocument()
  })

  it('chama importNfeByKey com a chave desformatada ao submeter uma chave válida', async () => {
    const importSpy = vi.spyOn(apiClient, 'importNfeByKey').mockResolvedValue({status: 'enqueued'})
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', {name: 'Importar NF-e'}))
    const input = await screen.findByLabelText('Chave de acesso')
    await user.type(input, VALID_KEY)

    const submit = await screen.findByRole('button', {name: 'Importar'})
    expect(submit).not.toBeDisabled()
    await user.click(submit)

    await waitFor(() => expect(importSpy).toHaveBeenCalledWith(VALID_KEY))
  })
})
