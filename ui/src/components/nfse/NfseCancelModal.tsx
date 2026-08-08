'use client'

import {useState} from 'react'
import {Modal} from '@/components/ui/modal'
import {OptionsSelect} from '@/components/ui/options-select'
import {JustificationField} from '@/components/ui/justification-field'
import {CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal'
import {ApiError} from '@/lib/api/client'
import {NFSE_CANCELLATION_MOTIVES} from '@/lib/data/nfse_motives'

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
        <p className="text-sm text-gray-600">
          O prazo é definido pelo município e não é informado pelo ADN. Se ele já tiver encerrado, o fisco recusará o pedido sem alterar a nota.
        </p>
        <div>
          <label htmlFor="nfse-cancel-code" className="block text-sm font-medium text-gray-700 mb-1.5">Motivo do cancelamento</label>
          <OptionsSelect id="nfse-cancel-code" value={reasonCode} onValueChange={setReasonCode}
                         options={[...NFSE_CANCELLATION_MOTIVES]} placeholder="Selecione o motivo"/>
        </div>
        <JustificationField
          id="nfse-cancel-description"
          value={reasonDescription}
          onChange={setReasonDescription}
          minLength={CANCEL_JUSTIFICATION_MIN_LENGTH}
          placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
        />
        {error != null && (
          <p role="alert" className="text-xs text-red-600">
            {error instanceof ApiError ? error.detail : 'Não foi possível cancelar a NFS-e. Tente novamente.'}
          </p>
        )}
      </div>
    </Modal>
  )
}
