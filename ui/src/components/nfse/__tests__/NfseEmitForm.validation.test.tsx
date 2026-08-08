import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {fireEvent, render, screen, waitFor} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {NfseEmitForm} from '@/components/nfse/NfseEmitForm'
import {apiClient} from '@/lib/api/client'
import type {OrganizationOut} from '@/lib/types/api'

const push = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({push}),
}))

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_TEST'}}),
}))

const organization = {
  pk: 'CNPJ_TEST',
  name: 'Empresa Teste',
  person: {},
  created_at: '2026-08-08T00:00:00Z',
  updated_at: '2026-08-08T00:00:00Z',
} as OrganizationOut

describe('validação do emissor NFS-e', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
    vi.spyOn(apiClient, 'getNfseConfig').mockResolvedValue({provider: 'nacional', environment: 1, serie: '1'} as never)
    vi.spyOn(apiClient, 'getOrganization').mockResolvedValue(organization)
    vi.spyOn(apiClient, 'getNfses').mockResolvedValue({
      items: [], next_cursor: null, previous_cursor: null, has_next: false, has_previous: false,
    })
    vi.spyOn(apiClient, 'getServices').mockResolvedValue({
      items: [], next_cursor: null, previous_cursor: null, has_next: false, has_previous: false,
    })
  })

  it('anuncia a validação e foca o primeiro campo inválido em vez de silenciar', async () => {
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}})
    render(
      <QueryClientProvider client={queryClient}>
        <NfseEmitForm/>
      </QueryClientProvider>,
    )

    fireEvent.click(await screen.findByRole('button', {name: 'Emitir NFS-e'}))
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}))

    expect(await screen.findByRole('alert')).toHaveTextContent('Revise o campo destacado')
    await waitFor(() => expect(document.getElementById('service.service_id')).toHaveFocus())
  })

  it('usa o Input ShadCN de data e mantém a competência em ISO Date', async () => {
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}})
    render(
      <QueryClientProvider client={queryClient}>
        <NfseEmitForm/>
      </QueryClientProvider>,
    )

    const competence = await screen.findByLabelText('Data de competência')
    expect(competence).toHaveAttribute('type', 'date')
    expect((competence as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})
