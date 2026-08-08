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

    await user.click(screen.getByRole('button'))
    await user.type(screen.getByPlaceholderText('Buscar...'), 'psicanalze')
    await user.click(screen.getByRole('button', {name: /Psicanálise/}))

    expect(onValueChange).toHaveBeenCalledWith('415')
  })
})
