'use client'

import {ReactNode, startTransition, useEffect, useState} from 'react'
import {useAuth} from '@/lib/hooks/useAuth'
import {startOAuthFlow} from '@/lib/auth/oauth'
import {TermsAddendumGate} from '@/components/terms-addendum-gate'
import {OnboardingGate} from '@/components/onboarding/OnboardingGate'

const OAUTH_ATTEMPT_KEY = 'oauth_last_attempt_ms'
const OAUTH_DEBOUNCE_MS = 15_000

export function ProtectedRoute({children}: { children: ReactNode }) {
  const {user, loading} = useAuth()
  const [blocked, setBlocked] = useState(false)

  useEffect(() => {
    if (!loading && !user) {
      const last = Number(sessionStorage.getItem(OAUTH_ATTEMPT_KEY) ?? 0)
      if (Date.now() - last < OAUTH_DEBOUNCE_MS) {
        startTransition(() => setBlocked(true))
        return
      }
      sessionStorage.setItem(OAUTH_ATTEMPT_KEY, String(Date.now()))
      void startOAuthFlow(window.location.pathname)
    }
  }, [user, loading])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <div
            className="w-12 h-12 border-4 border-primary-200 border-t-primary-600 rounded-full animate-spin mx-auto mb-4"></div>
          <p className="text-gray-600">Carregando...</p>
        </div>
      </div>
    )
  }

  if (!user) {
    if (blocked) {
      return (
        <div className="flex items-center justify-center min-h-screen">
          <div className="text-center space-y-4 max-w-sm px-4">
            <p className="text-red-600 text-sm">Falha na autenticação. Verifique a configuração do servidor.</p>
            <button
              className="text-primary-600 underline text-sm"
              onClick={() => {
                sessionStorage.removeItem(OAUTH_ATTEMPT_KEY)
                void startOAuthFlow(window.location.pathname)
              }}
            >
              Tentar novamente
            </button>
          </div>
        </div>
      )
    }
    return null
  }

  if (!user.terms_addendum_accepted) {
    return <TermsAddendumGate/>
  }

  // Order matters: the addendum is a legal precondition to using the product at
  // all, so it is answered before anyone is asked to choose a plan.
  return <OnboardingGate>{children}</OnboardingGate>
}
