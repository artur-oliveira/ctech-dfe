import {describe, expect, it, vi} from 'vitest'
import {render, screen, fireEvent} from '@testing-library/react'
import {UfOverridesEditor} from '@/components/tax/UfOverridesEditor'

describe('UfOverridesEditor', () => {
  it('adiciona um card vazio ao clicar em "Adicionar override por UF"', () => {
    const onChange = vi.fn()
    render(<UfOverridesEditor value={[]} onChange={onChange} simples={false}/>)
    fireEvent.click(screen.getByText('+ Adicionar override por UF'))
    expect(onChange).toHaveBeenCalledWith([{ufs: [], overrides: {}}])
  })

  it('alterna uma UF no card existente', () => {
    const onChange = vi.fn()
    render(<UfOverridesEditor value={[{ufs: [], overrides: {}}]} onChange={onChange} simples={false}/>)
    fireEvent.click(screen.getByRole('button', {name: 'SP'}))
    expect(onChange).toHaveBeenCalledWith([{ufs: ['SP'], overrides: {}}])
  })

  it('remove um card', () => {
    const onChange = vi.fn()
    render(<UfOverridesEditor value={[{ufs: ['SP'], overrides: {}}]} onChange={onChange} simples={false}/>)
    fireEvent.click(screen.getByText('remover'))
    expect(onChange).toHaveBeenCalledWith([])
  })
})
