'use client'

import {useState} from 'react'
import {useMutation} from '@tanstack/react-query'
import {toast} from 'sonner'

interface UseEntityDeleteOptions<TItem> {
  mutationFn: (id: string) => Promise<unknown>
  getId: (item: TItem) => string
  /** Message shown in the undo toast, e.g. "Usuário excluído" or `Produto "X" excluído`. */
  getDeletedMessage: (item: TItem) => string
  onSuccess?: () => void
  /** How long the undo window stays open before the delete commits (ms). */
  delayMs?: number
}

/**
 * Optimistic delete with an undo window. Clicking delete hides the row
 * immediately and shows a toast with "Desfazer"; the actual delete request is
 * fired only when the toast's window elapses. Undo cancels it and restores the
 * row. Shared by products/persons/vehicles/members so the experience is uniform.
 *
 * The rendered list must skip hidden rows: `items.filter(i => !isHidden(getId(i)))`.
 */
export function useEntityDelete<TItem>({
                                         mutationFn,
                                         getId,
                                         getDeletedMessage,
                                         onSuccess,
                                         delayMs = 5000,
                                       }: UseEntityDeleteOptions<TItem>) {
  const [hidden, setHidden] = useState<Set<string>>(new Set())
  const mutation = useMutation({mutationFn, onSuccess})

  const reveal = (id: string) =>
    setHidden((prev) => {
      const next = new Set(prev)
      next.delete(id)
      return next
    })

  const handleDelete = (item: TItem) => {
    const id = getId(item)
    setHidden((prev) => new Set(prev).add(id))

    const timer = setTimeout(() => {
      mutation.mutate(id, {onSettled: () => reveal(id)})
    }, delayMs)

    toast(getDeletedMessage(item), {
      duration: delayMs,
      action: {
        label: 'Desfazer',
        onClick: () => {
          clearTimeout(timer)
          reveal(id)
        },
      },
    })
  }

  return {
    handleDelete,
    isHidden: (id: string) => hidden.has(id),
    /** Drop rows currently in the undo window from a list before rendering. */
    filterVisible: (items: TItem[]) => items.filter((i) => !hidden.has(getId(i))),
    isPending: mutation.isPending,
  }
}
