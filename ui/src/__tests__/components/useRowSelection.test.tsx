import {describe, it, expect} from 'vitest'
import {renderHook, act} from '@testing-library/react'
import {useRowSelection} from '@/lib/hooks/useRowSelection'

describe('useRowSelection', () => {
  it('toggles individual rows and reports count', () => {
    const {result} = renderHook(() => useRowSelection(['a', 'b', 'c']))
    act(() => result.current.toggle('a'))
    act(() => result.current.toggle('b'))
    expect(result.current.count).toBe(2)
    expect(result.current.isSelected('a')).toBe(true)
    expect(result.current.someSelected).toBe(true)
    expect(result.current.allSelected).toBe(false)
  })

  it('select-all and clear', () => {
    const {result} = renderHook(() => useRowSelection(['a', 'b']))
    act(() => result.current.toggleAll())
    expect(result.current.allSelected).toBe(true)
    expect(result.current.count).toBe(2)
    act(() => result.current.toggleAll())
    expect(result.current.count).toBe(0)
  })

  it('ignores selected ids that leave the visible list', () => {
    const {result, rerender} = renderHook(({ids}) => useRowSelection(ids), {
      initialProps: {ids: ['a', 'b', 'c']},
    })
    act(() => result.current.toggle('c'))
    expect(result.current.count).toBe(1)
    // 'c' disappears from the list → no longer counted.
    rerender({ids: ['a', 'b']})
    expect(result.current.count).toBe(0)
    expect(result.current.selectedIds).toEqual([])
  })
})
