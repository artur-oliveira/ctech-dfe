'use client'

import {useCallback, useMemo, useState} from 'react'

/**
 * Generic multi-row selection for list/table pages. Tracks selected ids in a
 * Set; `allIds` (the currently-visible row ids) drives the header select-all
 * checkbox and its indeterminate state. Shared by products/persons/vehicles so
 * the bulk-select experience is uniform.
 */
export function useRowSelection(allIds: string[]) {
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const toggle = useCallback((id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const clear = useCallback(() => setSelected(new Set()), [])

  // Only count ids still present in the visible list (rows can disappear).
  const selectedIds = useMemo(() => allIds.filter(id => selected.has(id)), [allIds, selected])

  const allSelected = allIds.length > 0 && selectedIds.length === allIds.length
  const someSelected = selectedIds.length > 0 && !allSelected

  const toggleAll = useCallback(() => {
    setSelected(prev => (allIds.every(id => prev.has(id)) ? new Set() : new Set(allIds)))
  }, [allIds])

  return {
    selectedIds,
    count: selectedIds.length,
    isSelected: (id: string) => selected.has(id),
    toggle,
    toggleAll,
    clear,
    allSelected,
    someSelected,
  }
}
