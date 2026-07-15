'use client'

import {useId, useState} from 'react'
import {useSavedFilterViews, type SavedFilterView} from '@/lib/hooks/useSavedFilterViews'

/**
 * Saved named NSU filter views for a distribution page. Persists via
 * useSavedFilterViews (localStorage). Renders a trigger that opens a native
 * popover listing saved views (click to apply), with a save-row for the
 * current filter. Opens top-layer so it never clips inside the page.
 */
export function SavedFilterViews({
  pageId,
  currentNsu,
  onApply,
}: {
  pageId: string
  currentNsu: string
  onApply: (nsu: string) => void
}) {
  const popoverId = useId()
  const {views, saveView, deleteView} = useSavedFilterViews(pageId)
  const [name, setName] = useState('')

  const handleSave = () => {
    if (!name.trim()) return
    saveView(name, currentNsu)
    setName('')
  }

  return (
    <div className="relative inline-block">
      <button
        type="button"
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        {...({popovertarget: popoverId} as any)}
        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
      >
        <svg className="size-3.5" viewBox="0 0 20 20" fill="none" aria-hidden="true">
          <path d="M3 5h14M6 10h8M9 15h2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/>
        </svg>
        Visualizações{views.length > 0 && <span className="text-gray-400">({views.length})</span>}
      </button>

      <div
        id={popoverId}
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        {...({popover: 'auto'} as any)}
        role="dialog"
        aria-label="Visualizações de filtro salvas"
        className="gt-pop m-0 block w-64 rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-popover"
      >
        {views.length === 0 ? (
          <p className="px-1 py-2 text-gray-400">Nenhuma visualização salva.</p>
        ) : (
          <ul className="max-h-56 space-y-0.5 overflow-y-auto">
            {views.map((v: SavedFilterView) => (
              <li key={v.name} className="flex items-center justify-between gap-2 rounded-md px-1 py-1 hover:bg-gray-50">
                <button
                  type="button"
                  onClick={() => onApply(v.nsu)}
                  className="min-w-0 flex-1 text-left"
                >
                  <span className="block truncate font-medium text-gray-700">{v.name}</span>
                  <span className="block truncate font-mono text-gray-400">{v.nsu || 'todos'}</span>
                </button>
                <button
                  type="button"
                  onClick={() => deleteView(v.name)}
                  aria-label={`Excluir ${v.name}`}
                  className="shrink-0 rounded p-1 text-gray-400 hover:text-red-600"
                >
                  <svg className="size-3.5" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                    <path d="M5 6h10M8 6V4h4v2M6 6l1 10h6l1-10" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="mt-2 flex items-center gap-1.5 border-t border-gray-100 pt-2">
          <input
            value={name}
            onChange={e => setName(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                e.preventDefault()
                handleSave()
              }
            }}
            placeholder="Nome da visualização"
            className="h-7 min-w-0 flex-1 rounded-md border border-gray-200 px-2 text-xs text-gray-900 outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50"
          />
          <button
            type="button"
            onClick={handleSave}
            disabled={!name.trim()}
            className="h-7 shrink-0 rounded-md bg-brand-600 px-2 text-xs font-medium text-white transition-colors hover:bg-brand-700 disabled:opacity-50"
          >
            Salvar
          </button>
        </div>
      </div>
    </div>
  )
}
