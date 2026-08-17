'use client'

import {useEffect, useState} from 'react'
import {useRouter} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {Button} from '@/components/ui/button'
import {formatCents} from '@/lib/constants/billing'
import {ONBOARDING_ROOT, STEP_COMPANY, STEP_PLAN} from '@/lib/constants/onboarding'

/** How often the settlement snapshot is re-read while the user waits. */
const POLL_INTERVAL_MS = 3_000

/**
 * How long the screen waits before saying so out loud.
 *
 * A PIX usually settles in seconds, but "usually" is not a guarantee, and a
 * screen with no exit is worse than a slow payment: it turns a wait into a
 * support call. After a minute this stops pretending and offers a way forward.
 */
const POLL_CEILING_MS = 60_000

function CheckoutReturnContent() {
  const router = useRouter()
  const [gaveUpWaiting, setGaveUpWaiting] = useState(false)

  const {data: subscription} = useQuery({
    queryKey: queryKeys.billing.subscription(),
    queryFn: () => apiClient.getSubscription(),
    refetchInterval: gaveUpWaiting ? false : POLL_INTERVAL_MS,
  })

  useEffect(() => {
    const timer = window.setTimeout(() => setGaveUpWaiting(true), POLL_CEILING_MS)
    return () => window.clearTimeout(timer)
  }, [])

  const settled = subscription?.grants_service === true
  useEffect(() => {
    if (settled) router.replace(`${ONBOARDING_ROOT}/${STEP_COMPANY}`)
  }, [settled, router])

  const invoice = subscription?.open_invoice
  const noSubscription = subscription?.has_subscription === false

  if (noSubscription) {
    return (
      <OnboardingShell
        current={STEP_PLAN}
        title="Nenhuma assinatura em aberto"
        description="A assinatura não chegou a ser criada. Escolha um plano para continuar."
      >
        <Button size="lg" className="w-full sm:w-auto" onClick={() => router.replace(`${ONBOARDING_ROOT}/${STEP_PLAN}`)}>
          Escolher plano
        </Button>
      </OnboardingShell>
    )
  }

  return (
    <OnboardingShell
      current={STEP_PLAN}
      title={gaveUpWaiting ? 'Ainda não recebemos o pagamento' : 'Confirmando seu pagamento'}
      description={
        gaveUpWaiting
          ? 'Um PIX pode levar alguns minutos para cair. Assim que cair, sua assinatura fica ativa e avisamos por e-mail — você não precisa esperar aqui.'
          : 'Assim que o pagamento cair, seguimos para o cadastro da empresa.'
      }
    >
      <div className="rounded-xl border border-gray-200 bg-white p-5">
        {!gaveUpWaiting && (
          <div className="flex items-center gap-3">
            <div
              className="h-5 w-5 shrink-0 animate-spin rounded-full border-2 border-brand-100 border-t-brand-600 motion-reduce:animate-none"
              role="status"
              aria-label="Aguardando confirmação"
            />
            <p className="text-sm text-gray-600">Aguardando a confirmação do banco…</p>
          </div>
        )}

        {invoice && (
          <div className={gaveUpWaiting ? '' : 'mt-4 border-t border-gray-100 pt-4'}>
            <div className="flex items-baseline justify-between gap-4">
              <span className="text-sm text-gray-600">Fatura em aberto</span>
              <span className="text-base font-semibold text-gray-900 tabular-nums">
                {formatCents(invoice.total_cents)}
              </span>
            </div>
            {invoice.checkout_url && (
              <Button
                variant="outline"
                className="mt-4 w-full sm:w-auto"
                onClick={() => {
                  window.location.href = invoice.checkout_url!
                }}
              >
                Abrir a tela de pagamento
              </Button>
            )}
          </div>
        )}
      </div>

      <div className="sticky bottom-0 -mx-4 mt-6 border-t border-gray-200 bg-gray-50/95 px-4 py-3 backdrop-blur md:-mx-8 md:px-8">
        <Button
          variant="outline"
          size="lg"
          className="w-full sm:w-auto"
          onClick={() => router.push(`${ONBOARDING_ROOT}/${STEP_COMPANY}`)}
        >
          Continuar a configuração
        </Button>
        <p className="mt-2 text-xs text-gray-500">
          Você já pode cadastrar a empresa. A emissão libera quando o pagamento for confirmado.
        </p>
      </div>
    </OnboardingShell>
  )
}

export default function CheckoutReturnPage() {
  return (
    <ProtectedRoute>
      <CheckoutReturnContent/>
    </ProtectedRoute>
  )
}
