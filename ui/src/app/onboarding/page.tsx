'use client'

import {useEffect} from 'react'
import {useRouter} from 'next/navigation'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {ONBOARDING_ROOT, STEP_DONE} from '@/lib/constants/onboarding'

/** `/onboarding` resumes wherever setup stopped, so the flow has one entry. */
function OnboardingEntry() {
  const router = useRouter()
  const {nextStep, isPending} = useOnboarding()

  useEffect(() => {
    if (isPending) return
    router.replace(nextStep?.path ?? `${ONBOARDING_ROOT}/${STEP_DONE}`)
  }, [isPending, nextStep, router])

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

export default function OnboardingPage() {
  return (
    <ProtectedRoute>
      <OnboardingEntry/>
    </ProtectedRoute>
  )
}
