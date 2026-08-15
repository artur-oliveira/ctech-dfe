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

  it('remover um CFOP não afeta outro CFOP do mesmo grupo adicionado separadamente', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<TaxProfileForm initialData={initialData} onSubmit={onSubmit}/>)

    for (const variant of ['5920', '6920']) {
      fireEvent.click(screen.getAllByRole('combobox')[0])
      fireEvent.change(screen.getByPlaceholderText('Código ou descrição...'), {target: {value: variant}})
      fireEvent.click(screen.getByRole('option', {name: new RegExp(`^${variant}`)}))
    }

    expect(screen.getByText('5920')).toBeInTheDocument()
    expect(screen.getByText('6920')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', {name: 'Remover CFOP 5920'}))
    expect(screen.queryByText('5920')).not.toBeInTheDocument()
    expect(screen.getByText('6920')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', {name: 'Salvar perfil'}))
    await waitFor(() => expect(onSubmit).toHaveBeenCalled())
    expect(onSubmit.mock.calls[0][0].cfops).toEqual(['5102', '6920'])
  })

  it('reabre o toggle IBS/CBS ao editar um perfil que já tem ibs_cbs_cst preenchido', () => {
    render(<TaxProfileForm initialData={{...initialData, ibs_cbs_cst: '410'} as unknown as TaxProfileItemOut}
                           onSubmit={vi.fn()}/>)
    expect(screen.getByRole('checkbox', {name: /IBS \/ CBS/})).toBeChecked()
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
