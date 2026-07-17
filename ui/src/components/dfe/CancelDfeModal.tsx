'use client'

import {Modal} from '@/components/ui/modal'
import {JustificationField} from '@/components/ui/justification-field'

export const CANCEL_JUSTIFICATION_MIN_LENGTH = 15
export const CANCEL_JUSTIFICATION_MAX_LENGTH = 255

interface CancelDfeModalProps {
  isOpen: boolean
  /** 'NF-e' | 'NFC-e' — used in the title and body copy. */
  docLabel: string
  docNumber: string | number
  justification: string
  onJustificationChange: (value: string) => void
  onClose: () => void
  onConfirm: () => void
  loading: boolean
  error?: unknown
}

/** Shared cancel-document confirmation modal: irreversibility notice + justification field + error feedback. */
export function CancelDfeModal({
                                  isOpen, docLabel, docNumber, justification, onJustificationChange,
                                  onClose, onConfirm, loading, error,
                                }: CancelDfeModalProps) {
  return (
    <Modal
      isOpen={isOpen}
      title={`Cancelar ${docLabel} nº ${docNumber}`}
      onClose={onClose}
      onSubmit={onConfirm}
      submitLabel="Confirmar cancelamento"
      cancelLabel="Voltar"
      danger
      loading={loading}
      submitDisabled={justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH}
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          Esta ação é <span className="font-medium text-red-600">irreversível</span>. A {docLabel} será cancelada
          junto à SEFAZ e não poderá ser reativada.
        </p>
        <JustificationField
          id="cancel-justification"
          value={justification}
          onChange={onJustificationChange}
          minLength={CANCEL_JUSTIFICATION_MIN_LENGTH}
          maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
          placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
        />
        {error != null && (
          <p className="text-xs text-red-600">{(error as Error)?.message ?? 'Erro ao cancelar documento.'}</p>
        )}
      </div>
    </Modal>
  )
}
