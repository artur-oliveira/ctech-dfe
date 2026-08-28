import {render, screen} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {describe, expect, it, vi} from 'vitest'
import {EntityForm} from '@/components/EntityForm'

const resetLookup = vi.fn()
const lookup = vi.fn()

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: null}),
}))

vi.mock('@/lib/hooks/useCnpjLookup', () => ({
  useCnpjLookup: () => ({
    state: {status: 'idle', result: null, phase: null, currentUf: null, errorMessage: null},
    lookup,
    reset: resetLookup,
  }),
}))

vi.mock('@/components/ui/cnpj-lookup-badge', () => ({CnpjLookupBadge: () => null}))
vi.mock('@/components/ui/address-fields', () => ({AddressFields: () => <div>Campos do endereço</div>}))

describe('EntityForm — revelação por papel', () => {
  it('mantém o cadastro simples e revela os blocos operacionais relevantes', async () => {
    const user = userEvent.setup()
    render(<EntityForm variant="person" onSubmit={vi.fn()} />)

    expect(screen.queryByText('Recebimento do frete no MDF-e')).not.toBeInTheDocument()
    expect(screen.queryByText('Dados fiscais da NFS-e')).not.toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', {name: 'Transportadora'}))
    await user.click(screen.getByRole('button', {name: '+ Dados complementares e fiscais'}))

    expect(screen.getByText('Recebimento do frete no MDF-e')).toBeInTheDocument()
    expect(screen.getByText('ICMS retido sobre o frete')).toBeInTheDocument()
    expect(screen.queryByText('Dados fiscais da NFS-e')).not.toBeInTheDocument()

    await user.click(screen.getByRole('checkbox', {name: 'Prestador'}))
    expect(screen.getByText('Dados fiscais da NFS-e')).toBeInTheDocument()
  })

  it('marca os seletores PF/PJ como botões pressionáveis e força PF no cadastro estrangeiro', async () => {
    const user = userEvent.setup()
    render(<EntityForm variant="person" onSubmit={vi.fn()} />)

    expect(screen.getByRole('button', {name: 'Pessoa Jurídica'})).toHaveAttribute('aria-pressed', 'true')
    await user.click(screen.getByRole('checkbox', {name: 'Pessoa no exterior (sem CPF/CNPJ)'}))

    expect(screen.getByLabelText('Documento estrangeiro *')).toBeInTheDocument()
    expect(screen.getByLabelText('Nome *')).toBeInTheDocument()
  })
})
