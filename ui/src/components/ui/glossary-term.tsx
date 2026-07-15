'use client'

import React, {useId, useRef} from 'react'
import {cn} from '@/lib/utils'
import {GLOSSARY, type GlossaryKey} from '@/lib/constants/glossary'

/**
 * Inline glossary hint for fiscal jargon. Renders `children` (usually the field
 * label) followed by a small "?" button. Tapping/clicking it opens a native
 * popover with a plain-language definition.
 *
 * Uses the HTML Popover API so the panel renders in the top layer — it never
 * gets clipped by a form's `overflow` (unlike an absolutely-positioned tooltip)
 * and works on touch (unlike `title`). CSS anchor positioning (position-area)
 * is Chrome-only, so we position the panel under the button with JS on open —
 * works in Firefox/Safari too. Top-layer coords are viewport-relative, so the
 * button's getBoundingClientRect maps directly to the panel's fixed position.
 */
const GAP = 6 // px between button and panel

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
    const btnRef = useRef<HTMLButtonElement>(null)
    const popRef = useRef<HTMLSpanElement>(null)
    const {label, definition} = GLOSSARY[term]

    function place() {
        const btn = btnRef.current
        const pop = popRef.current
        if (!btn || !pop) return
        const r = btn.getBoundingClientRect()
        const left = Math.max(GAP, Math.min(r.left, window.innerWidth - pop.offsetWidth - GAP))
        const top = Math.min(r.bottom + GAP, window.innerHeight - pop.offsetHeight - GAP)
        pop.style.left = `${left}px`
        pop.style.top = `${Math.max(GAP, top)}px`
    }

    return (
        <span className={cn('inline-flex items-center gap-1', className)}>
      {children}
            <button
                ref={btnRef}
                type="button"
                popoverTarget={id}
                aria-label={`O que é ${label}?`}
                className="inline-grid size-4 place-items-center rounded-full border border-gray-300 text-xs font-semibold leading-none text-gray-500 transition-colors hover:border-brand-500 hover:text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
            >
        ?
      </button>
      <span
          ref={popRef}
          id={id}
          popover="auto"
          role="tooltip"
          onToggle={(e) => e.newState === 'open' && place()}
          className="gt-pop fixed m-0 block max-w-xs rounded-lg border border-gray-200 bg-white p-3 text-xs leading-relaxed text-gray-700 shadow-popover"
      >
        <span className="mb-1 block text-xs font-semibold tracking-normal text-gray-900 normal-case">{label}</span>
          {definition}
      </span>
    </span>
    )
}
