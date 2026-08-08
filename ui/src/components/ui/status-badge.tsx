'use client'

import {type ReactNode, useState} from 'react'
import {Modal} from '@/components/ui/modal'

/**
 * Doc-neutral status badge primitive. Status uses a FIXED semantic palette
 * (green / red / amber / gray / blue) and is intentionally NOT recolored by
 * `data-dfe-theme` — a user must recognize "Autorizada" instantly across every
 * document type. See DESIGN.md §7. The per-type label/class maps live in each
 * doc's StatusBadge module and feed `label` / `className` here.
 */
export function StatusBadge({label, className, isTransitional, size = 'sm'}: {
  label: string
  className: string
  isTransitional?: boolean
  size?: 'sm' | 'md'
}) {
  const sizeClass = size === 'md' ? 'gap-1.5 px-2.5 py-1 text-sm' : 'gap-1 px-2 py-0.5 text-xs'
  return (
    <span
      className={`inline-flex items-center rounded font-medium ${sizeClass} ${className} ${isTransitional ? 'animate-pulse motion-reduce:animate-none' : ''}`}
    >
      {isTransitional && (
        <span
          className="inline-block w-1.5 h-1.5 rounded-full bg-current opacity-70 animate-pulse motion-reduce:animate-none"/>
      )}
      {label}
    </span>
  )
}

/**
 * Wraps a badge so that, when a motive exists for the status, the badge becomes
 * a button opening a portal Modal with the motive. The Modal (not a hover
 * tooltip) works on mobile and never gets clipped by the table's overflow
 * container. `motiveTitle` names the cause (rejection / failure / retry) —
 * passing it as null means this status has no motive to show.
 */
export function StatusCell({badge, sefazMotive, motiveTitle, iconClassName = 'text-red-600'}: {
  badge: ReactNode
  sefazMotive: string | null
  motiveTitle: string | null
  iconClassName?: string
}) {
  const [open, setOpen] = useState(false)
  if (!(motiveTitle && sefazMotive)) return <>{badge}</>
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}
              className="inline-flex items-center gap-1 cursor-pointer" title={`Ver ${motiveTitle.toLowerCase()}`}>
        {badge}
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
             strokeLinecap="round" strokeLinejoin="round" className={`${iconClassName} shrink-0`} aria-hidden="true">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
      </button>
      <Modal isOpen={open} title={motiveTitle} onClose={() => setOpen(false)} cancelLabel="Fechar">
        <p className="text-sm text-gray-700 whitespace-pre-wrap break-words">{sefazMotive}</p>
      </Modal>
    </>
  )
}
