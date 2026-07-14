import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, screen, fireEvent, cleanup} from '@testing-library/react'
import {KeyboardShortcuts} from '@/components/layout/KeyboardShortcuts'

const push = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({push}),
  usePathname: () => '/cte',
}))

describe('KeyboardShortcuts', () => {
  beforeEach(() => {
    push.mockClear()
  })

  it("routes 'n' to the emit page of the current doc type", () => {
    render(<KeyboardShortcuts/>)
    fireEvent.keyDown(document.body, {key: 'n'})
    expect(push).toHaveBeenCalledWith('/cte/emit')
  })

  it("does not fire 'n' while typing in a field", () => {
    render(<KeyboardShortcuts/>)
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    fireEvent.keyDown(input, {key: 'n'})
    expect(push).not.toHaveBeenCalled()
    input.remove()
  })

  it("toggles the help dialog on '?' and closes on Escape", () => {
    render(<KeyboardShortcuts/>)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    fireEvent.keyDown(document.body, {key: '?', shiftKey: true})
    const dialog = screen.getByRole('dialog', {name: 'Atalhos de teclado'})
    expect(dialog).toBeInTheDocument()
    expect(dialog).toHaveTextContent('Nova emissão')

    fireEvent.keyDown(document.body, {key: 'Escape'})
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('cleans up after unmount (no listener leak)', () => {
    const {unmount} = render(<KeyboardShortcuts/>)
    unmount()
    cleanup()
    fireEvent.keyDown(document.body, {key: 'n'})
    expect(push).not.toHaveBeenCalled()
  })
})
