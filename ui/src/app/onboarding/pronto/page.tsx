'use client'

import Link from 'next/link'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {buttonVariants} from '@/components/ui/button'
import {cn} from '@/lib/utils'
import {useAuth} from '@/lib/hooks/useAuth'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {DOCUMENT_METERS, METER_LABELS} from '@/lib/constants/billing'
import {FIRST_ISSUANCE_PATH, STEP_DONE} from '@/lib/constants/onboarding'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

function DoneStepContent() {
  const {selectedOrg} = useAuth()
  const {configured} = useOnboarding()
  const {subscription} = useSubscription()

  const ready = DOCUMENT_METERS.filter((m) => configured[m as DocVariant])
  const primary = ready[0] as DocVariant | undefined
  const blocked = subscription && !subscription.grants_service && !subscription.no_charge

  return (
    <OnboardingShell
      current={STEP_DONE}
      title="Tudo pronto"
      description={
        selectedOrg
          ? `${selectedOrg.name} está configurada e pode emitir.`
          : 'Sua conta está configurada.'
      }
    >
      <div className="rounded-xl border border-gray-200 bg-white p-5">
        <p className="text-sm font-medium text-gray-900">Documentos habilitados</p>
        <ul className="mt-3 flex flex-col gap-2">
          {ready.map((meter) => (
            <li key={meter} className="flex items-center justify-between gap-3">
              <span className="flex items-center gap-2 text-sm text-gray-700">
                <span aria-hidden="true" className="text-success">✓</span>
                {METER_LABELS[meter]}
              </span>
              <Link
                href={FIRST_ISSUANCE_PATH[meter as DocVariant]}
                className="text-sm font-medium text-brand-700 underline-offset-4 hover:underline"
              >
                Emitir
              </Link>
            </li>
          ))}
        </ul>
      </div>

      {blocked && (
        <p className="mt-4 rounded-lg border border-warning/30 bg-amber-50 px-4 py-3 text-sm leading-relaxed text-warning">
          A configuração está completa, mas a emissão libera quando o pagamento da assinatura for confirmado.
        </p>
      )}

      <div className="sticky bottom-0 -mx-4 mt-6 flex flex-col gap-2 border-t border-gray-200 bg-gray-50/95 px-4 py-3 backdrop-blur sm:flex-row md:-mx-8 md:px-8">
        {primary && (
          <Link
            href={FIRST_ISSUANCE_PATH[primary]}
            className={cn(buttonVariants({size: 'lg'}), 'w-full sm:w-auto')}
          >
            Emitir a primeira {METER_LABELS[primary]}
          </Link>
        )}
        <Link
          href="/dashboard"
          className={cn(buttonVariants({variant: 'outline', size: 'lg'}), 'w-full sm:w-auto')}
        >
          Ir para o painel
        </Link>
      </div>
    </OnboardingShell>
  )
}

export default function DoneStepPage() {
  return (
    <ProtectedRoute>
      <DoneStepContent/>
    </ProtectedRoute>
  )
}
