'use client'

import Link from 'next/link'
import Image from 'next/image'
import {usePathname} from 'next/navigation'
import {BookOpen} from 'lucide-react'
import type {ReactNode} from 'react'
import {
  BriefcaseIcon,
  CalendarClockIcon,
  CreditCardIcon,
  CteIcon,
  MdfeIcon,
  NfceIcon,
  NfeIcon,
  NfseIcon,
  PercentIcon,
  ImportIcon,
  PackageIcon,
  RouteIcon,
  SettingsIcon,
  ShieldIcon,
  ShoppingBagIcon,
  TruckIcon,
  UsersIcon,
  VehicleSetIcon,
} from "@/components/ui/icon"
import {Button} from '@/components/ui/button'
import {useAuth} from '@/lib/hooks/useAuth'
import {ROLE_ADMIN, ROLE_OWNER} from '@/lib/data/roles'
import {SUBSCRIPTION_PATH} from '@/lib/billing/notice'

interface NavItem {
  href: string
  label: string
  icon: ReactNode
  sub?: boolean
  /** When set, the item is shown only to members with one of these roles. */
  roles?: string[]
}

interface NavGroup {
  label: string
  items: NavItem[]
}

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

const CardIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="5" width="20" height="14" rx="2"/>
    <line x1="2" y1="10" x2="22" y2="10"/>
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
      {href: '/guide', label: 'Guia', icon: <BookOpen size={16}/>},
    ],
  },
  {
    label: 'Documentos Fiscais',
    items: [
      {href: '/nfe', label: 'NF-e', icon: <NfeIcon/>},
      {href: '/nfce', label: 'NFC-e', icon: <NfceIcon/>},
      {href: '/cte', label: 'CT-e', icon: <CteIcon/>},
      {href: '/mdfe', label: 'MDF-e', icon: <MdfeIcon/>},
      {href: '/nfse', label: 'NFS-e', icon: <NfseIcon/>},
    ],
  },
  {
    label: 'Cadastros',
    items: [
      {href: '/persons', label: 'Pessoas', icon: <UsersIcon/>},
      {href: '/products', label: 'Produtos', icon: <ShoppingBagIcon/>},
      {href: '/services', label: 'Serviços', icon: <BriefcaseIcon/>},
      {href: '/tax-profiles', label: 'Perfis fiscais', icon: <PercentIcon/>},
      {href: '/operations', label: 'Naturezas de operação', icon: <RouteIcon/>},
      {href: '/payment-terms', label: 'Condições de pagamento', icon: <CalendarClockIcon/>},
      {href: '/payment-terminals', label: 'Terminais de pagamento', icon: <CreditCardIcon/>},
      {href: '/vehicles', label: 'Veículos', icon: <TruckIcon/>},
      {href: '/vehicle-sets', label: 'Composições veiculares', icon: <VehicleSetIcon/>},
      {href: '/toll-providers', label: 'Vale-pedágio', icon: <RouteIcon/>},
      {href: '/cargo-units', label: 'Unidades de carga', icon: <PackageIcon/>},
      {href: '/import-declarations', label: 'Declarações de importação', icon: <ImportIcon/>},
    ],
  },
  {
    label: 'Configurações',
    items: [
      {href: '/organizations', label: 'Organizações', icon: <BuildingIcon/>},
      {href: '/members', label: 'Usuários', icon: <UsersIcon/>, roles: [ROLE_OWNER, ROLE_ADMIN]},
      // USER and VIEWER never see it: they cannot act on the plan and cannot read
      // it either — `GET /organizations/{pk}/plan` is OWNER/ADMIN only.
      {href: SUBSCRIPTION_PATH, label: 'Assinatura', icon: <CardIcon/>, roles: [ROLE_OWNER, ROLE_ADMIN]},
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
  const {selectedOrg} = useAuth()
  const role = selectedOrg?.role

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
      <nav className="flex-1 overflow-y-auto py-4 px-3">
        {navGroups.map((group) => {
          const items = group.items.filter((item) => !item.roles || (role != null && item.roles.includes(role)))
          if (items.length === 0) return null
          return (
            <div key={group.label} className="mb-5">
              <p className="px-2 mb-1 text-xs font-semibold uppercase tracking-wider text-gray-400">
                {group.label}
              </p>
              <ul className="space-y-0.5">
                {items.map((item) => {
                  const active = isItemActive(item.href, pathname)
                  return (
                    <li key={item.href}>
                      <Link
                        href={item.href}
                        onClick={onClose}
                        className={[
                          'flex items-center gap-2.5 px-2 py-2 min-h-11 sm:min-h-0 rounded-md text-sm transition-colors',
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
                        <span className={item.sub ? 'text-sm' : ''}>
                        {item.label}
                      </span>
                      </Link>
                    </li>
                  )
                })}
              </ul>
            </div>
          )
        })}
      </nav>
    </aside>
  )
}
