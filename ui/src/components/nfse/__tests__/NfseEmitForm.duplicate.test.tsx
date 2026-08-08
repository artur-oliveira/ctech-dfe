import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {render, screen, waitFor} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {NfseEmitForm} from '@/components/nfse/NfseEmitForm'
import {apiClient} from '@/lib/api/client'

vi.mock('next/navigation', () => ({useRouter: () => ({push: vi.fn()})}))
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_ORG'}})}))

describe('duplicação de NFS-e', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
    vi.spyOn(apiClient, 'getNfseConfig').mockResolvedValue({provider: 'nacional', environment: 1, serie: '1'} as never)
    vi.spyOn(apiClient, 'getOrganization').mockResolvedValue({
      pk: 'CNPJ_ORG', name: 'Empresa Teste', person: {},
      created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
    } as never)
    vi.spyOn(apiClient, 'getNfses').mockResolvedValue({
      items: [], next_cursor: null, previous_cursor: null, has_next: false, has_previous: false,
    })
    vi.spyOn(apiClient, 'getNfse').mockResolvedValue({
      sk: 'DPS_ORIGINAL', number: 42, competence: '2026-01-31', emit_input: {
        tp_emit: 1,
        customer_id: 'CNPJ_ORG',
        service: {service_id: 'SERVICE_1', description: 'Consultoria mensal', value: '1500.00', tax_rate: '5.00'},
        additional_info: 'Contrato mensal',
      },
    } as never)
    vi.spyOn(apiClient, 'getService').mockResolvedValue({
      sk: 'SERVICE_1', code: 'CONSULT', description: 'Consultoria', value: '1200.00',
      trib_nacional_code: '010101', trib_municipal_code: null, iss: {tax_rate: '3.00', trib_issqn: 1},
    } as never)
  })

  it('restaura IDs e overrides e avança a data civil em um mês', async () => {
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}})
    render(
      <QueryClientProvider client={client}>
        <NfseEmitForm mode="duplicate" sourceIdDps="DPS_ORIGINAL"/>
      </QueryClientProvider>,
    )

    expect(await screen.findByText(/Cópia da NFS-e nº 42/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByLabelText('Data de competência')).toHaveValue('2026-02-28'))
    expect(screen.getByLabelText('Valor do serviço')).toHaveValue('1.500,00')
    expect(apiClient.getService).toHaveBeenCalledWith('SERVICE_1')
  })
})
