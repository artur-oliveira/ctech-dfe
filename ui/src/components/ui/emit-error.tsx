'use client'

import {useEffect, useRef} from 'react'

/**
 * Emission failure banner, shared by every DF-e emit form.
 *
 * The emit button lives in the sticky action bar at the bottom of a long form,
 * so an error rendered at the top of the page is invisible exactly when it
 * matters. This renders adjacent to the action bar, announces itself
 * (`role="alert"` + `aria-live="assertive"`) and scrolls itself into view when
 * the message changes.
 */
export function EmitError({message}: { message: string | null }) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!message) return
    ref.current?.scrollIntoView?.({
      block: 'nearest',
      behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    })
  }, [message])

  if (!message) return null

  return (
    <div
      ref={ref}
      role="alert"
      aria-live="assertive"
      className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger"
    >
      {message}
    </div>
  )
}
