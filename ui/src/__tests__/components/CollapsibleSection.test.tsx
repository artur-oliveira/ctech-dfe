import {describe, it, expect} from 'vitest'
import {render, screen, fireEvent} from '@testing-library/react'
import {CollapsibleSection} from '@/components/ui/collapsible-section'

describe('CollapsibleSection', () => {
  it('is collapsed by default and expands on header click', () => {
    render(
      <CollapsibleSection title="Configurações avançadas">
        <p>Conteúdo avançado</p>
      </CollapsibleSection>,
    )

    const button = screen.getByRole('button', {name: /Configurações avançadas/i})
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Conteúdo avançado')).not.toBeInTheDocument()

    fireEvent.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Conteúdo avançado')).toBeInTheDocument()
  })

  it('respects defaultOpen', () => {
    render(
      <CollapsibleSection title="Aberto" defaultOpen>
        <p>Visível</p>
      </CollapsibleSection>,
    )
    expect(screen.getByRole('button', {name: /Aberto/i})).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Visível')).toBeInTheDocument()
  })
})
