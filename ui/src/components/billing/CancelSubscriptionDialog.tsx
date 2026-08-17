'use client'

import {useState} from 'react'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {Modal} from '@/components/ui/modal'
import {formatISODateBR} from '@/lib/utils/dfe'
import type {AccountSubscription} from '@/lib/types/billing'

interface CancelSubscriptionDialogProps {
  isOpen: boolean
  onClose: () => void
  subscription: AccountSubscription
}

const WHEN_PERIOD_END = 'period_end'
const WHEN_IMMEDIATE = 'immediate'

/**
 * Cancellation, with the two kinds kept apart.
 *
 * At the period end the customer keeps what they already paid for; immediately
 * they give it up. They are different operations, so they are two choices with
 * their consequence written next to them — and the immediate one needs its own
 * acknowledgement, because "cancelar" clicked in a hurry must not stop today's
 * issuance by accident.
 */
export function CancelSubscriptionDialog({isOpen, onClose, subscription}: CancelSubscriptionDialogProps) {
  const qc = useQueryClient()
  const [when, setWhen] = useState<string>(WHEN_PERIOD_END)
  const [acknowledged, setAcknowledged] = useState(false)

  const immediate = when === WHEN_IMMEDIATE

  const cancel = useMutation({
    mutationFn: () => apiClient.cancelBillingSubscription(!immediate),
    onSuccess: async () => {
      await qc.invalidateQueries({queryKey: queryKeys.billing.subscription()})
      onClose()
    },
  })

  const periodEnd = subscription.period_end ? formatISODateBR(subscription.period_end) : null

  return (
    <Modal
      isOpen={isOpen}
      title="Cancelar assinatura"
      onClose={onClose}
      onSubmit={() => cancel.mutate()}
      submitLabel={cancel.isPending ? 'Cancelando…' : 'Cancelar assinatura'}
      cancelLabel="Manter assinatura"
      submitDisabled={cancel.isPending || (immediate && !acknowledged)}
      loading={cancel.isPending}
      danger
    >
      <fieldset className="flex flex-col gap-3">
        <legend className="sr-only">Quando cancelar</legend>

        {[
          {
            value: WHEN_PERIOD_END,
            label: periodEnd ? `No fim do período, em ${periodEnd}` : 'No fim do período pago',
            help: 'Você continua emitindo até lá e não há nova cobrança. Dá para voltar atrás a qualquer momento antes dessa data.',
          },
          {
            value: WHEN_IMMEDIATE,
            label: 'Agora',
            help: 'A emissão para imediatamente e o restante do período já pago não é devolvido. Os documentos já emitidos continuam disponíveis para consulta e download.',
          },
        ].map((option) => (
          <label
            key={option.value}
            className={`flex cursor-pointer gap-3 rounded-xl border p-4 ${
              when === option.value ? 'border-gray-900 bg-gray-50' : 'border-gray-200'
            }`}
          >
            <input
              type="radio"
              name="cancel-when"
              value={option.value}
              checked={when === option.value}
              onChange={() => {
                setWhen(option.value)
                setAcknowledged(false)
              }}
              className="mt-1 size-4 shrink-0 accent-gray-900"
            />
            <span className="min-w-0">
              <span className="block text-sm font-medium text-gray-900">{option.label}</span>
              <span className="mt-1 block text-sm leading-relaxed text-gray-600">{option.help}</span>
            </span>
          </label>
        ))}
      </fieldset>

      {immediate && (
        <label className="mt-4 flex cursor-pointer items-start gap-3 rounded-xl border border-red-200 bg-red-50 p-4">
          <input
            type="checkbox"
            checked={acknowledged}
            onChange={(e) => setAcknowledged(e.target.checked)}
            className="mt-0.5 size-4 shrink-0 accent-red-600"
          />
          <span className="text-sm leading-relaxed text-danger">
            Entendi que a emissão para agora e que não há devolução do período já pago.
          </span>
        </label>
      )}

      {cancel.error && (
        <p role="alert" className="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger">
          {cancel.error.message}
        </p>
      )}
    </Modal>
  )
}
