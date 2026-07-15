'use client'

import {useQueryClient} from '@tanstack/react-query'
import {getMockState, setMockState} from './state'

const STATUS_OPTIONS = [500, 422, 403]

/**
 * Dev-only control to flip the mock between success and simulated-error flows
 * so authenticated surfaces can be reviewed in both states. Rendered only when
 * `NEXT_PUBLIC_MOCK_API=true` (see layout.tsx).
 */
export function MockDevPanel() {
  const qc = useQueryClient()
  const {mode, status} = getMockState()

  const apply = (next: Partial<Parameters<typeof setMockState>[0]>) => {
    setMockState(next)
    // Re-run active queries so list/detail pages reflect the new mode.
    void qc.invalidateQueries()
  }

  return (
    <div
      className="fixed bottom-0 left-0 right-0 z-50 flex items-center gap-2 border-t border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 md:left-3 md:right-auto md:bottom-3 md:rounded-lg md:border">
      <span className="font-semibold uppercase tracking-wide">Mock API</span>
      <span className="rounded bg-amber-200 px-1.5 py-0.5 font-mono">{mode}</span>
      <button
        type="button"
        onClick={() => apply({mode: 'ok'})}
        className="rounded bg-white px-2 py-1 font-medium ring-1 ring-amber-300 hover:bg-amber-100"
      >
        Sucesso
      </button>
      <button
        type="button"
        onClick={() => apply({mode: 'error'})}
        className="rounded bg-white px-2 py-1 font-medium ring-1 ring-amber-300 hover:bg-amber-100"
      >
        Erro
      </button>
      <select
        aria-label="Status do erro simulado"
        value={status}
        onChange={(e) => setMockState({status: Number(e.target.value)})}
        className="rounded bg-white px-1 py-1 ring-1 ring-amber-300"
      >
        {STATUS_OPTIONS.map((s) => (
          <option key={s} value={s}>{s}</option>
        ))}
      </select>
    </div>
  )
}
