import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest'
import {act, renderHook} from '@testing-library/react'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import type {ReactNode} from 'react'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'

const toastFn = vi.fn()
vi.mock('sonner', () => ({
  toast: (...args: unknown[]) => toastFn(...args),
}))

interface Item {
  id: string
  name: string
}

function wrapper({children}: {children: ReactNode}) {
  const qc = new QueryClient({defaultOptions: {mutations: {retry: false}}})
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

function setup(mutationFn: (id: string) => Promise<unknown>) {
  return renderHook(
    () =>
      useEntityDelete<Item>({
        mutationFn,
        getId: (i) => i.id,
        getDeletedMessage: (i) => `${i.name} excluído`,
        delayMs: 5000,
      }),
    {wrapper},
  )
}

const ITEM: Item = {id: 'a', name: 'Produto A'}

describe('useEntityDelete', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    toastFn.mockClear()
  })
  afterEach(() => vi.useRealTimers())

  it('hides the row immediately and only deletes after the undo window elapses', async () => {
    const mutationFn = vi.fn().mockResolvedValue(undefined)
    const {result} = setup(mutationFn)

    act(() => result.current.handleDelete(ITEM))

    // Hidden right away, but nothing deleted yet.
    expect(result.current.isHidden('a')).toBe(true)
    expect(result.current.filterVisible([ITEM])).toEqual([])
    expect(mutationFn).not.toHaveBeenCalled()
    expect(toastFn).toHaveBeenCalledWith('Produto A excluído', expect.objectContaining({duration: 5000}))

    // Fire the timer and flush react-query's microtask queue.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(mutationFn).toHaveBeenCalledOnce()
    expect(mutationFn.mock.calls[0][0]).toBe('a')
  })

  it('undo cancels the delete and restores the row', async () => {
    const mutationFn = vi.fn().mockResolvedValue(undefined)
    const {result} = setup(mutationFn)

    act(() => result.current.handleDelete(ITEM))
    expect(result.current.isHidden('a')).toBe(true)

    // Trigger the toast's "Desfazer" action.
    const opts = toastFn.mock.calls[0][1] as {action: {label: string; onClick: () => void}}
    expect(opts.action.label).toBe('Desfazer')
    act(() => opts.action.onClick())

    expect(result.current.isHidden('a')).toBe(false)
    expect(result.current.filterVisible([ITEM])).toEqual([ITEM])

    act(() => {
      vi.advanceTimersByTime(10_000)
    })
    expect(mutationFn).not.toHaveBeenCalled()
  })
})
