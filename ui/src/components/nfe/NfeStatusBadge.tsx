'use client'

import {useState} from 'react'
import type {NfeStatus} from '@/lib/types/api'
import {Modal} from '@/components/ui/modal'

export const NFE_STATUS_LABELS: Record<NfeStatus, string> = {
  pending: 'Pendente',
  authorized: 'Autorizada',
  rejected: 'Rejeitada',
  failed: 'Falha',
  cancel_pending: 'Cancelando',
  cancelled: 'Cancelada',
}

export const NFE_STATUS_CLASSES: Record<NfeStatus, string> = {
  pending: 'bg-amber-50 text-amber-700',
  authorized: 'bg-green-100 text-green-700',
  rejected: 'bg-red-100 text-red-700',
  failed: 'bg-red-200 text-red-800',
  cancel_pending: 'bg-orange-100 text-orange-700',
  cancelled: 'bg-gray-100 text-gray-500',
}

// Transitional (in-flight) statuses get a subtle pulse + animated dot to signal
// "work in progress". Respects prefers-reduced-motion.
const TRANSITIONAL_STATUSES: ReadonlySet<NfeStatus> = new Set<NfeStatus>(['pending', 'cancel_pending'])

// In-flight statuses (Pendente / Cancelando) signal "work in progress" and get the pulse animation.
export const isTransitionalStatus = (status: NfeStatus): boolean => TRANSITIONAL_STATUSES.has(status)

export function NfeStatusBadge({status}: { status: NfeStatus }) {
  const isTransitional = isTransitionalStatus(status)
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${NFE_STATUS_CLASSES[status] ?? 'bg-gray-100 text-gray-600'} ${isTransitional ? 'animate-pulse motion-reduce:animate-none' : ''}`}
    >
      {isTransitional && (
        <span
          className="inline-block w-1.5 h-1.5 rounded-full bg-current opacity-70 animate-pulse motion-reduce:animate-none"/>
      )}
      {NFE_STATUS_LABELS[status] ?? status}
    </span>
  )
}

export function NfeStatusCell({status, sefazMotive}: { status: NfeStatus; sefazMotive: string | null }) {
  const [open, setOpen] = useState(false)
  const hasMotive = (status === 'rejected' || status === 'failed') && !!sefazMotive
  if (!hasMotive) return <NfeStatusBadge status={status}/>
  // Clickable badge → modal (portal): works on mobile and never gets clipped by
  // the table's overflow container, unlike a hover tooltip.
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}
              className="inline-flex items-center gap-1 cursor-pointer" title="Ver motivo da rejeição">
        <NfeStatusBadge status={status}/>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
             strokeLinecap="round" strokeLinejoin="round" className="text-red-400 shrink-0" aria-hidden="true">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
      </button>
      <Modal isOpen={open} title="Motivo da rejeição" onClose={() => setOpen(false)} cancelLabel="Fechar">
        <p className="text-sm text-gray-700 whitespace-pre-wrap break-words">{sefazMotive}</p>
      </Modal>
    </>
  )
}
