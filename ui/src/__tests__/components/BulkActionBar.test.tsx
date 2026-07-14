import {describe, it, expect, vi} from 'vitest'
import {render, screen} from '@testing-library/react'
import {BulkActionBar} from '@/components/ui/bulk-action-bar'

describe('BulkActionBar', () => {
  it('renders nothing when count is 0', () => {
    const {container} = render(
      <BulkActionBar count={0} onClear={() => {}}><button>Excluir</button></BulkActionBar>,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('shows the count, action button, and clear handler', () => {
    const onClear = vi.fn()
    render(
      <BulkActionBar count={3} onClear={onClear}>
        <button>Excluir</button>
      </BulkActionBar>,
    )
    expect(screen.getByText('3 selecionados')).toBeInTheDocument()
    expect(screen.getByRole('button', {name: 'Excluir'})).toBeInTheDocument()
    screen.getByRole('button', {name: 'Limpar'}).click()
    expect(onClear).toHaveBeenCalledOnce()
  })
})
