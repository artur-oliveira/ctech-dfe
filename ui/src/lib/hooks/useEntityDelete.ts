'use client'

import {useMutation} from '@tanstack/react-query'

interface UseEntityDeleteOptions<TItem> {
  mutationFn: (id: string) => Promise<unknown>
  getId: (item: TItem) => string
  getConfirmMessage: (item: TItem) => string
  onSuccess?: () => void
}

export function useEntityDelete<TItem>({
  mutationFn,
  getId,
  getConfirmMessage,
  onSuccess,
}: UseEntityDeleteOptions<TItem>) {
  const mutation = useMutation({mutationFn, onSuccess})

  const handleDelete = (item: TItem) => {
    if (!confirm(getConfirmMessage(item))) return
    mutation.mutate(getId(item))
  }

  return {handleDelete, isPending: mutation.isPending}
}
