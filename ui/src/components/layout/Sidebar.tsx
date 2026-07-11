'use client'

import Link from 'next/link'
import {usePathname} from 'next/navigation'
import type {ReactNode} from 'react'
import {CteIcon, MdfeIcon, NfceIcon, NfeIcon} from "@/components/ui/icon"
import {Button} from '@/components/ui/button'

interface NavItem {
  href: string
  label: string
  icon: ReactNode
  sub?: boolean
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const TruckIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="1" y="3" width="15" height="13"/>
    <polygon points="16 8 20 8 23 11 23 16 16 16 16 8"/>
    <circle cx="5.5" cy="18.5" r="2.5"/>
    <circle cx="18.5" cy="18.5" r="2.5"/>
  </svg>
)

const ShoppingBagIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M6 2L3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z"/>
    <line x1="3" y1="6" x2="21" y2="6"/>
    <path d="M16 10a4 4 0 0 1-8 0"/>
  </svg>
)

const UsersIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
    <circle cx="9" cy="7" r="4"/>
    <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
    <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
  </svg>
)

const SettingsIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="3"/>
    <path
      d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
  </svg>
)

const ShieldIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
  </svg>
)

const ClipboardIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>
    <rect x="8" y="2" width="8" height="4" rx="1"/>
    <line x1="9" y1="12" x2="15" y2="12"/>
    <line x1="9" y1="16" x2="15" y2="16"/>
  </svg>
)

const BuildingIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="9" width="18" height="13"/>
    <path d="M9 22V12h6v10"/>
    <path d="M3 9l9-7 9 7"/>
  </svg>
)

const GridIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="3" width="7" height="7"/>
    <rect x="14" y="3" width="7" height="7"/>
    <rect x="14" y="14" width="7" height="7"/>
    <rect x="3" y="14" width="7" height="7"/>
  </svg>
)

const navGroups: NavGroup[] = [
  {
    label: 'Visão Geral',
    items: [
      {href: '/dashboard', label: 'Painel', icon: <GridIcon/>},
    ],
  },
  {
    label: 'Documentos Fiscais',
    items: [
      {href: '/nfe', label: 'NF-e', icon: <NfeIcon/>},
      {href: '/nfce', label: 'NFC-e', icon: <NfceIcon/>},
      {href: '/cte', label: 'CT-e', icon: <CteIcon/>},
      {href: '/mdfe', label: 'MDF-e', icon: <MdfeIcon/>},
    ],
  },
  {
    label: 'Cadastros',
    items: [
      {href: '/products', label: 'Produtos', icon: <ShoppingBagIcon/>},
      {href: '/vehicles', label: 'Veículos', icon: <TruckIcon/>},
      {href: '/persons', label: 'Pessoas', icon: <UsersIcon/>},
    ],
  },
  {
    label: 'Configurações',
    items: [
      {href: '/organizations', label: 'Organizações', icon: <BuildingIcon/>},
      {href: '/fiscal-config', label: 'Configuração Fiscal', icon: <SettingsIcon/>},
      {href: '/certificates', label: 'Certificados', icon: <ShieldIcon/>},
      {href: '/audit-logs', label: 'Log de Auditoria', icon: <ClipboardIcon/>},
    ],
  },
]

// All hrefs in the nav — used to resolve the most specific active match.
const allHrefs = navGroups.flatMap(g => g.items.map(i => i.href))

function isItemActive(href: string, pathname: string): boolean {
  if (pathname === href) return true
  if (!pathname.startsWith(href + '/')) return false
  // A deeper nav item takes precedence — don't mark the parent active.
  return !allHrefs.some(
    other =>
      other !== href &&
      other.startsWith(href + '/') &&
      (pathname === other || pathname.startsWith(other + '/')),
  )
}

interface SidebarProps {
  open: boolean
  onClose: () => void
}

export function Sidebar({open, onClose}: SidebarProps) {
  const pathname = usePathname()

  return (
    <aside
      className={[
        'fixed left-0 top-0 bottom-0 flex flex-col bg-white border-r border-gray-200 z-20',
        'transition-transform duration-200 ease-in-out',
        'md:translate-x-0',
        open ? 'translate-x-0' : '-translate-x-full',
      ].join(' ')}
      style={{width: 'var(--sidebar-width)'}}
    >
      {/* Logo */}
      <div
        className="flex items-center justify-between px-5 h-(--topbar-height) border-b border-gray-200 shrink-0">
        <div className="flex items-center gap-2.5">
          <div
            className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0"
            style={{backgroundColor: 'var(--brand-600)'}}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2.5"
                 strokeLinecap="round" strokeLinejoin="round">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
              <polyline points="14 2 14 8 20 8"/>
            </svg>
          </div>
          <span className="font-semibold text-gray-900 text-[15px] tracking-tight">CTech DF-e</span>
        </div>
        {/* Close button — only shown on mobile */}
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onClose}
          className="md:hidden text-gray-400 hover:text-gray-600"
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
      <nav className="flex-1 overflow-y-auto py-4 px-3">
        {navGroups.map((group) => (
          <div key={group.label} className="mb-5">
            <p className="px-2 mb-1 text-[11px] font-semibold uppercase tracking-wider text-gray-400">
              {group.label}
            </p>
            <ul className="space-y-0.5">
              {group.items.map((item) => {
                const active = isItemActive(item.href, pathname)
                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      onClick={onClose}
                      className={[
                        'flex items-center gap-2.5 px-2 py-2 rounded-md text-sm transition-colors',
                        item.sub ? 'ml-4' : '',
                        active
                          ? 'bg-brand-50 text-brand-700 font-medium'
                          : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900',
                      ].join(' ')}
                    >
                      <span className={[
                        'shrink-0',
                        active ? 'text-brand-600' : 'text-gray-400',
                        item.sub ? 'opacity-75' : '',
                      ].join(' ')}>
                        {item.icon}
                      </span>
                      <span className={item.sub ? 'text-[13px]' : ''}>
                        {item.label}
                      </span>
                    </Link>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </nav>
    </aside>
  )
}
