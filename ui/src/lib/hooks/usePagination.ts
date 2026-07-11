'use client'

import { useState } from 'react'
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import type { PaginatedResponse } from '@/lib/types/api'

export interface UsePaginationOptions<T> {
  /** Base query key — cursor is appended automatically. */
  queryKey: readonly unknown[]
  queryFn: (cursor?: string) => Promise<PaginatedResponse<T>>
  enabled?: boolean
  staleTime?: number
}

export interface UsePaginationResult<T> {
  items: T[]
  isLoading: boolean
  isFetching: boolean
  hasNext: boolean
  hasPrevious: boolean
  goNext: () => void
  goPrevious: () => void
  reset: () => void
}

export function usePagination<T>({
  queryKey,
  queryFn,
  enabled = true,
  staleTime,
}: UsePaginationOptions<T>): UsePaginationResult<T> {
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  // Stack of previous cursors — each entry is the cursor that produced the current page
  const [stack, setStack] = useState<(string | undefined)[]>([])

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [...queryKey, { cursor }],
    queryFn: () => queryFn(cursor),
    enabled,
    staleTime,
  } as UseQueryOptions<PaginatedResponse<T>>)

  const goNext = () => {
    if (!data?.next_cursor) return
    setStack(prev => [...prev, cursor])
    setCursor(data.next_cursor ?? undefined)
  }

  const goPrevious = () => {
    if (stack.length === 0) return
    const prev = stack[stack.length - 1]
    setStack(s => s.slice(0, -1))
    setCursor(prev)
  }

  const reset = () => {
    setCursor(undefined)
    setStack([])
  }

  return {
    items: data?.items ?? [],
    isLoading,
    isFetching,
    hasNext: data?.has_next ?? false,
    hasPrevious: stack.length > 0,
    goNext,
    goPrevious,
    reset,
  }
}
