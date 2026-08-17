'use client'

import {useMemo, useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {Modal} from '@/components/ui/modal'
import {PlanChooser} from '@/components/billing/PlanChooser'
import {buildPlanOptions} from '@/lib/billing/catalog'
import {formatCents, PLAN_ONDEMAND} from '@/lib/constants/billing'
import type {AccountSubscription} from '@/lib/types/billing'

interface ChangePlanDialogProps {
  isOpen: boolean
  onClose: () => void
  subscription: AccountSubscription
}

/**
 * Plan change from an account that already has one.
 *
 * The exact prorated amount is not shown before confirming, and that is a real
 * gap rather than a decision: ctech-billing prorates on the change itself and
 * publishes no preview endpoint, so any number rendered here would be this
 * screen's arithmetic rather than the invoice's. What it does instead is state
 * the rule and the new monthly price plainly, and send the user straight to the
 * charge, where the amount is billing's own.
 */
export function ChangePlanDialog({isOpen, onClose, subscription}: ChangePlanDialogProps) {
  const qc = useQueryClient()
  const [chosen, setChosen] = useState<string | null>(null)

  const plansQuery = useQuery({
    queryKey: queryKeys.billing.plans(),
    queryFn: () => apiClient.listBillingPlans(),
    enabled: isOpen,
  })

  const options = useMemo(() => buildPlanOptions(plansQuery.data?.data ?? []), [plansQuery.data])
  const selected = options.find((o) => o.productId === chosen) ?? null
  const isCurrent = selected?.plan === subscription.plan

  const change = useMutation({
    mutationFn: (priceIds: string[]) => apiClient.changeBillingPlan({price_ids: priceIds}),
    onSuccess: async (result) => {
      await qc.invalidateQueries({queryKey: queryKeys.billing.subscription()})
      const checkoutUrl = result.invoice?.checkout_url
      if (checkoutUrl) {
        window.location.href = checkoutUrl
        return
      }
      onClose()
    },
  })

  return (
    <Modal
      isOpen={isOpen}
      title="Mudar de plano"
      size="lg"
      onClose={onClose}
      onSubmit={() => selected && change.mutate(selected.priceIds)}
      submitLabel={change.isPending ? 'Mudando…' : 'Confirmar mudança'}
      submitDisabled={!selected || isCurrent || change.isPending}
      loading={change.isPending}
    >
      {plansQuery.isPending ? (
        <div className="flex flex-col gap-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-20 rounded-xl bg-gray-100 motion-safe:animate-pulse"/>
          ))}
        </div>
      ) : (
        <>
          <PlanChooser
            options={options}
            value={chosen}
            onChange={setChosen}
            currentPlan={subscription.plan}
          />

          {selected && !isCurrent && (
            <p className="mt-4 rounded-lg bg-gray-50 px-4 py-3 text-sm leading-relaxed text-gray-600">
              {selected.plan === PLAN_ONDEMAND || selected.monthlyCents === 0
                ? 'A partir da mudança você passa a ser cobrado pelo novo plano. O que já foi pago deste ciclo vira crédito na próxima fatura.'
                : `A mensalidade passa a ser ${formatCents(selected.monthlyCents)}. Nesta fatura entra apenas a diferença proporcional aos dias que faltam do ciclo atual — o valor exato aparece na tela de pagamento, antes de qualquer cobrança.`}
            </p>
          )}

          {change.error && (
            <p role="alert" className="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger">
              {change.error.message}
            </p>
          )}
        </>
      )}
    </Modal>
  )
}
