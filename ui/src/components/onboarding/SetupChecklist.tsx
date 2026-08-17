'use client'

import Link from 'next/link'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {buttonVariants} from '@/components/ui/button'
import {cn} from '@/lib/utils'
import {STEP_DONE} from '@/lib/constants/onboarding'

/**
 * What is left of first-run setup, on the dashboard.
 *
 * The flow is resumable rather than mandatory past the first two layers, so
 * someone who left in the middle needs to see where they stopped — and someone
 * who finished needs this to disappear entirely, not linger as a permanent
 * reminder of a task already done.
 */
export function SetupChecklist() {
  const {steps, nextStep, isPending} = useOnboarding()

  const remaining = steps.filter((s) => !s.done && s.id !== STEP_DONE)
  if (isPending || remaining.length === 0 || !nextStep) return null

  const total = steps.filter((s) => s.id !== STEP_DONE).length
  const done = total - remaining.length

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-base font-semibold text-gray-900">Terminar a configuração</h2>
        <span className="text-sm text-gray-500 tabular-nums">
          {done} de {total}
        </span>
      </div>

      <ol className="mt-4 flex flex-col gap-2.5">
        {steps
          .filter((s) => s.id !== STEP_DONE)
          .map((step) => (
            <li key={step.id} className="flex items-center gap-3">
              <span
                aria-hidden="true"
                className={`flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${
                  step.done ? 'bg-brand-600 text-white' : 'border-2 border-gray-300 text-transparent'
                }`}
              >
                ✓
              </span>
              {step.done ? (
                <span className="text-sm text-gray-500 line-through">{step.title}</span>
              ) : (
                <Link
                  href={step.path}
                  className="text-sm text-gray-700 underline-offset-4 hover:text-gray-900 hover:underline"
                >
                  {step.title}
                  {step.optional && <span className="ml-1.5 text-xs text-gray-500">(opcional)</span>}
                </Link>
              )}
            </li>
          ))}
      </ol>

      <Link href={nextStep.path} className={cn(buttonVariants(), 'mt-4 w-full sm:w-auto')}>
        Continuar
      </Link>
    </section>
  )
}
