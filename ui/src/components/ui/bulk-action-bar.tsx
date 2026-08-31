'use client'

import {Button} from '@/components/ui/button'

/**
 * Sticky bar shown when one or more rows are selected. Renders the selection
 * count, a clear button, and caller-supplied action buttons (e.g. bulk delete).
 * Edge-to-edge on mobile per the UI mobile-first rules.
 */
export function BulkActionBar({
  count,
  onClear,
  children,
}: {
  count: number
  onClear: () => void
  /** Action buttons for the selection (e.g. delete). */
  children?: React.ReactNode
}) {
  if (count === 0) return null

  return (
    <div className="sticky bottom-(--bottomnav-height) z-10 -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-white px-4 py-3 sm:flex-row sm:items-center sm:justify-between md:-mx-8 md:px-8">
      <div className="flex items-center gap-3">
        <span className="text-sm font-medium text-gray-900">
          {count} {count === 1 ? 'selecionado' : 'selecionados'}
        </span>
        <Button variant="ghost" size="xs" onClick={onClear} className="text-gray-500 hover:text-gray-700">
          Limpar
        </Button>
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  )
}
