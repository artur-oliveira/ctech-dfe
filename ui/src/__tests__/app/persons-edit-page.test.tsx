import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {render, screen, waitFor} from '@testing-library/react'
import {describe, expect, it, vi} from 'vitest'
import EditPersonPage from '@/app/persons/edit/page'
import {apiClient} from '@/lib/api/client'
import type {PersonCreate, PersonItemOut} from '@/lib/types/api'

vi.mock('next/navigation', () => ({
  useRouter: () => ({push: vi.fn()}),
  useSearchParams: () => new URLSearchParams('id=12345678909'),
}))

vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => children}))
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => children}))
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_11222333000181'}})}))

// O formulário real só entra no caminho para devolver o payload; o que está sob
// teste é a página repassá-lo inteiro para o PUT.
const PAYLOAD: PersonCreate = {
  cpf_or_cnpj: '12345678909',
  name: 'PESSOA TESTE',
  roles: ['customer', 'carrier'],
  person: {} as PersonCreate['person'],
}
vi.mock('@/components/persons/PersonForm', () => ({
  PersonForm: ({onSubmit}: {onSubmit: (d: PersonCreate) => Promise<void>}) => (
    <button onClick={() => void onSubmit(PAYLOAD)}>salvar</button>
  ),
}))

describe('página de edição de pessoa', () => {
  it('envia os papéis no PUT — omiti-los apagaria os papéis existentes', async () => {
    vi.spyOn(apiClient, 'getPerson').mockResolvedValue({
      pk: 'CNPJ_11222333000181',
      sk: 'CPF_12345678909',
      name: 'PESSOA TESTE',
      roles: ['customer'],
      person: {},
      created_at: '2026-08-07T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    } as PersonItemOut)
    const update = vi.spyOn(apiClient, 'updatePerson').mockResolvedValue({} as PersonItemOut)
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}})

    render(
      <QueryClientProvider client={client}>
        <EditPersonPage/>
      </QueryClientProvider>,
    )

    const save = await screen.findByRole('button', {name: 'salvar'})
    save.click()

    await waitFor(() => expect(update).toHaveBeenCalledWith('12345678909', expect.objectContaining({
      roles: ['customer', 'carrier'],
    })))
  })
})
