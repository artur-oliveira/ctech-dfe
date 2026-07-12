'use client'

import {type ReactNode, useState} from 'react'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {Modal} from '@/components/ui/modal'
import {Combobox} from '@/components/ui/combobox'
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status'
import {CITIES, CITY_OPTIONS} from '@/lib/data/cities'

const CANCEL_JUSTIFICATION_MIN_LENGTH = 15
const CANCEL_JUSTIFICATION_MAX_LENGTH = 255

/** Minimal shape needed to dispatch MDF-e cancel/close actions. */
export interface MdfeActionTarget {
  sk: string
  number: number | string
  /** Default UF for the close (encerramento) modal — usually the trip end UF. */
  uf_end?: string
}

/**
 * useMdfeActions centralizes the MDF-e cancel (110111) and close/encerramento
 * (110112) flows — mutations, modal state, and the rendered modals — so the list
 * and detail pages share one implementation. Render the returned `modals` once
 * and wire trigger buttons to `openCancel`/`openClose`.
 */
export function useMdfeActions(orgPk?: string) {
  const qc = useQueryClient()
  const [cancelTarget, setCancelTarget] = useState<MdfeActionTarget | null>(null)
  const [justification, setJustification] = useState('')
  const [closeTarget, setCloseTarget] = useState<MdfeActionTarget | null>(null)
  const [closeUf, setCloseUf] = useState('')
  const [closeMun, setCloseMun] = useState('')

  // Optimistically show the transitional status (the GSI is eventually
  // consistent); the WebSocket delivers the final status when the worker finishes.
  const patchStatus = (accessKey: string, status: 'cancel_pending' | 'close_pending') => {
    setDocStatusOptimistic(qc, queryKeys.mdfes.lists(orgPk), accessKey, status)
    void qc.invalidateQueries({queryKey: queryKeys.mdfes.detail(accessKey)})
  }

  const cancelMutation = useMutation({
    mutationFn: ({accessKey, justification}: { accessKey: string; justification: string }) =>
      apiClient.cancelMdfe(accessKey, justification),
    onSuccess: (_data, {accessKey}) => {
      setCancelTarget(null)
      setJustification('')
      patchStatus(accessKey, 'cancel_pending')
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao cancelar MDF-e.'),
  })

  const closeMutation = useMutation({
    mutationFn: ({accessKey, cMun, uf}: { accessKey: string; cMun: string; uf: string }) =>
      apiClient.closeMdfe(accessKey, cMun, uf || undefined),
    onSuccess: (_data, {accessKey}) => {
      setCloseTarget(null)
      setCloseMun('')
      setCloseUf('')
      patchStatus(accessKey, 'close_pending')
      toast.success('Encerramento enviado à SEFAZ.')
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao encerrar MDF-e.'),
  })

  const openCancel = (m: MdfeActionTarget) => {
    setJustification('')
    setCancelTarget(m)
  }
  const openClose = (m: MdfeActionTarget) => {
    setCloseMun('')
    setCloseUf(m.uf_end ?? '')
    setCloseTarget(m)
  }

  const handleConfirmCancel = () => {
    if (!cancelTarget || justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return
    cancelMutation.mutate({accessKey: cancelTarget.sk, justification: justification.trim()})
  }
  const handleConfirmClose = () => {
    if (!closeTarget || closeMun.length !== 7) return
    closeMutation.mutate({accessKey: closeTarget.sk, cMun: closeMun, uf: closeUf})
  }

  const modals: ReactNode = (
    <>
      <Modal
        isOpen={cancelTarget !== null}
        title={cancelTarget ? `Cancelar MDF-e nº ${cancelTarget.number}` : ''}
        onClose={() => {
          setCancelTarget(null);
          setJustification('')
        }}
        onSubmit={handleConfirmCancel}
        submitLabel="Confirmar cancelamento"
        cancelLabel="Voltar"
        danger
        loading={cancelMutation.isPending}
        submitDisabled={justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Esta ação é <span className="font-medium text-red-600">irreversível</span>. O MDF-e será cancelado junto à
            SEFAZ.
            Só é possível cancelar antes do início do transporte.
          </p>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">Justificativa</label>
            <textarea value={justification} onChange={(e) => setJustification(e.target.value)} rows={4}
                      maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
                      placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
                      className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-red-400 resize-none"/>
          </div>
        </div>
      </Modal>

      <Modal
        isOpen={closeTarget !== null}
        title={closeTarget ? `Encerrar MDF-e nº ${closeTarget.number}` : ''}
        onClose={() => {
          setCloseTarget(null);
          setCloseMun('');
          setCloseUf('')
        }}
        onSubmit={handleConfirmClose}
        submitLabel="Confirmar encerramento"
        cancelLabel="Voltar"
        loading={closeMutation.isPending}
        submitDisabled={closeMun.length !== 7 || !closeUf}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            O encerramento informa à SEFAZ que o transporte terminou. Informe o município onde a viagem foi concluída.
          </p>
          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Município de encerramento</label>
            <Combobox
              value={closeMun}
              onValueChange={(code) => {
                setCloseMun(code)
                const city = CITIES.find((c) => c.code === code)
                setCloseUf(city?.uf ?? '')
              }}
              options={CITY_OPTIONS}
              placeholder="Selecione o município"
              searchPlaceholder="Buscar município…"
            />
          </div>
        </div>
      </Modal>
    </>
  )

  return {openCancel, openClose, modals}
}
