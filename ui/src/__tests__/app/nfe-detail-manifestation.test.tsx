import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {render, screen, waitFor, within} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {describe, expect, it, vi, beforeEach} from 'vitest'
import NfeDetailPage from '@/app/nfe/detail/page'
import {apiClient} from '@/lib/api/client'
import type {NfeDetailOut, PaginatedResponse, NfeEventOut} from '@/lib/types/api'

const ACCESS_KEY = '35250512345678000195550010000000011000000015'

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(`key=${ACCESS_KEY}`),
}))

vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => children}))
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => children}))
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_11222333000181'}})}))

function baseDoc(overrides: Partial<NfeDetailOut> = {}): NfeDetailOut {
  return {
    pk: 'hom#CNPJ_11222333000181',
    sk: ACCESS_KEY,
    incoming: 1,
    year: 2026, month: 8, day: 12,
    status: 'authorized',
    sefaz_status: null, sefaz_motive: null,
    emit_cpf_cnpj: '11222333000181', emit_name: 'Fornecedor Teste',
    dest_cpf_cnpj: '12345678909', dest_name: 'Empresa Teste',
    number: 123, serie: 1, total: '100.00',
    dh_emi: null, created_at: '2026-08-12T00:00:00Z',
    products: [], payments: [], additional_info: null,
    xml_s3_key: null, sefaz_protocol: null,
    ...overrides,
  }
}

function emptyEvents(): PaginatedResponse<NfeEventOut> {
  return {items: [], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false}
}

function renderPage() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return render(
    <QueryClientProvider client={client}>
      <NfeDetailPage/>
    </QueryClientProvider>,
  )
}

describe('NfeDetail — manifestation e importação via distribuição', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('mostra o botão Manifestar apenas para NF-e destinada (incoming=1)', async () => {
    vi.spyOn(apiClient, 'getNfe').mockResolvedValue(baseDoc({incoming: 1}))
    vi.spyOn(apiClient, 'getNfeEvents').mockResolvedValue(emptyEvents())

    renderPage()

    expect(await screen.findByRole('button', {name: 'Manifestar'})).toBeInTheDocument()
  })

  it('oculta o botão Manifestar para NF-e própria (incoming=0)', async () => {
    vi.spyOn(apiClient, 'getNfe').mockResolvedValue(baseDoc({incoming: 0, status: 'authorized'}))
    vi.spyOn(apiClient, 'getNfeEvents').mockResolvedValue(emptyEvents())

    renderPage()

    await screen.findByText(`NF-e ${123}`)
    expect(screen.queryByRole('button', {name: 'Manifestar'})).not.toBeInTheDocument()
  })

  it('envia a manifestação com o tipo padrão e invalida as queries relevantes', async () => {
    vi.spyOn(apiClient, 'getNfe').mockResolvedValue(baseDoc({incoming: 1}))
    vi.spyOn(apiClient, 'getNfeEvents').mockResolvedValue(emptyEvents())
    const sendSpy = vi.spyOn(apiClient, 'sendManifestation').mockResolvedValue({} as NfeDetailOut)

    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', {name: 'Manifestar'}))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', {name: 'Manifestar'}))

    await waitFor(() => expect(sendSpy).toHaveBeenCalledWith(ACCESS_KEY, '210210', 1, undefined))
  })

  it('botão Importar via distribuição fica desabilitado quando a nota já está completa', async () => {
    vi.spyOn(apiClient, 'getNfe').mockResolvedValue(baseDoc({incoming: 1, products: [{
      product_id: 'PROD-1', product_code: 'P1',
      description: 'Produto', ncm: '00000000', cfop: '5102', unit: 'UN',
      quantity: '1', unit_value: '10.00', discount: '0', total: '10.00',
    }]}))
    vi.spyOn(apiClient, 'getNfeEvents').mockResolvedValue(emptyEvents())

    renderPage()

    const button = await screen.findByRole('button', {name: 'Importar via distribuição'})
    expect(button).toBeDisabled()
  })

  it('botão Importar via distribuição chama importNfeByKey quando a nota é só resumo', async () => {
    vi.spyOn(apiClient, 'getNfe').mockResolvedValue(baseDoc({incoming: 1, products: []}))
    vi.spyOn(apiClient, 'getNfeEvents').mockResolvedValue(emptyEvents())
    const importSpy = vi.spyOn(apiClient, 'importNfeByKey').mockResolvedValue({status: 'enqueued'})

    const user = userEvent.setup()
    renderPage()

    const button = await screen.findByRole('button', {name: 'Importar via distribuição'})
    expect(button).not.toBeDisabled()
    await user.click(button)

    await waitFor(() => expect(importSpy).toHaveBeenCalledWith(ACCESS_KEY))
  })
})
