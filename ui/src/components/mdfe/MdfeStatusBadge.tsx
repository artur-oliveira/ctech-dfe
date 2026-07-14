'use client'

import type {MdfeStatus} from '@/lib/types/api'
import {StatusBadge, StatusCell} from '@/components/ui/status-badge'

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
  return (
    <StatusBadge
      label={MDFE_STATUS_LABELS[status] ?? status}
      className={MDFE_STATUS_CLASSES[status] ?? 'bg-gray-100 text-gray-600'}
      isTransitional={isTransitionalMdfeStatus(status)}
    />
  )
}

export function MdfeStatusCell({status, sefazMotive}: { status: MdfeStatus; sefazMotive: string | null }) {
  return (
    <StatusCell
      badge={<MdfeStatusBadge status={status}/>}
      sefazMotive={sefazMotive}
      showMotive={status === 'rejected' || status === 'failed'}
    />
  )
}
