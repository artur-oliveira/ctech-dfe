'use client'

import {useState} from 'react'
import Link from 'next/link'
import Image from 'next/image'
import {usePathname} from 'next/navigation'
import {ChevronDown} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {useAuth} from '@/lib/hooks/useAuth'
import {
  contextForPath,
  DOC_CONTEXTS,
  isItemActive,
  NAV_GROUPS,
  type DocContext,
  type NavItem,
} from '@/lib/navigation/nav'

const DOC_GROUP_LABEL = 'Documentos Fiscais'

const ITEM_BASE =
  'flex items-center gap-2.5 px-2 py-2 min-h-11 sm:min-h-0 rounded-md text-sm transition-colors'
const ITEM_ACTIVE = 'bg-brand-50 text-brand-700 font-medium'
const ITEM_IDLE = 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'

function NavLink(
  {item, active, sub, onNavigate}:
  {item: NavItem; active: boolean; sub?: boolean; onNavigate: () => void},
) {
  return (
    <Link
      href={item.href}
      onClick={onNavigate}
      aria-current={active ? 'page' : undefined}
      className={[ITEM_BASE, sub ? 'pl-2.5' : '', active ? ITEM_ACTIVE : ITEM_IDLE].join(' ')}
    >
      {/* Aninhado, o trilho à esquerda já dá a estrutura — o ícone só roubaria
          a largura de que os rótulos longos precisam. */}
      {!sub && (
        <span className={['shrink-0', active ? 'text-brand-600' : 'text-gray-400'].join(' ')}>
          {item.icon}
        </span>
      )}
      <span className="truncate">{item.label}</span>
    </Link>
  )
}

/**
 * Um tipo de documento e, aninhados, a emissão e os cadastros que só existem
 * por causa dele. O contexto da rota atual abre sozinho; os outros abrem no
 * chevron, sem navegar.
 */
function ContextItem(
  {ctx, pathname, onNavigate}:
  {ctx: DocContext; pathname: string; onNavigate: () => void},
) {
  const inContext = contextForPath(pathname)?.key === ctx.key
  const [expanded, setExpanded] = useState(false)
  const open = inContext || expanded
  const children: NavItem[] = [...(ctx.emit ? [ctx.emit] : []), ...ctx.items]
  const active = isItemActive(ctx.href, pathname)
  const panelId = `nav-context-${ctx.key}`

  return (
    <li>
      <div className={[
        'flex items-center gap-1 rounded-md',
        active ? ITEM_ACTIVE : 'text-gray-600 hover:bg-gray-50',
      ].join(' ')}>
        <Link
          href={ctx.href}
          onClick={onNavigate}
          aria-current={active ? 'page' : undefined}
          className={[ITEM_BASE, 'flex-1 min-w-0 hover:text-gray-900', active ? 'text-brand-700 font-medium' : ''].join(' ')}
        >
          <span className={['shrink-0', active ? 'text-brand-600' : 'text-gray-400'].join(' ')}>
            {ctx.icon}
          </span>
          <span className="truncate">{ctx.label}</span>
        </Link>
        {children.length > 0 && (
          <button
            type="button"
            onClick={() => setExpanded(v => !v)}
            aria-expanded={open}
            aria-controls={panelId}
            aria-label={`${open ? 'Recolher' : 'Expandir'} ${ctx.label}`}
            className="shrink-0 flex items-center justify-center size-11 sm:size-7 rounded-md text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
          >
            <ChevronDown
              size={14}
              aria-hidden="true"
              className={['transition-transform duration-150 motion-reduce:transition-none', open ? 'rotate-180' : ''].join(' ')}
            />
          </button>
        )}
      </div>
      {open && children.length > 0 && (
        <ul id={panelId} className="mt-0.5 ml-2 space-y-0.5 border-l border-gray-200 pl-1">
          {children.map(item => (
            <li key={item.href}>
              <NavLink item={item} active={isItemActive(item.href, pathname)} sub onNavigate={onNavigate}/>
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

interface SidebarProps {
  open: boolean
  onClose: () => void
}

export function Sidebar({open, onClose}: SidebarProps) {
  const pathname = usePathname()
  const {selectedOrg} = useAuth()
  const role = selectedOrg?.role

  return (
    <aside
      className={[
        // Acima da navegação inferior (z-30): com o drawer aberto, uma navegação por vez.
        'fixed left-0 top-0 bottom-0 flex flex-col bg-white border-r border-gray-200 z-40',
        'transition-transform duration-200 ease-in-out motion-reduce:transition-none',
        'md:translate-x-0',
        open ? 'translate-x-0' : '-translate-x-full',
      ].join(' ')}
      style={{width: 'var(--sidebar-width)'}}
    >
      {/* Logo */}
      <div
        className="flex items-center justify-between px-5 h-(--topbar-height) border-b border-gray-200 shrink-0">
        <div className="flex items-center gap-2.5">
          <Image src="/app.svg" alt="" aria-hidden="true" width={28} height={28} className="shrink-0" unoptimized/>
          <span className="font-semibold text-gray-900 text-base tracking-tight">CTech DF-e</span>
        </div>
        {/* Close button — only shown on mobile */}
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onClose}
          className="md:hidden min-h-11 min-w-11 text-gray-400 hover:text-gray-600"
          aria-label="Fechar menu"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
               strokeLinecap="round" strokeLinejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"/>
            <line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        </Button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4 px-3 pb-24 md:pb-4" aria-label="Navegação principal">
        {NAV_GROUPS.map((group) => {
          const items = group.items.filter((item) => !item.roles || (role != null && item.roles.includes(role)))
          if (items.length === 0) return null
          const isDocGroup = group.label === DOC_GROUP_LABEL
          return (
            <div key={group.label} className="mb-5">
              <p className="px-2 mb-1 text-xs font-semibold uppercase tracking-wider text-gray-400">
                {group.label}
              </p>
              <ul className="space-y-0.5">
                {isDocGroup
                  ? DOC_CONTEXTS.map(ctx => (
                    <ContextItem key={ctx.key} ctx={ctx} pathname={pathname} onNavigate={onClose}/>
                  ))
                  : items.map(item => (
                    <li key={item.href}>
                      <NavLink item={item} active={isItemActive(item.href, pathname)} onNavigate={onClose}/>
                    </li>
                  ))}
              </ul>
            </div>
          )
        })}
      </nav>
    </aside>
  )
}
