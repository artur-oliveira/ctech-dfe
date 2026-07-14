'use client'

import type {NfeStatus} from '@/lib/types/api'
import {StatusBadge, StatusCell} from '@/components/ui/status-badge'

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
  return (
    <StatusBadge
      label={NFE_STATUS_LABELS[status] ?? status}
      className={NFE_STATUS_CLASSES[status] ?? 'bg-gray-100 text-gray-600'}
      isTransitional={isTransitionalStatus(status)}
    />
  )
}

export function NfeStatusCell({status, sefazMotive}: { status: NfeStatus; sefazMotive: string | null }) {
  return (
    <StatusCell
      badge={<NfeStatusBadge status={status}/>}
      sefazMotive={sefazMotive}
      showMotive={status === 'rejected' || status === 'failed'}
    />
  )
}
