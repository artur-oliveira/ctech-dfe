'use client'

import {useEffect, type ReactNode} from 'react'
import {usePathname, useRouter} from 'next/navigation'
import {useAuth} from '@/lib/hooks/useAuth'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {ONBOARDING_ROOT, STEP_CHECKOUT_RETURN, STEP_COMPANY, STEP_PLAN} from '@/lib/constants/onboarding'
import {STATUS_INCOMPLETE} from '@/lib/constants/billing'
import {ROLE_OWNER} from '@/lib/data/roles'

/** Routes that must stay reachable without a plan or a company. */
const EXEMPT_PREFIXES = [ONBOARDING_ROOT, '/invite', '/callback', '/login', '/terms-addendum']

/**
 * Sends an account that has not finished the required layers of setup into the
 * onboarding flow.
 *
 * Two rules keep this from catching the wrong people:
 *
 * - **Members are never gated.** Someone invited into an organization operates
 *   under the owner's plan and has no subscription of their own. Asking them to
 *   pick one would sell a second plan for the same company.
 * - **A failed lookup lets the user through.** The subscription is a
 *   convenience snapshot, and a network blip is not a reason to lock an account
 *   out of a product it already pays for. The API blocks issuance on its own
 *   side; this gate only decides where to point someone.
 */
export function OnboardingGate({children}: { children: ReactNode }) {
  const {user} = useAuth()
  const pathname = usePathname()
  const router = useRouter()
  const {subscription, isPending, error} = useSubscription()

  const exempt = EXEMPT_PREFIXES.some((p) => pathname.startsWith(p))
  const organizations = user?.organizations ?? []
  const memberOnly = organizations.length > 0 && !organizations.some((o) => o.role === ROLE_OWNER)

  const needsPlan = !!subscription && !subscription.has_subscription && !subscription.no_charge
  // INCOMPLETE is precisely "chose the paid plan and never paid". The account
  // has a subscription, so the plan step is answered; what is missing is the
  // payment landing, which is what the return screen waits for.
  const awaitingPayment = subscription?.status === STATUS_INCOMPLETE
  const needsCompany = !needsPlan && !awaitingPayment && organizations.length === 0

  const target = needsPlan
    ? `${ONBOARDING_ROOT}/${STEP_PLAN}`
    : awaitingPayment
      ? `${ONBOARDING_ROOT}/${STEP_CHECKOUT_RETURN}`
      : needsCompany
        ? `${ONBOARDING_ROOT}/${STEP_COMPANY}`
        : null

  const shouldRedirect = !exempt && !memberOnly && !error && !!target

  useEffect(() => {
    if (shouldRedirect && target) router.replace(target)
  }, [shouldRedirect, target, router])

  if (exempt || memberOnly || error) return <>{children}</>

  // Holding the page while the snapshot loads avoids a flash of the dashboard
  // for an account that is about to be redirected out of it.
  if (isPending || shouldRedirect) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div
          className="h-10 w-10 animate-spin rounded-full border-4 border-brand-100 border-t-brand-600 motion-reduce:animate-none"
          role="status"
          aria-label="Carregando"
        />
      </div>
    )
  }

  return <>{children}</>
}
