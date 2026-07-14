import {describe, it, expect} from 'vitest'
import {render, screen} from '@testing-library/react'
import {TableShell, TABLE_CELL} from '@/components/ui/table-shell'

describe('TableShell', () => {
  it('renders a labelled table with the standardized header and body rows', () => {
    render(
      <TableShell ariaLabel="Teste" minWidth={480} headers={['Nome', {label: 'Valor', align: 'right'}]}>
        <tr>
          <td className={TABLE_CELL}>A</td>
          <td className={`${TABLE_CELL} text-right`}>1</td>
        </tr>
      </TableShell>,
    )

    const table = screen.getByRole('table', {name: 'Teste'})
    expect(table).toBeInTheDocument()
    expect(table).toHaveStyle({minWidth: '480px'})
    // Header text + standardized uppercase labels are present.
    expect(screen.getByText('Nome')).toBeInTheDocument()
    expect(screen.getByText('Valor')).toBeInTheDocument()
    // Standardized body cell padding applied to the row's cells.
    expect(screen.getByText('A').closest('td')).toHaveClass(TABLE_CELL)
  })

  it('applies the dimmed opacity state for background refetch', () => {
    const {container} = render(
      <TableShell ariaLabel="Teste" headers={['Nome']} dimmed>
        <tr><td className={TABLE_CELL}>A</td></tr>
      </TableShell>,
    )
    expect(container.firstElementChild).toHaveClass('opacity-60')
  })

  it('carries data-label on body cells for the mobile card layout', () => {
    render(
      <TableShell ariaLabel="Teste" headers={['Nome', 'Valor', {label: '', align: 'right'}]}>
        <tr>
          <td data-label="Nome" className={TABLE_CELL}>A</td>
          <td data-label="Valor" className={TABLE_CELL}>1</td>
          <td className={`${TABLE_CELL} text-right`}>editar</td>
        </tr>
      </TableShell>,
    )

    const nome = screen.getByText('A').closest('td')
    const valor = screen.getByText('1').closest('td')
    const acao = screen.getByText('editar').closest('td')
    // Data-label cells feed the column name shown in the stacked mobile card.
    expect(nome).toHaveAttribute('data-label', 'Nome')
    expect(valor).toHaveAttribute('data-label', 'Valor')
    // Control / action cells carry no data-label.
    expect(acao).not.toHaveAttribute('data-label')
  })
})
