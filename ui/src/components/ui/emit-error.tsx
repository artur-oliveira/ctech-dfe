'use client';

import {useEffect, useRef} from 'react';
import Link from 'next/link';
import type {EmitAction, EmitFailure} from '@/lib/billing/notice';

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
 * person ends up phoning support. A local 422 (layout length, MOC rule,
 * incomplete record) lists each refusal with its fix next to it.
 */
export function EmitError({failure}: { failure: EmitFailure | null }) {
  const ref = useRef<HTMLDivElement>(null);
  const message = failure?.message ?? null;

  useEffect(() => {
    if (!message) return;
    ref.current?.scrollIntoView?.({
      block: 'nearest',
      behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    });
  }, [message]);

  if (!failure) return null;

  return (
    <div
      ref={ref}
      role="alert"
      aria-live="assertive"
      className="space-y-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger"
    >
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <span className="text-pretty">{failure.message}</span>
        {failure.action && <ActionLink action={failure.action}/>}
      </div>
      {failure.issues && (
        <ul className="space-y-2 border-t border-red-200 pt-3">
          {failure.issues.map((issue, i) => (
            <li key={i} className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
              <span className="min-w-0 text-pretty">
                <span className="text-gray-900">{issue.text}</span>
                {issue.hint && <span className="mt-0.5 block text-gray-700">{issue.hint}</span>}
              </span>
              {issue.action && <ActionLink action={issue.action}/>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** A fix that lives on another screen is a link; one in this form is a button that jumps to the block. */
function ActionLink({action}: { action: EmitAction }) {
  const className = 'inline-flex min-h-11 shrink-0 items-center font-semibold text-danger underline underline-offset-4 sm:min-h-0';
  if ('href' in action) return <Link href={action.href} className={className}>{action.label}</Link>;
  return <button type="button" onClick={action.onClick} className={className}>{action.label}</button>;
}
