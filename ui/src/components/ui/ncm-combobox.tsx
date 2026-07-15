'use client'

import React, {useCallback, useEffect, useRef, useState} from 'react'
import {createPortal} from 'react-dom'
import {FuseResult} from 'fuse.js'
import {CheckIcon, ChevronDownIcon, LoaderCircleIcon} from 'lucide-react'
import {cn} from '@/lib/utils'
import {ALL_NCMS, type NcmEntry} from '@/lib/data/ncm'
import {Highlighted} from '@/components/ui/highlight'

// ─── Types ────────────────────────────────────────────────────────────────────
const NCM_MAP = new Map(
  ALL_NCMS.map(ncm => [
    ncm.code.replace(/\D/g, ''),
    ncm
  ])
)

interface NcmComboboxProps {
  value?: string | null
  onValueChange?: (value: string) => void
  placeholder?: string
  disabled?: boolean
  className?: string
  id?: string
}

interface DropdownPos {
  top: number
  left: number
  width: number
  maxWidth: number
}

type WorkerResult = { id: number; results: FuseResult<NcmEntry>[] }

// ─── Component ────────────────────────────────────────────────────────────────

export function NcmCombobox({
                              value,
                              onValueChange,
                              placeholder = 'Buscar NCM',
                              disabled,
                              className,
                              id,
                            }: NcmComboboxProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [results, setResults] = useState<FuseResult<NcmEntry>[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [pos, setPos] = useState<DropdownPos | null>(null)

  const triggerRef = useRef<HTMLButtonElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const workerRef = useRef<Worker | null>(null)
  const queryIdRef = useRef(0)
  const [activeIndex, setActiveIndex] = useState(-1)
  const itemRefs = useRef<
    Array<HTMLButtonElement | null>
  >([])

  const selected = value ? NCM_MAP.get(value) : undefined

  const updatePosition = useCallback(() => {
    const rect = triggerRef.current?.getBoundingClientRect()
    if (!rect) return

    const MARGIN = 8
    const idealWidth = Math.max(rect.width, Math.min(420, window.innerWidth - MARGIN * 2))
    const left = Math.min(rect.left, window.innerWidth - idealWidth - MARGIN)

    setPos({
      top: rect.bottom + 4,
      left: Math.max(MARGIN, left),
      width: idealWidth,
      maxWidth: idealWidth,
    })
  }, [])

  const handleSearchChange = useCallback((q: string) => {
    setSearch(q)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (q.trim().length < 2) {
      setResults([])
      setIsSearching(false)
      return
    }
    setIsSearching(true)
    debounceRef.current = setTimeout(() => {
      const id = ++queryIdRef.current
      workerRef.current?.postMessage({query: q, id})
    }, 300)
  }, [])

  const resetSearch = useCallback(() => {
    setSearch('')
    setResults([])
    setIsSearching(false)
  }, [])

  const handleSelect = (optValue: string) => {
    onValueChange?.(optValue)
    setOpen(false)
    resetSearch()
  }

  const handleKeyDown = (
    e: React.KeyboardEvent,
  ) => {
    if (!open) return

    switch (e.key) {
      case 'Escape':
        setOpen(false)
        resetSearch()
        break

      case 'ArrowDown':
        e.preventDefault()

        setActiveIndex((v) =>
          Math.min(v + 1, results.length - 1),
        )

        break

      case 'ArrowUp':
        e.preventDefault()

        setActiveIndex((v) =>
          Math.max(v - 1, 0),
        )

        break

      case 'Home':
        e.preventDefault()
        setActiveIndex(0)
        break

      case 'End':
        e.preventDefault()
        setActiveIndex(results.length - 1)
        break

      case 'Enter':
        e.preventDefault()

        if (activeIndex >= 0) {
          const item = results[activeIndex]?.item

          if (item) {
            handleSelect(
              item.code.replace(/\D/g, ''),
            )
          }
        }

        break
    }
  }

  useEffect(() => {
    if (!open) return
    updatePosition()
    inputRef.current?.focus()
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) return

    const handle = () => {
      updatePosition()
    }

    window.addEventListener('scroll', handle, true)
    window.addEventListener('resize', handle)

    return () => {
      window.removeEventListener('scroll', handle, true)
      window.removeEventListener('resize', handle)
    }
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (
        !triggerRef.current?.contains(e.target as Node) &&
        !dropdownRef.current?.contains(e.target as Node)
      ) {
        setOpen(false)
        resetSearch()
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open, resetSearch])

  useEffect(() => {
    if (activeIndex < 0) return

    itemRefs.current[
      activeIndex
      ]?.scrollIntoView({
      block: 'nearest',
    })
  }, [activeIndex])

  useEffect(() => {
    const worker = new Worker(
      new URL('../../lib/workers/ncm-search.worker.ts', import.meta.url),
    )
    worker.onmessage = (e: MessageEvent<WorkerResult>) => {
      if (e.data.id === queryIdRef.current) {
        setResults(e.data.results)
        setIsSearching(false)
      }
    }
    workerRef.current = worker

    worker.onmessage = (
      e: MessageEvent<WorkerResult>,
    ) => {
      if (e.data.id === queryIdRef.current) {
        const nextResults = e.data.results

        setResults(nextResults)
        setActiveIndex(
          nextResults.length ? 0 : -1,
        )
        setIsSearching(false)
      }
    }

    return () => worker.terminate()
  }, [])

  useEffect(() => {
    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current)
      }
    }
  }, [])


  const dropdown = open && pos ? (
    <div
      ref={dropdownRef}
      style={{top: pos.top, left: pos.left, width: pos.width, maxWidth: pos.maxWidth}}
      className="fixed z-9999 rounded-lg border border-input bg-popover text-popover-foreground shadow-popover ring-1 ring-foreground/10"
    >
      <div className="flex items-center gap-2 border-b border-input px-3 py-2">
        <input
          ref={inputRef}
          value={search}
          onChange={(e) => handleSearchChange(e.target.value)}
          placeholder="Código ou descrição..."
          className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
        {isSearching && (
          <LoaderCircleIcon className="size-3.5 shrink-0 animate-spin text-muted-foreground"/>
        )}
      </div>
      <div id="ncm-listbox"
           role="listbox"
           className="max-h-72 overflow-x-hidden overflow-y-auto p-1">
        {search.trim().length < 2 ? (
          <p className="px-2 py-4 text-center text-sm text-muted-foreground">
            Digite ao menos 2 caracteres para buscar
          </p>
        ) : isSearching && results.length === 0 ? null : results.length === 0 ? (
          <p className="px-2 py-4 text-center text-sm text-muted-foreground">Nenhum resultado</p>
        ) : (
          results.map(({item}, index) => {
            const optValue = item.code.replace(/\D/g, '')
            const pathLabel = item.path.join(' › ')
            return (
              <button
                key={optValue}
                role="option"
                aria-selected={optValue === value}
                type="button"
                ref={(el) => {
                  itemRefs.current[index] = el
                }}
                onClick={() => handleSelect(optValue)}
                className={cn(
                  'flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground',

                  activeIndex === index &&
                  'bg-accent text-accent-foreground',

                  optValue === value &&
                  'bg-accent/50',
                )}
              >
                <span className="flex-1 min-w-0 overflow-hidden">
                  <span className="block font-mono leading-snug overflow-hidden text-ellipsis whitespace-nowrap">
                    <Highlighted text={`${item.code} – ${item.description}`} query={search}/>
                  </span>
                  {pathLabel && (
                    <span className="block text-xs text-muted-foreground truncate mt-0.5">
                      <Highlighted text={pathLabel} query={search}/>
                    </span>
                  )}
                </span>
                {optValue === value && (
                  <CheckIcon className="mt-0.5 size-4 shrink-0 text-primary"/>
                )}
              </button>
            )
          })
        )}
      </div>
    </div>
  ) : null

  return (
    <div className={cn('relative min-w-0', className)} onKeyDown={handleKeyDown}>
      <button
        ref={triggerRef}
        id={id}
        role="combobox"
        aria-expanded={open}
        aria-controls="ncm-listbox"
        aria-haspopup="listbox"
        type="button"
        disabled={disabled}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          'flex h-8 w-full items-center justify-between gap-1.5 rounded-lg border border-input bg-transparent px-2.5 text-sm transition-colors outline-none select-none',
          'focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
          'disabled:cursor-not-allowed disabled:opacity-50',
        )}
      >
        <span className="block min-w-0 flex-1 truncate text-left font-mono">
          {selected ? (
            `${selected.code} – ${selected.description}`
          ) : (
            <span className="text-muted-foreground font-sans">{placeholder}</span>
          )}
        </span>
        <ChevronDownIcon className="size-4 shrink-0 text-muted-foreground"/>
      </button>

      {typeof document !== 'undefined' && createPortal(dropdown, document.body)}
    </div>
  )
}
