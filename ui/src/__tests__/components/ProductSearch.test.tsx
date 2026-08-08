import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, screen, fireEvent, waitFor} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {ProductSearch} from '@/components/ui/product-search'
import type {ProductOut} from '@/lib/types/api'

const products = [
  {sk: 'PRODUCT_1', code: '001', description: 'Coca-Cola 2L', value: '12.50', cfop_config: [{cfop: '5102'}]},
  {sk: 'PRODUCT_2', code: '002', description: 'Pão de forma', value: '9.90', cfop_config: []},
] as unknown as ProductOut[]

const getProducts = vi.fn()

vi.mock('@/lib/api/client', () => ({
  apiClient: {getProducts: (...args: unknown[]) => getProducts(...args)},
}))

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'CNPJ_1'}}),
}))

function renderSearch(props: Partial<React.ComponentProps<typeof ProductSearch>> = {}) {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}})
  const onSelect = vi.fn()
  render(
    <QueryClientProvider client={client}>
      <ProductSearch onSelect={onSelect} {...props}/>
    </QueryClientProvider>,
  )
  return {onSelect}
}

describe('ProductSearch', () => {
  beforeEach(() => {
    getProducts.mockReset()
    getProducts.mockResolvedValue({items: products})
  })

  it('adds the highlighted product when Enter is pressed — the barcode path', async () => {
    const {onSelect} = renderSearch()
    await screen.findByText('Coca-Cola 2L')

    fireEvent.keyDown(screen.getByLabelText('Buscar produto'), {key: 'Enter'})

    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({sk: 'PRODUCT_1'}))
  })

  it('moves the highlight with the arrow keys before adding', async () => {
    const {onSelect} = renderSearch()
    await screen.findByText('Pão de forma')
    const input = screen.getByLabelText('Buscar produto')

    fireEvent.keyDown(input, {key: 'ArrowDown'})
    fireEvent.keyDown(input, {key: 'Enter'})

    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({sk: 'PRODUCT_2'}))
  })

  it('never selects a product the document cannot use, by key or by click', async () => {
    const {onSelect} = renderSearch({
      disabledReason: (p) => (p.sk === 'PRODUCT_2' ? 'sem CFOP de NFC-e' : null),
    })
    await screen.findByText('sem CFOP de NFC-e')
    const input = screen.getByLabelText('Buscar produto')

    // ArrowDown cannot move past the only selectable row.
    fireEvent.keyDown(input, {key: 'ArrowDown'})
    fireEvent.keyDown(input, {key: 'Enter'})
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({sk: 'PRODUCT_1'}))

    fireEvent.click(screen.getByText(/Pão de forma/))
    expect(onSelect).toHaveBeenCalledTimes(1)
  })

  it('clears the query after a selection so the next scan starts clean', async () => {
    renderSearch()
    await screen.findByText('Coca-Cola 2L')
    const input = screen.getByLabelText('Buscar produto') as HTMLInputElement

    fireEvent.change(input, {target: {value: 'Coca'}})
    expect(input.value).toBe('Coca')

    fireEvent.keyDown(input, {key: 'Enter'})

    await waitFor(() => expect(input.value).toBe(''))
  })

  it('closes on Escape only when the panel owns a close action', async () => {
    const onClose = vi.fn()
    renderSearch({onClose})
    await screen.findByText('Coca-Cola 2L')

    fireEvent.keyDown(screen.getByLabelText('Buscar produto'), {key: 'Escape'})

    expect(onClose).toHaveBeenCalledOnce()
  })
})
