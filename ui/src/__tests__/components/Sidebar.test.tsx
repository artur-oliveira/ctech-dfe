import {describe, it, expect, vi, afterEach} from 'vitest'
import {render, screen, fireEvent, cleanup} from '@testing-library/react'
import {Sidebar} from '@/components/layout/Sidebar'

let pathname = '/dashboard'

vi.mock('next/navigation', () => ({
  usePathname: () => pathname,
}))

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {role: 'OWNER'}}),
}))

function renderAt(path: string) {
  pathname = path
  return render(<Sidebar open onClose={() => {}}/>)
}

describe('Sidebar', () => {
  afterEach(cleanup)

  it('keeps only shared registries in the global Cadastros group', () => {
    renderAt('/dashboard')
    expect(screen.getByRole('link', {name: 'Pessoas'})).toBeInTheDocument()
    expect(screen.getByRole('link', {name: 'Produtos'})).toBeInTheDocument()
    // Exclusivo de NFS-e: só aparece dentro do contexto.
    expect(screen.queryByRole('link', {name: 'Serviços'})).not.toBeInTheDocument()
  })

  it('expands the active document context and its registries', () => {
    renderAt('/nfse')
    expect(screen.getByRole('link', {name: 'Serviços'})).toHaveAttribute('href', '/services')
    expect(screen.getByRole('link', {name: 'Emitir NFS-e'})).toBeInTheDocument()
    // O contexto de outro documento continua recolhido.
    expect(screen.queryByRole('link', {name: 'Veículos'})).not.toBeInTheDocument()
  })

  it('stays in context on a registry route that belongs to the document', () => {
    renderAt('/services/new')
    expect(screen.getByRole('link', {name: 'Locais de prestação'})).toBeInTheDocument()
  })

  it('expands another context on the chevron without navigating', () => {
    renderAt('/dashboard')
    fireEvent.click(screen.getByRole('button', {name: 'Expandir MDF-e'}))
    expect(screen.getByRole('link', {name: 'Veículos'})).toHaveAttribute('href', '/vehicles')
  })

  it('marks the current page for assistive tech', () => {
    renderAt('/persons')
    expect(screen.getByRole('link', {name: 'Pessoas'})).toHaveAttribute('aria-current', 'page')
  })
})
