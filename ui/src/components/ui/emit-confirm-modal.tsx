'use client'

import type {ReactNode} from 'react'
import {Modal} from '@/components/ui/modal'

interface EmitConfirmModalProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  docLabel: string
  summary: {label: string; value: ReactNode}[]
}

/** Confirmation step in front of any SEFAZ emission — restates the summary before an irreversible submit. */
export function EmitConfirmModal({open, onClose, onConfirm, docLabel, summary}: EmitConfirmModalProps) {
  return (
    <Modal isOpen={open} title={`Confirmar emissão de ${docLabel}`} onClose={onClose} onSubmit={onConfirm}
           submitLabel="Confirmar e emitir" cancelLabel="Revisar">
      <div className="space-y-3">
        <p className="text-sm text-gray-600">
          Esta ação envia o documento à SEFAZ e não pode ser desfeita diretamente. Confira os dados antes de
          confirmar.
        </p>
        <dl className="rounded-lg border border-gray-200 bg-gray-50 divide-y divide-gray-100">
          {summary.map(({label, value}) => (
            <div key={label} className="flex items-center justify-between px-4 py-2 text-sm">
              <dt className="text-gray-500">{label}</dt>
              <dd className="font-medium text-gray-900 text-right">{value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </Modal>
  )
}
