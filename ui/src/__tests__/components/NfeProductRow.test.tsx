import {describe, it, expect, vi} from 'vitest'
import {render, screen} from '@testing-library/react'
import {ProductRow} from '@/components/nfe/NfeEmitForm'
import type {ProductOut} from '@/lib/types/api'

const product = {
  sk: 'PRODUCT_1', description: 'Vasilhame', cfop_config: [
    {cfop: '5405'}, {cfop: '5920'}, {cfop: '6920'},
  ],
} as unknown as ProductOut

const baseItem = {product, cfop: '5920', cfopSuffix: '920', qty: '1', unitValue: '10', discount: '0'}

describe('ProductRow CFOP grouping', () => {
  it('shows block message when inter variant is missing for other UF', () => {
    const onChange = vi.fn()
    const item = {...baseItem, cfop: '', cfopSuffix: '405'}
    render(<ProductRow item={item} index={0} sameUf={false} onChange={onChange} onRemove={() => {}}/>)
    expect(screen.getByText(/Configure o CFOP 6xxx/)).toBeInTheDocument()
  })

  it('blocks with a UF-unknown message when destination UF is not resolved', () => {
    const onChange = vi.fn()
    render(<ProductRow item={baseItem} index={0} sameUf={null} onChange={onChange} onRemove={() => {}}/>)
    expect(screen.getByText(/Selecione um destinatário com UF/)).toBeInTheDocument()
  })
})
