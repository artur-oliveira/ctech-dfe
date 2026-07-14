'use client'

import {useState} from 'react'
import {cn} from '@/lib/utils'

/**
 * Disclosure for an optional/advanced group of fields. Collapsed by default so
 * the primary flow stays uncluttered ("expert mode" in the emit wizard); the
 * chevron + label make the seam discoverable, and the region is keyboard- and
 * screen-reader-reachable via a native <button> + aria-expanded.
 */
export function CollapsibleSection({
  title,
  description,
  defaultOpen = false,
  children,
  className,
}: {
  title: string
  description?: string
  defaultOpen?: boolean
  children: React.ReactNode
  className?: string
}) {
  const [open, setOpen] = useState(defaultOpen)
  const panelId = `collapsible-${title.replace(/\s+/g, '-').toLowerCase()}`

  return (
    <div className={cn('rounded-xl border border-gray-200 bg-white', className)}>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen(v => !v)}
        className="flex w-full items-center justify-between gap-2 px-5 py-3.5 text-left focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
      >
        <span className="min-w-0">
          <span className="block text-xs font-semibold uppercase tracking-wider text-gray-500">{title}</span>
          {description && <span className="mt-0.5 block text-xs text-gray-400">{description}</span>}
        </span>
        <svg
          className={cn('size-4 shrink-0 text-gray-400 transition-transform duration-200', open && 'rotate-180')}
          viewBox="0 0 20 20" fill="none" aria-hidden="true"
        >
          <path d="M5 7.5 10 12.5 15 7.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      </button>
      {open && (
        <div id={panelId} className="space-y-3 border-t border-gray-100 px-5 py-4">
          {children}
        </div>
      )}
    </div>
  )
}
