'use client'

import {useEffect, useMemo, useRef, useState} from 'react'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {useDebounce} from '@/lib/hooks/useDebounce'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import type {ProductOut} from '@/lib/types/api'

interface ProductSearchProps {
  onSelect: (product: ProductOut) => void
  /** Renders the panel chrome with a close action. Omit for an always-on search (POS). */
  onClose?: () => void
  /** Returns a reason string when a product cannot be used by this document. */
  disabledReason?: (product: ProductOut) => string | null
  placeholder?: string
  autoFocus?: boolean
  className?: string
}

/**
 * Product lookup shared by every emit flow.
 *
 * Keyboard-first: type to filter, ↑/↓ to move, Enter to add the highlighted
 * item, Esc to close. A barcode scanner is a keyboard that types fast and ends
 * with Enter, so the same path serves the NFC-e counter without extra wiring.
 */
export function ProductSearch({
  onSelect,
  onClose,
  disabledReason,
  placeholder = 'Código ou descrição...',
  autoFocus,
  className,
}: ProductSearchProps) {
  const {selectedOrg} = useAuth()
  const [query, setQuery] = useState('')
  const debounced = useDebounce(query, 300)
  const [highlight, setHighlight] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const {data, isLoading} = useQuery({
    queryKey: queryKeys.products.list(selectedOrg?.pk),
    queryFn: () => apiClient.getProducts({limit: 50}),
    enabled: !!selectedOrg,
  })

  const filtered = useMemo(() => {
    const all = data?.items ?? []
    if (!debounced) return all
    const q = debounced.toLowerCase()
    return all.filter((p) => p.description.toLowerCase().includes(q) || p.code.toLowerCase().includes(q))
  }, [data, debounced])

  const selectable = filtered.filter((p) => !disabledReason?.(p))

  // Reset the highlight when the result set changes (setState during render —
  // the codebase's standard way to avoid the React 19 effect setState cascade).
  const [prevQuery, setPrevQuery] = useState(debounced)
  if (debounced !== prevQuery) {
    setPrevQuery(debounced)
    setHighlight(0)
  }

  // Keep the highlighted row inside the scroll viewport.
  useEffect(() => {
    listRef.current?.querySelector<HTMLElement>('[data-highlighted="true"]')
      ?.scrollIntoView?.({block: 'nearest'})
  }, [highlight])

  const choose = (product: ProductOut) => {
    if (disabledReason?.(product)) return
    onSelect(product)
    setQuery('')
    setHighlight(0)
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlight((h) => Math.min(h + 1, Math.max(0, selectable.length - 1)))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlight((h) => Math.max(0, h - 1))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const target = selectable[highlight]
      if (target) choose(target)
    } else if (e.key === 'Escape' && onClose) {
      e.preventDefault()
      onClose()
    }
  }

  return (
    <div className={className}>
      {onClose && (
        <div className="flex items-center justify-between mb-3">
          <p className="text-sm font-medium text-gray-600">Buscar produto</p>
          <Button type="button" variant="ghost" size="xs" onClick={onClose}
                  className="text-gray-500 hover:text-gray-700">
            Fechar
          </Button>
        </div>
      )}
      <Input
        ref={inputRef}
        type="text"
        autoFocus={autoFocus ?? !!onClose}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        aria-label="Buscar produto"
        className="w-full"
      />
      <div ref={listRef} className="max-h-48 overflow-y-auto space-y-0.5 mt-3">
        {isLoading ? (
          <div className="py-1">
            <LoadingSkeleton count={3} height="h-8" rounded="rounded-md"/>
          </div>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-gray-500 py-2">Nenhum produto encontrado.</p>
        ) : (
          filtered.map((p) => {
            const reason = disabledReason?.(p) ?? null
            const isHighlighted = !reason && selectable[highlight]?.sk === p.sk
            return (
              <button
                key={p.sk}
                type="button"
                disabled={!!reason}
                data-highlighted={isHighlighted}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => choose(p)}
                className={`w-full text-left px-3 py-2 rounded-md transition-colors flex items-center justify-between gap-2 ${
                  reason ? 'opacity-50 cursor-not-allowed' : isHighlighted ? 'bg-white shadow-card' : 'hover:bg-white'
                }`}
              >
                <span className="text-sm text-gray-900 min-w-0 truncate">
                  {p.description}
                  {p.brand && <span className="ml-1.5 text-xs text-gray-500">{p.brand}</span>}
                  {reason && <span className="ml-1.5 text-xs text-danger">{reason}</span>}
                </span>
                <span className="text-xs text-gray-500 shrink-0">
                  {parseFloat(p.value).toLocaleString('pt-BR', {minimumFractionDigits: 2, maximumFractionDigits: 2})}
                </span>
              </button>
            )
          })
        )}
      </div>
    </div>
  )
}
