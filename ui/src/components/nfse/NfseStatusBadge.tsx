'use client'

import type {NfseStatus} from '@/lib/types/api'
import {StatusBadge, StatusCell} from '@/components/ui/status-badge'

export const NFSE_STATUS_LABELS: Record<NfseStatus, string> = {
  pending: 'Pendente',
  processing: 'Processando',
  authorized: 'Autorizada',
  rejected: 'Rejeitada',
  cancelled: 'Cancelada',
  error: 'Erro',
}

export const NFSE_STATUS_CLASSES: Record<NfseStatus, string> = {
  pending: 'bg-amber-50 text-amber-700',
  processing: 'bg-blue-50 text-blue-700',
  authorized: 'bg-green-100 text-green-700',
  rejected: 'bg-red-100 text-red-700',
  cancelled: 'bg-gray-100 text-gray-500',
  error: 'bg-red-200 text-red-800',
}

export type NfseStatusTone = 'success' | 'danger' | 'warning' | 'info' | 'neutral'

const NFSE_STATUS_TONES: Record<NfseStatus, NfseStatusTone> = {
  pending: 'warning',
  processing: 'info',
  authorized: 'success',
  rejected: 'danger',
  cancelled: 'neutral',
  error: 'danger',
}

const TRANSITIONAL_STATUSES: ReadonlySet<NfseStatus> = new Set<NfseStatus>(['pending', 'processing'])

// Status desconhecido devolve o próprio valor — nunca "Desconhecido", que esconde a informação de quem está depurando.
export const nfseStatusLabel = (status: string): string => NFSE_STATUS_LABELS[status as NfseStatus] ?? status

export const nfseStatusTone = (status: string): NfseStatusTone => NFSE_STATUS_TONES[status as NfseStatus] ?? 'neutral'

export const isTransitionalNfseStatus = (status: NfseStatus): boolean => TRANSITIONAL_STATUSES.has(status)

export function NfseStatusBadge({status}: { status: NfseStatus }) {
  return (
    <StatusBadge
      label={nfseStatusLabel(status)}
      className={NFSE_STATUS_CLASSES[status] ?? 'bg-gray-100 text-gray-600'}
      isTransitional={isTransitionalNfseStatus(status)}
    />
  )
}

export function NfseStatusCell({status, sefazMotive}: { status: NfseStatus; sefazMotive: string | null }) {
  return (
    <StatusCell
      badge={<NfseStatusBadge status={status}/>}
      sefazMotive={sefazMotive}
      showMotive={status === 'rejected' || status === 'error'}
    />
  )
}
