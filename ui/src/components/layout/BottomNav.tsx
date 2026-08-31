'use client'

import {useState} from 'react'
import Link from 'next/link'
import {usePathname} from 'next/navigation'
import {FileText, LayoutGrid, Menu, Plus, Search} from 'lucide-react'
import {contextForPath, DOC_CONTEXTS, isItemActive} from '@/lib/navigation/nav'

const TAB_BASE =
  'flex flex-1 flex-col items-center justify-center gap-0.5 min-h-14 px-0.5 text-xs font-medium transition-colors'
const TAB_ACTIVE = 'text-brand-700'
const TAB_IDLE = 'text-gray-600'

interface BottomNavProps {
  onOpenMenu: () => void
  onOpenSearch: () => void
}

/**
 * Navegação primária do mobile. Substitui o alcance do polegar pelo que a barra
 * lateral resolve no desktop: painel, troca de documento, emissão e busca.
 * A barra lateral segue disponível em "Menu" para tudo o que é secundário.
 */
export function BottomNav({onOpenMenu, onOpenSearch}: BottomNavProps) {
  const pathname = usePathname()
  const [sheetOpen, setSheetOpen] = useState(false)
  const activeContext = contextForPath(pathname)
  // CT-e ainda não emite: nesse contexto a ação cai no primeiro tipo que emite.
  const emitContext = activeContext?.emit ? activeContext : DOC_CONTEXTS.find(c => c.emit)!
  const emitHref = emitContext.emit!.href

  return (
    <>
      {sheetOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/40 md:hidden"
          onClick={() => setSheetOpen(false)}
          role="presentation"
        />
      )}

      <nav
        aria-label="Navegação principal (mobile)"
        className="fixed inset-x-0 bottom-0 z-30 border-t border-gray-200 bg-white md:hidden"
      >
        {sheetOpen && (
          <div
            id="bottomnav-docs"
            className="absolute inset-x-0 bottom-full max-h-[60vh] overflow-y-auto border-t border-gray-200 bg-white p-2 shadow-popover"
          >
            <p className="px-2 pb-1 pt-1 text-xs font-semibold uppercase tracking-wider text-gray-500">
              Documentos fiscais
            </p>
            <ul className="grid grid-cols-1 gap-0.5">
              {DOC_CONTEXTS.map(ctx => {
                const active = activeContext?.key === ctx.key
                return (
                  <li key={ctx.key}>
                    <Link
                      href={ctx.href}
                      onClick={() => setSheetOpen(false)}
                      aria-current={isItemActive(ctx.href, pathname) ? 'page' : undefined}
                      className={[
                        'flex items-center gap-2.5 rounded-md px-2 py-2 min-h-11 text-sm transition-colors',
                        active ? 'bg-brand-50 text-brand-700 font-medium' : 'text-gray-700 hover:bg-gray-50',
                      ].join(' ')}
                    >
                      <span className={active ? 'text-brand-600' : 'text-gray-400'}>{ctx.icon}</span>
                      {ctx.label}
                    </Link>
                  </li>
                )
              })}
            </ul>

            {activeContext && activeContext.items.length > 0 && (
              <>
                <p className="px-2 pb-1 pt-3 text-xs font-semibold uppercase tracking-wider text-gray-500">
                  Cadastros de {activeContext.label}
                </p>
                <ul className="grid grid-cols-1 gap-0.5">
                  {activeContext.items.map(item => (
                    <li key={item.href}>
                      <Link
                        href={item.href}
                        onClick={() => setSheetOpen(false)}
                        aria-current={isItemActive(item.href, pathname) ? 'page' : undefined}
                        className={[
                          'flex items-center gap-2.5 rounded-md px-2 py-2 min-h-11 text-sm transition-colors',
                          isItemActive(item.href, pathname)
                            ? 'bg-brand-50 text-brand-700 font-medium'
                            : 'text-gray-700 hover:bg-gray-50',
                        ].join(' ')}
                      >
                        <span className="text-gray-400">{item.icon}</span>
                        {item.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
        )}

        <ul className="flex items-stretch">
          <li className="flex flex-1">
            <Link
              href="/dashboard"
              onClick={() => setSheetOpen(false)}
              aria-current={isItemActive('/dashboard', pathname) ? 'page' : undefined}
              className={[TAB_BASE, isItemActive('/dashboard', pathname) ? TAB_ACTIVE : TAB_IDLE].join(' ')}
            >
              <LayoutGrid size={20} aria-hidden="true"/>
              Painel
            </Link>
          </li>

          <li className="flex flex-1">
            <button
              type="button"
              onClick={() => setSheetOpen(v => !v)}
              aria-expanded={sheetOpen}
              aria-controls="bottomnav-docs"
              className={[TAB_BASE, activeContext || sheetOpen ? TAB_ACTIVE : TAB_IDLE].join(' ')}
            >
              <FileText size={20} aria-hidden="true"/>
              <span className="w-full truncate text-center">{activeContext?.label ?? 'Documentos'}</span>
            </button>
          </li>

          <li className="flex flex-1">
            <Link
              href={emitHref}
              onClick={() => setSheetOpen(false)}
              aria-label={`Emitir ${emitContext.label}`}
              className={[TAB_BASE, isItemActive(emitHref, pathname) ? TAB_ACTIVE : TAB_IDLE].join(' ')}
            >
              <span
                className="flex size-7 items-center justify-center rounded-full bg-brand-600 text-white">
                <Plus size={16} aria-hidden="true"/>
              </span>
              Emitir
            </Link>
          </li>

          <li className="flex flex-1">
            <button
              type="button"
              onClick={() => {
                setSheetOpen(false)
                onOpenSearch()
              }}
              className={[TAB_BASE, TAB_IDLE].join(' ')}
            >
              <Search size={20} aria-hidden="true"/>
              Buscar
            </button>
          </li>

          <li className="flex flex-1">
            <button
              type="button"
              onClick={() => {
                setSheetOpen(false)
                onOpenMenu()
              }}
              aria-label="Abrir menu completo"
              className={[TAB_BASE, TAB_IDLE].join(' ')}
            >
              <Menu size={20} aria-hidden="true"/>
              Menu
            </button>
          </li>
        </ul>
      </nav>
    </>
  )
}
