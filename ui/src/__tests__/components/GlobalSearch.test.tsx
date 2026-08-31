import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, screen, fireEvent} from '@testing-library/react'
import {GlobalSearch} from '@/components/layout/GlobalSearch'

const push = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({push}),
}))

// USER não enxerga Assinatura — o índice respeita o mesmo filtro da barra lateral.
const selectedOrg = {role: 'USER'}
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg}),
}))

describe('GlobalSearch', () => {
  beforeEach(() => {
    push.mockClear()
  })

  const type = (value: string) =>
    fireEvent.change(screen.getByLabelText('Buscar páginas e cadastros'), {target: {value}})

  it('renders nothing while closed', () => {
    const {container} = render(<GlobalSearch open={false} onClose={() => {}}/>)
    expect(container).toBeEmptyDOMElement()
  })

  it('finds a page by its label', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    type('veículos')
    expect(screen.getByText('Veículos')).toBeInTheDocument()
  })

  it('finds a page by a keyword that is not in the label', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    type('placa')
    expect(screen.getByText('Veículos')).toBeInTheDocument()
  })

  it('tolerates typos (fuzzy match)', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    type('certificad')
    expect(screen.getByText('Certificados')).toBeInTheDocument()
  })

  it('shows the document context of each result', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    type('serviços')
    expect(screen.getByText('NFS-e · Cadastros')).toBeInTheDocument()
  })

  it('hides entries the member role cannot access', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    type('assinatura')
    expect(screen.queryByText('Assinatura')).not.toBeInTheDocument()
  })

  it('navigates on Enter and closes', () => {
    const onClose = vi.fn()
    render(<GlobalSearch open onClose={onClose}/>)
    type('perfis fiscais')
    fireEvent.keyDown(screen.getByRole('dialog'), {key: 'Enter'})
    expect(push).toHaveBeenCalledWith('/tax-profiles')
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('moves the cursor with the arrow keys', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    const dialog = screen.getByRole('dialog')
    fireEvent.keyDown(dialog, {key: 'ArrowDown'})
    fireEvent.keyDown(dialog, {key: 'Enter'})
    expect(push).toHaveBeenCalledWith('/guide')
  })

  it('reports an empty result set', () => {
    render(<GlobalSearch open onClose={() => {}}/>)
    type('zzzzzzzz')
    expect(screen.getByText(/Nada encontrado/)).toBeInTheDocument()
  })

  it('closes on Escape', () => {
    const onClose = vi.fn()
    render(<GlobalSearch open onClose={onClose}/>)
    fireEvent.keyDown(screen.getByRole('dialog'), {key: 'Escape'})
    expect(onClose).toHaveBeenCalledOnce()
  })
})
