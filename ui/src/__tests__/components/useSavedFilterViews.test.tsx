import {describe, it, expect, beforeEach} from 'vitest'
import {renderHook, act} from '@testing-library/react'
import {useSavedFilterViews} from '@/lib/hooks/useSavedFilterViews'
import {STORAGE_KEY_SAVED_FILTER_VIEWS} from '@/lib/constants/storage'

describe('useSavedFilterViews', () => {
  beforeEach(() => window.localStorage.clear())

  it('saves and applies named views, namespaced per page', () => {
    const {result} = renderHook(() => useSavedFilterViews('cte-distributions'))

    act(() => result.current.saveView('Minha view', '12345'))
    expect(result.current.views).toEqual([{name: 'Minha view', nsu: '12345'}])

    // Namespaced: a different page id starts empty.
    const other = renderHook(() => useSavedFilterViews('mdfe-distributions'))
    expect(other.result.current.views).toEqual([])

    // Persisted to localStorage under the page id.
    const raw = JSON.parse(window.localStorage.getItem(STORAGE_KEY_SAVED_FILTER_VIEWS)!)
    expect(raw['cte-distributions']).toEqual([{name: 'Minha view', nsu: '12345'}])
  })

  it('deletes a saved view', () => {
    const {result} = renderHook(() => useSavedFilterViews('cte-distributions'))
    act(() => result.current.saveView('A', '1'))
    act(() => result.current.deleteView('A'))
    expect(result.current.views).toEqual([])
  })

  it('replaces a view with the same name', () => {
    const {result} = renderHook(() => useSavedFilterViews('cte-distributions'))
    act(() => result.current.saveView('A', '1'))
    act(() => result.current.saveView('A', '2'))
    expect(result.current.views).toEqual([{name: 'A', nsu: '2'}])
  })
})
