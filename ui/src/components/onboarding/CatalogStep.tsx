'use client'

import type {ReactNode} from 'react'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {Button} from '@/components/ui/button'
import type {OnboardingStep} from '@/lib/constants/onboarding'

interface CatalogStepProps {
  step: OnboardingStep
  title: string
  description: string
  /** What has been registered so far, newest last. */
  added: string[]
  /** Wording for the empty and the running count. */
  noun: {singular: string; plural: string}
  onSkip: () => void
  onDone: () => void
  children: ReactNode
}

/**
 * The optional catalogue layers: products for NF-e / NFC-e, services for NFS-e.
 *
 * They share one screen because they differ only in which form sits inside and
 * what the items are called. Both are genuinely optional — a company can issue
 * its first document with a single item registered — so the way out is a button,
 * not a hidden link.
 */
export function CatalogStep({step, title, description, added, noun, onSkip, onDone, children}: CatalogStepProps) {
  return (
    <OnboardingShell
      current={step}
      title={title}
      description={description}
      action={
        <Button variant="ghost" onClick={onSkip}>
          Pular por enquanto
        </Button>
      }
    >
      {added.length > 0 && (
        <div className="mb-4 rounded-xl border border-gray-200 bg-white p-4">
          <p className="text-sm font-medium text-gray-900">
            {added.length} {added.length === 1 ? noun.singular : noun.plural} cadastrado
            {added.length === 1 ? '' : 's'}
          </p>
          <ul className="mt-2 flex flex-wrap gap-1.5">
            {added.map((name, i) => (
              <li
                key={`${name}-${i}`}
                className="rounded-md bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700"
              >
                {name}
              </li>
            ))}
          </ul>
          <Button className="mt-4 w-full sm:w-auto" onClick={onDone}>
            Continuar
          </Button>
        </div>
      )}

      <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">{children}</div>
    </OnboardingShell>
  )
}
