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

  it('permite escolher uma variante específica de CFOP (ex.: só o 6101), sem juntar o grupo inteiro', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TaxProfileForm initialData={initialData} onSubmit={onSubmit}/>)

    fireEvent.click(screen.getAllByRole('combobox')[0])
    fireEvent.change(screen.getByPlaceholderText('Código ou descrição...'), {target: {value: '6101'}})
    fireEvent.click(screen.getByRole('option', {name: /6101\(interestadual\)/}))

    expect(screen.getByText('6101')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', {name: 'Salvar perfil'}))

    await waitFor(() => expect(onSubmit).toHaveBeenCalled())
    const payload = onSubmit.mock.calls[0][0]
    expect(payload.cfops).toEqual(['5102', '6101'])

    fireEvent.click(screen.getByRole('button', {name: 'Remover CFOP 6101'}))
    expect(screen.queryByText('6101')).not.toBeInTheDocument()
    expect(screen.getByText('5102')).toBeInTheDocument()
  })

  it('agrupa em um chip só quando as variantes de um mesmo CFOP são todas adicionadas', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TaxProfileForm initialData={initialData} onSubmit={onSubmit}/>)

    for (const variant of ['5101', '6101', '7101']) {
      fireEvent.click(screen.getAllByRole('combobox')[0])
      fireEvent.change(screen.getByPlaceholderText('Código ou descrição...'), {target: {value: variant}})
      fireEvent.click(screen.getByRole('option', {name: new RegExp(`^${variant}`)}))
    }

    expect(screen.getByText('5101/6101/7101')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', {name: 'Salvar perfil'}))
    await waitFor(() => expect(onSubmit).toHaveBeenCalled())
    expect(onSubmit.mock.calls[0][0].cfops).toEqual(['5102', '5101', '6101', '7101'])
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
