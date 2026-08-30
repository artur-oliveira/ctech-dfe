import React from 'react'
import {Button} from '@/components/ui/button'

/**
 * The screen that shows when there is no screen: 404, an uncaught error, a
 * server in maintenance.
 *
 * One component for all three because they are the same moment — something did
 * not happen and the person needs to know what, and what to do next. Three
 * separately-invented layouts is how a product ends up with three different
 * ideas of what a dead end looks like.
 *
 * Deliberately dependency-free: no hooks, no context, no data. `global-error`
 * renders it outside the provider tree, where anything else would fail for the
 * same reason the boundary caught.
 */
export function SystemState({code, title, description, detail, children}: {
  code: '404' | '500' | '503'
  title: string
  description: string
  /** The line under the description — an error reference, a retry hint. */
  detail?: React.ReactNode
  /** The actions. The caller owns them: what to do next differs per code. */
  children?: React.ReactNode
}) {
  return (
    <main className="min-h-screen bg-gray-50 flex items-center justify-center px-4 py-12">
      <div className="text-center max-w-md">
        <p className="text-7xl font-bold text-gray-200 select-none" aria-hidden="true">{code}</p>
        <h1 className="mt-4 text-xl font-semibold text-gray-900 text-balance">{title}</h1>
        <p className="mt-2 text-sm text-gray-500 text-pretty">{description}</p>
        {detail && <p className="mt-3 text-xs text-gray-400 text-pretty">{detail}</p>}
        {children && <div className="mt-6 flex flex-wrap items-center justify-center gap-2">{children}</div>}
      </div>
    </main>
  )
}

/** The action every one of these screens ends with. */
export function SystemStateRetry({onRetry, label = 'Tentar novamente'}: { onRetry: () => void; label?: string }) {
  return (
    <Button variant="brand" onClick={onRetry}>{label}</Button>
  )
}
