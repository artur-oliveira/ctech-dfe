'use client'

import Image from 'next/image'
import type {ReactNode} from 'react'
import {Button} from '@/components/ui/button'
import {StepIndicator} from '@/components/ui/step-indicator'
import {useAuth} from '@/lib/hooks/useAuth'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import type {OnboardingStep} from '@/lib/constants/onboarding'

interface OnboardingShellProps {
  current: OnboardingStep
  title: string
  /** One line saying what this layer buys the user. Never instructions. */
  description?: ReactNode
  children: ReactNode
  /** Right-hand escape for optional layers. */
  action?: ReactNode
}

/**
 * The chrome for first-run setup.
 *
 * Deliberately not `RootLayout`: at the first two layers there is no
 * organization yet, so a sidebar of document types would be a menu where every
 * item is a dead end. The flow gets a single column, the step rail, and one way
 * out — signing out.
 */
export function OnboardingShell({current, title, description, children, action}: OnboardingShellProps) {
  const {logout} = useAuth()
  const {steps} = useOnboarding()

  return (
    <div className="min-h-screen bg-gray-50/60">
      <header className="sticky top-0 z-20 border-b border-gray-200 bg-white">
        <div className="mx-auto flex h-14 w-full max-w-2xl items-center justify-between px-4">
          <div className="flex items-center gap-2.5">
            <Image src="/app.svg" alt="" aria-hidden="true" width={26} height={26} className="shrink-0" unoptimized/>
            <span className="text-base font-semibold tracking-tight text-gray-900">CTech DF-e</span>
          </div>
          <Button variant="ghost" onClick={() => void logout('/')}>
            Sair
          </Button>
        </div>
      </header>

      <main className="mx-auto w-full max-w-2xl px-4 pb-16 pt-6 md:pt-10">
        <StepIndicator current={current} steps={steps.map((s) => ({id: s.id, label: s.label}))}/>

        <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold tracking-tight text-gray-900 text-balance">{title}</h1>
            {description && <p className="mt-1.5 text-sm leading-relaxed text-gray-600 text-pretty">{description}</p>}
          </div>
          {action && <div className="shrink-0">{action}</div>}
        </div>

        {children}
      </main>
    </div>
  )
}
