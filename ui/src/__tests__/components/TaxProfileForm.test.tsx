import {describe, expect, it, vi} from 'vitest'
import {render, screen, fireEvent, waitFor} from '@testing-library/react'
import {TaxProfileForm} from '@/components/tax-profiles/TaxProfileForm'
import type {TaxProfileItemOut} from '@/lib/types/api'

const initialData = {
  pk: 'CNPJ_1', sk: 'TAXPROFILE_1',
  name: 'Venda de mercadoria', description: '',
  cfops: ['5102'], pis: '01', cofins: '01',
  created_at: '2026-01-01', updated_at: '2026-01-01',
} as unknown as TaxProfileItemOut

describe('TaxProfileForm', () => {
  it('mostra o texto de ajuda mencionando overrides por UF', () => {
    render(<TaxProfileForm onSubmit={vi.fn()}/>)
    expect(screen.getByText(/overrides por UF/)).toBeInTheDocument()
  })

  it('inclui uf_overrides no submit ao marcar uma UF no editor de overrides', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TaxProfileForm initialData={initialData} onSubmit={onSubmit}/>)

    fireEvent.click(screen.getByText('+ Adicionar override por UF'))
    fireEvent.click(screen.getByRole('button', {name: 'RJ'}))
    fireEvent.click(screen.getByRole('button', {name: 'Salvar perfil'}))

    await waitFor(() => expect(onSubmit).toHaveBeenCalled())
    const payload = onSubmit.mock.calls[0][0]
    expect(payload.uf_overrides).toEqual([{ufs: ['RJ'], overrides: {}}])
  })
})
