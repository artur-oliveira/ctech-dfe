'use client'

import React, {useEffect, useMemo, useRef, useState} from 'react'
import {createPortal} from 'react-dom'
import Fuse from 'fuse.js'
import {CheckIcon, ChevronDownIcon} from 'lucide-react'
import {cn} from '@/lib/utils'
import {Highlighted} from '@/components/ui/highlight'

export interface ComboboxOption {
  value: string
  label: string    // full text shown in the dropdown
  display?: string // compact text shown in the trigger (defaults to value)
}

interface ComboboxProps {
  value?: string | null
  onValueChange?: (value: string) => void
  options: ComboboxOption[]
  placeholder?: string
  searchPlaceholder?: string
  disabled?: boolean
  className?: string
  id?: string
  fuzzySearch?: boolean
}

const PAGE_SIZE = 50

interface DropdownPos {
  top?: number
  bottom?: number
  left: number
  width: number
  maxWidth: number
}

export function Combobox({
                           value,
                           onValueChange,
                           options,
                           placeholder = 'Selecione',
                           searchPlaceholder = 'Buscar...',
                           disabled,
                           className,
                           id,
                           fuzzySearch = false,
                         }: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [pos, setPos] = useState<DropdownPos | null>(null)
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)
  const [prevSearch, setPrevSearch] = useState(search)
  const [prevOpen, setPrevOpen] = useState(open)
  if (prevSearch !== search || prevOpen !== open) {
    setPrevSearch(search)
    setPrevOpen(open)
    setVisibleCount(PAGE_SIZE)
  }
  const triggerRef = useRef<HTMLButtonElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const selected = value ? options.find((o) => o.value === value) : undefined

  const filtered = useMemo(() => {
    const q = search.trim()
    if (!q) return options
    if (fuzzySearch) {
      const fuse = new Fuse(options, {
        keys: ['value', 'label'],
        threshold: 0.3,
        ignoreDiacritics: true,
        ignoreLocation: true,
      })
      return fuse.search(q).map(({item}) => item)
    }
    const normalizedQuery = q.toLowerCase()
    return options.filter((o) =>
      o.label.toLowerCase().includes(normalizedQuery) || o.value.toLowerCase().includes(normalizedQuery),
    )
  }, [fuzzySearch, options, search])

  const visibleItems = filtered.slice(0, visibleCount)
  const hasMore = visibleCount < filtered.length

  // Recompute position whenever the dropdown opens
  useEffect(() => {
    if (!open) return
    const rect = triggerRef.current?.getBoundingClientRect()
    if (rect) {
      const DROPDOWN_HEIGHT = 300
      const spaceBelow = window.innerHeight - rect.bottom - 8
      const openUpward = spaceBelow < DROPDOWN_HEIGHT && rect.top > spaceBelow
      setPos({
        ...(openUpward
          ? {bottom: window.innerHeight - rect.top + 4}
          : {top: rect.bottom + 4}),
        left: rect.left,
        width: rect.width,
        maxWidth: Math.min(320, window.innerWidth - rect.left - 8),
      })
    }
    inputRef.current?.focus()
  }, [open])

  // Close on scroll outside the dropdown (the list's own scroll must not close it)
  useEffect(() => {
    if (!open) return
    const close = (e: Event) => {
      if (dropdownRef.current?.contains(e.target as Node)) return
      setOpen(false)
      setSearch('')
    }
    window.addEventListener('scroll', close, {capture: true, passive: true})
    return () => window.removeEventListener('scroll', close, {capture: true})
  }, [open])

  // Close on outside click — must exclude both trigger and portalled dropdown
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (
        !triggerRef.current?.contains(e.target as Node) &&
        !dropdownRef.current?.contains(e.target as Node)
      ) {
        setOpen(false)
        setSearch('')
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  // Load more on scroll
  useEffect(() => {
    const el = listRef.current
    if (!el) return
    const onScroll = () => {
      if (el.scrollTop + el.clientHeight >= el.scrollHeight - 32) {
        setVisibleCount((v) => v + PAGE_SIZE)
      }
    }
    el.addEventListener('scroll', onScroll, {passive: true})
    return () => el.removeEventListener('scroll', onScroll)
  })

  const handleSelect = (optValue: string) => {
    onValueChange?.(optValue)
    setOpen(false)
    setSearch('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setOpen(false)
      setSearch('')
    }
  }

  const dropdown = open && pos ? (
    <div
      ref={dropdownRef}
      style={{top: pos.top, bottom: pos.bottom, left: pos.left, minWidth: pos.width, maxWidth: pos.maxWidth}}
      className="fixed z-50 rounded-lg border border-input bg-popover text-popover-foreground shadow-popover ring-1 ring-foreground/10"
    >
      <div className="border-b border-input px-3 py-2">
        <input
          ref={inputRef}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={searchPlaceholder}
          className="w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
      </div>
      <div ref={listRef} className="max-h-60 overflow-y-auto p-1">
        {filtered.length === 0 ? (
          <p className="px-2 py-4 text-center text-sm text-muted-foreground">Nenhum resultado</p>
        ) : (
          <>
            {visibleItems.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => handleSelect(opt.value)}
                className={cn(
                  'flex w-full items-start gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground',
                  opt.value === value && 'bg-accent/50',
                )}
              >
                <span className="flex-1 leading-snug"><Highlighted text={opt.label} query={search}/></span>
                {opt.value === value && (
                  <CheckIcon className="mt-0.5 size-4 shrink-0 text-primary"/>
                )}
              </button>
            ))}
            {hasMore && (
              <p className="px-2 py-2 text-center text-xs text-muted-foreground">
                {filtered.length - visibleCount} resultado{filtered.length - visibleCount !== 1 ? 's' : ''} a mais —
                continue rolando
              </p>
            )}
          </>
        )}
      </div>
    </div>
  ) : null

  return (
    <div className={cn('relative min-w-0', className)} onKeyDown={handleKeyDown}>
      <button
        ref={triggerRef}
        id={id}
        type="button"
        disabled={disabled}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          'flex h-8 w-full items-center justify-between gap-1.5 rounded-lg border border-input bg-transparent px-2.5 text-sm transition-colors outline-none select-none',
          'focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
          'disabled:cursor-not-allowed disabled:opacity-50',
          !selected && 'text-muted-foreground',
        )}
      >
        <span className="block min-w-0 flex-1 truncate text-left text-foreground">
          {selected ? (selected.display ?? selected.value) : (
            <span className="text-muted-foreground">{placeholder}</span>
          )}
        </span>
        <ChevronDownIcon className="size-4 shrink-0 text-muted-foreground"/>
      </button>

      {typeof document !== 'undefined' && createPortal(dropdown, document.body)}
    </div>
  )
}
