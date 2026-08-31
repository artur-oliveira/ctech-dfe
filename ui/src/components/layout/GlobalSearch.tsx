'use client'

import {useMemo, useState} from 'react'
import {useRouter} from 'next/navigation'
import Fuse, {type IFuseOptions} from 'fuse.js'
import {Search} from 'lucide-react'
import {useAuth} from '@/lib/hooks/useAuth'
import {SEARCH_ENTRIES, type SearchEntry} from '@/lib/navigation/nav'

const MAX_RESULTS = 8

/** Empate entre rótulo, palavras-chave e contexto — o rótulo sempre ganha. */
const FUSE_OPTIONS: IFuseOptions<SearchEntry> = {
  keys: [
    {name: 'label', weight: 3},
    {name: 'keywords', weight: 2},
    {name: 'context', weight: 1},
  ],
  threshold: 0.4,
  ignoreLocation: true,
}

interface GlobalSearchProps {
  open: boolean
  onClose: () => void
}

/**
 * Busca global (⌘K / Ctrl+K). O índice vem de `lib/navigation/nav`, a mesma
 * configuração que desenha a navegação — página nova na navegação já nasce
 * pesquisável.
 */
export function GlobalSearch({open, onClose}: GlobalSearchProps) {
  // Montar só quando aberta zera consulta e cursor sem efeito de sincronização.
  if (!open) return null
  return <SearchDialog onClose={onClose}/>
}

function SearchDialog({onClose}: {onClose: () => void}) {
  const router = useRouter()
  const {selectedOrg} = useAuth()
  const role = selectedOrg?.role
  const [query, setQuery] = useState('')
  const [cursor, setCursor] = useState(0)

  const entries = useMemo(
    () => SEARCH_ENTRIES.filter(e => !e.roles || (role != null && e.roles.includes(role))),
    [role],
  )
  const fuse = useMemo(() => new Fuse(entries, FUSE_OPTIONS), [entries])

  const results: SearchEntry[] = useMemo(() => {
    const term = query.trim()
    if (!term) return entries.slice(0, MAX_RESULTS)
    return fuse.search(term, {limit: MAX_RESULTS}).map(r => r.item)
  }, [query, entries, fuse])

  const go = (entry: SearchEntry | undefined) => {
    if (!entry) return
    onClose()
    router.push(entry.href)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setCursor(c => (results.length ? (c + 1) % results.length : 0))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setCursor(c => (results.length ? (c - 1 + results.length) % results.length : 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      go(results[cursor])
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 p-4 pt-[10vh]"
      onClick={onClose}
      role="presentation"
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Buscar no aplicativo"
        className="w-full sm:max-w-lg overflow-hidden rounded-xl border border-gray-200 bg-white shadow-modal"
        onClick={e => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        <div className="flex items-center gap-2.5 border-b border-gray-200 px-4">
          <Search size={16} aria-hidden="true" className="shrink-0 text-gray-400"/>
          <input
            autoFocus
            value={query}
            onChange={e => {
              setQuery(e.target.value)
              setCursor(0)
            }}
            placeholder="Buscar páginas e cadastros…"
            aria-label="Buscar páginas e cadastros"
            aria-controls="global-search-results"
            autoComplete="off"
            className="h-12 w-full bg-transparent text-base md:text-sm text-gray-900 placeholder:text-gray-500 outline-hidden"
          />
          <kbd
            className="hidden sm:inline shrink-0 rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-xs font-medium text-gray-600">
            Esc
          </kbd>
        </div>

        <ul id="global-search-results" className="max-h-80 overflow-y-auto p-1.5">
          {results.length === 0 && (
            <li className="px-3 py-6 text-center text-sm text-gray-600">
              Nada encontrado para <b className="text-gray-900">{query}</b>.
            </li>
          )}
          {results.map((entry, i) => (
            <li key={entry.href}>
              <button
                type="button"
                onClick={() => go(entry)}
                onMouseEnter={() => setCursor(i)}
                aria-current={i === cursor ? 'true' : undefined}
                className={[
                  'flex w-full items-center gap-3 rounded-md px-2.5 py-2 min-h-11 sm:min-h-0 text-left transition-colors',
                  i === cursor ? 'bg-brand-50' : 'hover:bg-gray-50',
                ].join(' ')}
              >
                <span className={['shrink-0', i === cursor ? 'text-brand-600' : 'text-gray-400'].join(' ')}>
                  {entry.icon}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-gray-900">{entry.label}</span>
                  <span className="block truncate text-xs text-gray-600">{entry.context}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
