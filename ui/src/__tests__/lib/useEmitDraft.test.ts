import {describe, it, expect, beforeEach, vi} from 'vitest'
import {renderHook, act} from '@testing-library/react'
import {useEmitDraft} from '@/lib/hooks/useEmitDraft'
import {STORAGE_KEY_EMIT_DRAFT_PREFIX} from '@/lib/constants/storage'

interface Cart {
  items: string[]
}

const KEY = `${STORAGE_KEY_EMIT_DRAFT_PREFIX}_nfce_ORG_1`

function seedDraft(state: Cart, savedAt = Date.now()) {
  window.localStorage.setItem(KEY, JSON.stringify({savedAt, state}))
}

describe('useEmitDraft', () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.useFakeTimers()
  })

  it('autosaves the form state after the debounce, namespaced per doc type and org', () => {
    const state: Cart = {items: ['a']}
    renderHook(() => useEmitDraft('nfce', 'ORG_1', state, true))

    act(() => void vi.advanceTimersByTime(600))

    const raw = JSON.parse(window.localStorage.getItem(KEY)!)
    expect(raw.state).toEqual({items: ['a']})
    // A different document type does not see it.
    expect(window.localStorage.getItem(`${STORAGE_KEY_EMIT_DRAFT_PREFIX}_nfe_ORG_1`)).toBeNull()
  })

  it('does not persist an empty form', () => {
    renderHook(() => useEmitDraft('nfce', 'ORG_1', {items: []} as Cart, false))
    act(() => void vi.advanceTimersByTime(600))
    expect(window.localStorage.getItem(KEY)).toBeNull()
  })

  it('offers a stored draft back on mount and keeps it until the user answers', () => {
    seedDraft({items: ['salvo']})
    const {result} = renderHook(() => useEmitDraft('nfce', 'ORG_1', {items: []} as Cart, false))

    expect(result.current.recovered?.state).toEqual({items: ['salvo']})

    // The empty form must not overwrite the draft it is offering to restore.
    act(() => void vi.advanceTimersByTime(600))
    expect(window.localStorage.getItem(KEY)).not.toBeNull()
  })

  it('discards and clears remove the stored draft', () => {
    seedDraft({items: ['salvo']})
    const {result} = renderHook(() => useEmitDraft('nfce', 'ORG_1', {items: []} as Cart, false))

    act(() => result.current.discard())

    expect(result.current.recovered).toBeNull()
    expect(window.localStorage.getItem(KEY)).toBeNull()
  })

  it('ignores drafts older than a week', () => {
    seedDraft({items: ['antigo']}, Date.now() - 8 * 24 * 60 * 60 * 1000)
    const {result} = renderHook(() => useEmitDraft('nfce', 'ORG_1', {items: []} as Cart, false))
    expect(result.current.recovered).toBeNull()
    expect(window.localStorage.getItem(KEY)).toBeNull()
  })

  it('survives corrupt storage', () => {
    window.localStorage.setItem(KEY, 'not json')
    const {result} = renderHook(() => useEmitDraft('nfce', 'ORG_1', {items: []} as Cart, false))
    expect(result.current.recovered).toBeNull()
  })
})
