'use client'

import {useState} from 'react'
import {Modal} from '@/components/ui/modal'
import {Input} from '@/components/ui/input'
import {JustificationField} from '@/components/ui/justification-field'
import {CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal'

interface NfseCancelModalProps {
  isOpen: boolean
  docNumber: string | number
  loading: boolean
  error?: unknown
  onClose: () => void
  onConfirm: (data: { reasonCode: string; reasonDescription: string }) => void
}

/**
 * Cancelamento de NFS-e (TE101101) exige cMotivo (código, até 2 chars) e
 * xMotivo (descrição) — diferente do cancelamento de NF-e/NFC-e/MDF-e, que
 * usa só uma justificativa. Por isso não reusa CancelDfeModal.
 */
export function NfseCancelModal({isOpen, docNumber, loading, error, onClose, onConfirm}: NfseCancelModalProps) {
  const [reasonCode, setReasonCode] = useState('')
  const [reasonDescription, setReasonDescription] = useState('')

  const canSubmit = reasonCode.trim().length > 0 && reasonDescription.trim().length >= CANCEL_JUSTIFICATION_MIN_LENGTH

  const handleClose = () => {
    setReasonCode('')
    setReasonDescription('')
    onClose()
  }

  return (
    <Modal
      isOpen={isOpen}
      title={`Cancelar NFS-e nº ${docNumber}`}
      onClose={handleClose}
      onSubmit={() => onConfirm({reasonCode: reasonCode.trim(), reasonDescription: reasonDescription.trim()})}
      submitLabel="Confirmar cancelamento"
      cancelLabel="Voltar"
      danger
      loading={loading}
      submitDisabled={!canSubmit}
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          Esta ação é <span className="font-medium text-red-600">irreversível</span>. A NFS-e será cancelada
          junto ao fisco municipal e não poderá ser reativada.
        </p>
        <div>
          <label htmlFor="nfse-cancel-code" className="block text-sm font-medium text-gray-700 mb-1.5">Código do motivo</label>
          <Input id="nfse-cancel-code" value={reasonCode} onChange={(e) => setReasonCode(e.target.value)} maxLength={2} className="w-20"/>
          <p className="mt-1 text-xs text-gray-400">Consulte o manual do contribuinte do município.</p>
        </div>
        <JustificationField
          id="nfse-cancel-description"
          value={reasonDescription}
          onChange={setReasonDescription}
          minLength={CANCEL_JUSTIFICATION_MIN_LENGTH}
          placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
        />
        {error != null && (
          <p className="text-xs text-red-600">{(error as Error)?.message ?? 'Erro ao cancelar NFS-e.'}</p>
        )}
      </div>
    </Modal>
  )
}
