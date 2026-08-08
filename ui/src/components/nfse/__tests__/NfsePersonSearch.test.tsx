import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {fireEvent, render, screen, waitFor} from '@testing-library/react'
import {describe, expect, it, vi} from 'vitest'
import {NfsePersonSearch} from '@/components/nfse/NfsePersonSearch'
import {apiClient} from '@/lib/api/client'
import type {PersonItemOut} from '@/lib/types/api'

vi.mock('@/components/persons/PersonForm', () => ({PersonForm: () => null}))

const PERSON = {
  pk: 'CPF_12345678909',
  sk: 'CPF_12345678909',
  name: 'Pessoa Teste',
  person: {},
  created_at: '2026-08-07T00:00:00Z',
  updated_at: '2026-08-07T00:00:00Z',
} as PersonItemOut

describe('NfsePersonSearch', () => {
  it('busca CPF automaticamente após o debounce, sem botão Buscar', async () => {
    const getPerson = vi.spyOn(apiClient, 'getPersonByCpfCnpj').mockResolvedValue(PERSON)
    const onChange = vi.fn()
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}})

    render(
      <QueryClientProvider client={client}>
        <NfsePersonSearch value={null} onChange={onChange}/>
      </QueryClientProvider>,
    )

    fireEvent.change(screen.getByRole('combobox'), {target: {value: '123.456.789-09'}})
    expect(screen.queryByRole('button', {name: 'Buscar'})).not.toBeInTheDocument()
    expect(getPerson).not.toHaveBeenCalled()

    await waitFor(() => expect(getPerson).toHaveBeenCalledWith('12345678909'))
    await waitFor(() => expect(onChange).toHaveBeenCalledWith(PERSON))
  })
})
