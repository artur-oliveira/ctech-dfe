import {render, screen} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {describe, expect, it, vi} from 'vitest'
import {Combobox} from '@/components/ui/combobox'

describe('Combobox', () => {
  it('encontra opções por aproximação quando a busca Fuse está habilitada', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Combobox
        fuzzySearch
        value=""
        onValueChange={onValueChange}
        options={[
          {value: '415', label: '415 · 04.15 — Psicanálise · 3%'},
          {value: '416', label: '416 · 04.16 — Psicologia · 3%'},
        ]}
      />,
    )

    await user.click(screen.getByRole('combobox'))
    await user.type(screen.getByPlaceholderText('Buscar...'), 'psicanalze')
    await user.click(screen.getByRole('option', {name: /Psicanálise/}))

    expect(onValueChange).toHaveBeenCalledWith('415')
  })

  it('permite selecionar via teclado (seta baixo + enter)', async () => {
    const onValueChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Combobox
        value=""
        onValueChange={onValueChange}
        options={[
          {value: '415', label: '415 · 04.15 — Psicanálise · 3%'},
          {value: '416', label: '416 · 04.16 — Psicologia · 3%'},
        ]}
      />,
    )

    const trigger = screen.getByRole('combobox')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')

    trigger.focus()
    await user.keyboard('{ArrowDown}')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    await user.keyboard('{ArrowDown}{Enter}')

    expect(onValueChange).toHaveBeenCalledWith('415')
  })
})
