'use client'

import {useId} from 'react'
import {cn} from '@/lib/utils'
import {GLOSSARY, type GlossaryKey} from '@/lib/constants/glossary'

/**
 * Inline glossary hint for fiscal jargon. Renders `children` (usually the field
 * label) followed by a small "?" button. Tapping/clicking it opens a native
 * popover with a plain-language definition.
 *
 * Uses the HTML Popover API so the panel renders in the top layer — it never
 * gets clipped by a form's `overflow` (unlike an absolutely-positioned tooltip)
 * and works on touch (unlike `title`). The invoker button is the popover's
 * implicit anchor; `.gt-pop` (globals.css) positions it under the button where
 * CSS anchor positioning is supported, and centers it as a graceful fallback.
 */
export function GlossaryTerm({
  term,
  children,
  className,
}: {
  term: GlossaryKey
  children?: React.ReactNode
  className?: string
}) {
  const id = useId()
  const {label, definition} = GLOSSARY[term]
  return (
    <span className={cn('inline-flex items-center gap-1', className)}>
      {children}
      <button
        type="button"
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- popoverTarget typing lands in a later @types/react
        {...({popoverTarget: id} as any)}
        aria-label={`O que é ${label}?`}
        className="inline-grid size-4 place-items-center rounded-full border border-gray-300 text-xs font-semibold leading-none text-gray-500 transition-colors hover:border-brand-500 hover:text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
      >
        ?
      </button>
      <span
        id={id}
        // eslint-disable-next-line @typescript-eslint/no-explicit-any -- popover attribute typing lands in a later @types/react
        {...({popover: 'auto'} as any)}
        role="tooltip"
        className="gt-pop m-0 block max-w-xs rounded-lg border border-gray-200 bg-white p-3 text-xs leading-relaxed text-gray-700 shadow-lg"
      >
        <span className="mb-1 block text-xs font-semibold tracking-normal text-gray-900 normal-case">{label}</span>
        {definition}
      </span>
    </span>
  )
}
