'use client'

import {useEffect, useRef} from 'react'
import Link from 'next/link'
import type {EmitFailure} from '@/lib/billing/notice'

/**
 * Emission failure banner, shared by every DF-e emit form.
 *
 * The emit button lives in the sticky action bar at the bottom of a long form,
 * so an error rendered at the top of the page is invisible exactly when it
 * matters. This renders adjacent to the action bar, announces itself
 * (`role="alert"` + `aria-live="assertive"`) and scrolls itself into view when
 * the message changes.
 *
 * A failure the customer can fix — a quota that ran out, a lapsed subscription —
 * carries the link that fixes it, because a refusal with no way forward is how a
 * person ends up phoning support.
 */
export function EmitError({failure}: { failure: EmitFailure | null }) {
  const ref = useRef<HTMLDivElement>(null)
  const message = failure?.message ?? null

  useEffect(() => {
    if (!message) return
    ref.current?.scrollIntoView?.({
      block: 'nearest',
      behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    })
  }, [message])

  if (!failure) return null

  return (
    <div
      ref={ref}
      role="alert"
      aria-live="assertive"
      className="flex flex-col gap-2 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger sm:flex-row sm:items-center sm:justify-between"
    >
      <span className="text-pretty">{failure.message}</span>
      {failure.action && (
        <Link
          href={failure.action.href}
          className="shrink-0 font-semibold text-danger underline underline-offset-4"
        >
          {failure.action.label}
        </Link>
      )}
    </div>
  )
}
