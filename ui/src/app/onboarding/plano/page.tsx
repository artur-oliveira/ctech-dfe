'use client'

import {useEffect, useMemo, useState} from 'react'
import {useRouter} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {PlanChooser} from '@/components/billing/PlanChooser'
import {Button} from '@/components/ui/button'
import {buildPlanOptions} from '@/lib/billing/catalog'
import {ONBOARDING_ROOT, STEP_COMPANY, STEP_PLAN} from '@/lib/constants/onboarding'

function PlanStepContent() {
  const router = useRouter()
  const qc = useQueryClient()
  const [chosen, setChosen] = useState<string | null>(null)

  const plansQuery = useQuery({
    queryKey: queryKeys.billing.plans(),
    queryFn: () => apiClient.listBillingPlans(),
  })

  const options = useMemo(
    () => buildPlanOptions(plansQuery.data?.data ?? []),
    [plansQuery.data],
  )

  // The recommended plan leads the list and shows its allowance open, but the
  // choice is still derived rather than stored until the user makes one — and
  // the confirm button is what charges anything.
  const selected = chosen ?? options.find((o) => o.recommended)?.productId ?? options[0]?.productId ?? null

  // No-charge installations have nothing to sell; the layer disappears.
  const noCharge = plansQuery.data?.billing_enabled === false
  useEffect(() => {
    if (noCharge) router.replace(`${ONBOARDING_ROOT}/${STEP_COMPANY}`)
  }, [noCharge, router])

  const choose = useMutation({
    mutationFn: (productId: string) => {
      const option = options.find((o) => o.productId === productId)
      if (!option) throw new Error('Plano indisponível. Recarregue a página.')
      return apiClient.chooseBillingPlan({price_ids: option.priceIds})
    },
    onSuccess: async (result) => {
      await qc.invalidateQueries({queryKey: queryKeys.billing.subscription()})
      // An invoice comes back only when there is something to pay. Free and
      // on-demand skip checkout entirely and go straight on with setup.
      const checkoutUrl = result.invoice?.checkout_url
      if (checkoutUrl) {
        window.location.href = checkoutUrl
        return
      }
      router.push(`${ONBOARDING_ROOT}/${STEP_COMPANY}`)
    },
  })

  const selectedOption = options.find((o) => o.productId === selected)
  const paid = (selectedOption?.monthlyCents ?? 0) > 0

  return (
    <OnboardingShell
      current={STEP_PLAN}
      title="Escolha seu plano"
      description="Dá para trocar depois, a qualquer momento. O plano define quantos documentos você emite por mês e quantas empresas cabem na conta."
    >
      {plansQuery.isPending && (
        <div className="flex flex-col gap-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-24 rounded-xl bg-gray-100 motion-safe:animate-pulse"/>
          ))}
        </div>
      )}

      {plansQuery.error && (
        <div className="rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          Não foi possível carregar os planos. {plansQuery.error.message}
          <Button variant="outline" className="mt-3 w-full sm:w-auto" onClick={() => void plansQuery.refetch()}>
            Tentar de novo
          </Button>
        </div>
      )}

      {!plansQuery.isPending && !plansQuery.error && (
        <>
          <PlanChooser options={options} value={selected} onChange={setChosen}/>

          {choose.error && (
            <p className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {choose.error.message}
            </p>
          )}

          <div className="sticky bottom-0 -mx-4 mt-6 border-t border-gray-200 bg-gray-50/95 px-4 py-3 backdrop-blur md:-mx-8 md:px-8">
            <Button
              size="lg"
              className="w-full sm:w-auto"
              disabled={!selected || choose.isPending}
              onClick={() => selected && choose.mutate(selected)}
            >
              {choose.isPending ? 'Criando assinatura…' : paid ? 'Ir para o pagamento' : 'Continuar'}
            </Button>
            {paid && (
              <p className="mt-2 text-xs text-gray-500">
                Você vai para a tela de pagamento e volta para terminar a configuração.
              </p>
            )}
          </div>
        </>
      )}
    </OnboardingShell>
  )
}

export default function PlanStepPage() {
  return (
    <ProtectedRoute>
      <PlanStepContent/>
    </ProtectedRoute>
  )
}
