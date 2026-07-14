'use client'

import {useCallback, useState} from 'react'
import {STORAGE_KEY_SAVED_FILTER_VIEWS} from '@/lib/constants/storage'

export type SavedFilterView = {name: string; nsu: string}

/**
 * Named NSU filter views for a distribution page, persisted to localStorage.
 * No backend change — the NSU filter itself is applied client-side on the
 * already-fetched distribution list. `pageId` namespaces the views per page.
 */
export function useSavedFilterViews(pageId: string) {
  const [views, setViews] = useState<SavedFilterView[]>(() => {
    if (typeof window === 'undefined') return []
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY_SAVED_FILTER_VIEWS)
      if (!raw) return []
      const all = JSON.parse(raw) as Record<string, SavedFilterView[]>
      return all[pageId] ?? []
    } catch {
      return []
    }
  })

  const persist = useCallback((next: SavedFilterView[]) => {
    setViews(next)
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY_SAVED_FILTER_VIEWS)
      const all = raw ? (JSON.parse(raw) as Record<string, SavedFilterView[]>) : {}
      all[pageId] = next
      window.localStorage.setItem(STORAGE_KEY_SAVED_FILTER_VIEWS, JSON.stringify(all))
    } catch {
      /* localStorage unavailable — keep in-memory only */
    }
  }, [pageId])

  const saveView = useCallback((name: string, nsu: string) => {
    const trimmed = name.trim()
    if (!trimmed) return
    const next = [...views.filter(v => v.name !== trimmed), {name: trimmed, nsu}]
    persist(next)
  }, [views, persist])

  const deleteView = useCallback((name: string) => {
    persist(views.filter(v => v.name !== name))
  }, [views, persist])

  return {views, saveView, deleteView}
}
