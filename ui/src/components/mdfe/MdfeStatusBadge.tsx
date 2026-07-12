'use client'

import {useState} from 'react'
import type {MdfeStatus} from '@/lib/types/api'
import {Modal} from '@/components/ui/modal'

export const MDFE_STATUS_LABELS: Record<MdfeStatus, string> = {
  pending: 'Pendente',
  authorized: 'Autorizado',
  rejected: 'Rejeitado',
  failed: 'Falha',
  cancel_pending: 'Cancelando',
  cancelled: 'Cancelado',
  close_pending: 'Encerrando',
  closed: 'Encerrado',
}

export const MDFE_STATUS_CLASSES: Record<MdfeStatus, string> = {
  pending: 'bg-amber-50 text-amber-700',
  authorized: 'bg-green-100 text-green-700',
  rejected: 'bg-red-100 text-red-700',
  failed: 'bg-red-200 text-red-800',
  cancel_pending: 'bg-orange-100 text-orange-700',
  cancelled: 'bg-gray-100 text-gray-500',
  close_pending: 'bg-blue-50 text-blue-700',
  closed: 'bg-blue-100 text-blue-700',
}

// Transitional (in-flight) statuses get a subtle pulse to signal work in progress.
const TRANSITIONAL_STATUSES: ReadonlySet<MdfeStatus> = new Set<MdfeStatus>([
  'pending', 'cancel_pending', 'close_pending',
])

export const isTransitionalMdfeStatus = (status: MdfeStatus): boolean => TRANSITIONAL_STATUSES.has(status)

export function MdfeStatusBadge({status}: { status: MdfeStatus }) {
  const isTransitional = isTransitionalMdfeStatus(status)
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${MDFE_STATUS_CLASSES[status] ?? 'bg-gray-100 text-gray-600'} ${isTransitional ? 'animate-pulse motion-reduce:animate-none' : ''}`}
    >
      {isTransitional && (
        <span
          className="inline-block w-1.5 h-1.5 rounded-full bg-current opacity-70 animate-pulse motion-reduce:animate-none"/>
      )}
      {MDFE_STATUS_LABELS[status] ?? status}
    </span>
  )
}

export function MdfeStatusCell({status, sefazMotive}: { status: MdfeStatus; sefazMotive: string | null }) {
  const [open, setOpen] = useState(false)
  const hasMotive = (status === 'rejected' || status === 'failed') && !!sefazMotive
  if (!hasMotive) return <MdfeStatusBadge status={status}/>
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}
              className="inline-flex items-center gap-1 cursor-pointer" title="Ver motivo da rejeição">
        <MdfeStatusBadge status={status}/>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
             strokeLinecap="round" strokeLinejoin="round" className="text-red-400 shrink-0" aria-hidden="true">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
      </button>
      <Modal isOpen={open} title="Motivo da rejeição" onClose={() => setOpen(false)}>
        <p className="text-sm text-gray-700 whitespace-pre-wrap">{sefazMotive}</p>
      </Modal>
    </>
  )
}
