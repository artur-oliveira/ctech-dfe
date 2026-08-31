'use client'

import {useRouter, usePathname} from 'next/navigation'
import {useState} from 'react'
import {useKeyboardShortcuts} from '@/lib/hooks/useKeyboardShortcuts'
import {contextForPath, DOC_CONTEXTS} from '@/lib/navigation/nav'

/** New-issuance route for the doc-type implied by the current pathname. The
 *  route -> context map lives in `lib/navigation/nav`; CT-e has no issuance
 *  screen yet, so it falls back to the first type that does. */
function newIssueRoute(pathname: string): string {
  const ctx = contextForPath(pathname)
  return (ctx?.emit ?? DOC_CONTEXTS.find(c => c.emit)!.emit!).href
}

const SHORTCUTS: {keys: string; desc: string}[] = [
  {keys: '⌘K / Ctrl+K', desc: 'Buscar páginas e cadastros'},
  {keys: '/', desc: 'Buscar páginas e cadastros'},
  {keys: 'n', desc: 'Nova emissão (tipo da tela atual)'},
  {keys: '?', desc: 'Mostrar atalhos'},
  {keys: 'Esc', desc: 'Fechar diálogo / painel'},
]

/**
 * Mounts the app-wide keyboard shortcuts once (in RootLayout). Power-user
 * accelerants from the critique plan (Task 8). Respects input focus and
 * modifiers — see useKeyboardShortcuts.
 */
export function KeyboardShortcuts({onOpenSearch}: {onOpenSearch: () => void}) {
  const router = useRouter()
  const pathname = usePathname()
  const [helpOpen, setHelpOpen] = useState(false)

  useKeyboardShortcuts([
    {key: 'k', mod: true, global: true, handler: onOpenSearch},
    {key: '/', handler: onOpenSearch},
    {key: 'n', handler: () => router.push(newIssueRoute(pathname))},
    {key: '?', shift: true, global: true, handler: () => setHelpOpen(v => !v)},
    {key: 'Escape', global: true, handler: () => setHelpOpen(false)},
  ], [router, pathname, helpOpen, onOpenSearch])

  if (!helpOpen) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      onClick={() => setHelpOpen(false)}
      role="presentation"
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Atalhos de teclado"
        className="w-full max-w-sm rounded-xl border border-gray-200 bg-white p-5 shadow-modal"
        onClick={e => e.stopPropagation()}
      >
        <h2 className="text-sm font-semibold text-gray-900">Atalhos de teclado</h2>
        <ul className="mt-3 divide-y divide-gray-100">
          {SHORTCUTS.map(s => (
            <li key={s.keys} className="flex items-center justify-between gap-3 py-2">
              <span className="text-sm text-gray-600">{s.desc}</span>
              <kbd className="shrink-0 rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-xs font-medium text-gray-700">{s.keys}</kbd>
            </li>
          ))}
        </ul>
        <button
          type="button"
          onClick={() => setHelpOpen(false)}
          className="mt-4 w-full rounded-md border border-gray-200 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
        >
          Fechar
        </button>
      </div>
    </div>
  )
}
