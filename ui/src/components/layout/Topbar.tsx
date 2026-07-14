'use client'

import {useEffect, useRef, useState} from 'react'
import {useRouter} from 'next/navigation'
import {useAuth} from '@/lib/hooks/useAuth'
import {Button} from '@/components/ui/button'
import {endSessionRedirect} from '@/lib/auth/oauth'

const ChevronDownIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <polyline points="6 9 12 15 18 9"/>
  </svg>
)

const UserIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
    <circle cx="12" cy="7" r="4"/>
  </svg>
)

const SettingsIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="3"/>
    <path
      d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
  </svg>
)

const EditUserIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 24 24">
    <path fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
          d="M8 7a4 4 0 1 0 8 0a4 4 0 0 0-8 0M6 21v-2a4 4 0 0 1 4-4h3.5m4.92.61a2.1 2.1 0 0 1 2.97 2.97L18 22h-3v-3z"></path>
  </svg>
)

const LogOutIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
    <polyline points="16 17 21 12 16 7"/>
    <line x1="21" y1="12" x2="9" y2="12"/>
  </svg>
)

const MenuIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
       strokeLinecap="round" strokeLinejoin="round">
    <line x1="3" y1="6" x2="21" y2="6"/>
    <line x1="3" y1="12" x2="21" y2="12"/>
    <line x1="3" y1="18" x2="21" y2="18"/>
  </svg>
)

interface TopbarProps {
  onMenuClick: () => void
}

export function Topbar({onMenuClick}: TopbarProps) {
  const {user, selectedOrg, setSelectedOrg, logout} = useAuth()
  const router = useRouter()
  const [orgDropdownOpen, setOrgDropdownOpen] = useState(false)
  const [userDropdownOpen, setUserDropdownOpen] = useState(false)
  const orgRef = useRef<HTMLDivElement>(null)
  const userRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (orgRef.current && !orgRef.current.contains(e.target as Node)) {
        setOrgDropdownOpen(false)
      }
      if (userRef.current && !userRef.current.contains(e.target as Node)) {
        setUserDropdownOpen(false)
      }
    }

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        setOrgDropdownOpen(false)
        setUserDropdownOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [])

  const handleLogout = () => {
    logout()
    endSessionRedirect('/login')
  }

  const initials = user
    ? `${user.first_name[0] ?? ''}${user.last_name[0] ?? ''}`.toUpperCase()
    : ''

  return (
    <header
      className="fixed top-0 right-0 left-0 md:left-(--sidebar-width) flex items-center justify-between bg-white border-b border-gray-200 px-4 md:px-6 z-10"
      style={{height: 'var(--topbar-height)'}}
    >
      {/* Hamburger — mobile only */}
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={onMenuClick}
        className="md:hidden mr-3 shrink-0 text-gray-500 hover:text-gray-700"
        aria-label="Abrir menu"
      >
        <MenuIcon/>
      </Button>

      {/* Org selector */}
      <div className="relative flex-1 min-w-0" ref={orgRef}>
        {user?.organizations && user.organizations.length > 0 ? (
          <button
            onClick={() => setOrgDropdownOpen(!orgDropdownOpen)}
            aria-haspopup="menu"
            aria-expanded={orgDropdownOpen}
            className="flex items-center gap-2 px-3 py-1.5 rounded-md border border-gray-200 text-sm text-gray-700 hover:border-gray-300 hover:bg-gray-50 transition-colors max-w-full"
          >
            <span className="font-medium truncate max-w-35 sm:max-w-50">
              {selectedOrg?.description ?? selectedOrg?.name ?? 'Selecionar organização'}
            </span>
            <ChevronDownIcon/>
          </button>
        ) : (
          <span className="text-sm text-gray-400">Nenhuma organização</span>
        )}

        {orgDropdownOpen && user?.organizations && (
          <div
            className="absolute left-0 top-full mt-1 w-64 bg-white border border-gray-200 rounded-lg shadow-modal z-50 py-1">
            {user.organizations.map((org) => (
              <button
                key={org.pk}
                onClick={() => {
                  setSelectedOrg(org)
                  setOrgDropdownOpen(false)
                }}
                className={`w-full text-left px-4 py-2.5 text-sm hover:bg-gray-50 transition-colors ${
                  selectedOrg?.pk === org.pk ? 'text-brand-700 font-medium' : 'text-gray-700'
                }`}
              >
                <div className="font-medium">{org.name}</div>
                <div className="text-xs text-gray-400 mt-0.5">{org.role}</div>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* User menu */}
      <div className="relative shrink-0" ref={userRef}>
        <button
          onClick={() => setUserDropdownOpen(!userDropdownOpen)}
          aria-haspopup="menu"
          aria-expanded={userDropdownOpen}
          className="flex items-center gap-2 pl-2 pr-3 py-1.5 rounded-md hover:bg-gray-50 transition-colors"
        >
          <div
            className="w-7 h-7 rounded-full flex items-center justify-center text-white text-xs font-semibold shrink-0"
            style={{backgroundColor: 'var(--brand-600)'}}
          >
            {initials || <UserIcon/>}
          </div>
          <span className="hidden sm:inline text-sm text-gray-700 font-medium">
            {user?.first_name} {user?.last_name}
          </span>
          <ChevronDownIcon/>
        </button>

        {userDropdownOpen && (
          <div
            className="absolute right-0 top-full mt-1 w-52 bg-white border border-gray-200 rounded-lg shadow-modal z-50 py-1">
            <div className="px-4 py-2.5 border-b border-gray-100">
              <p className="text-sm font-medium text-gray-900">
                {user?.first_name} {user?.last_name}
              </p>
              <p className="text-xs text-gray-500 mt-0.5">{user?.email}</p>
            </div>
            <button
              onClick={() => {
                router.push('/profile')
                setUserDropdownOpen(false)
              }}
              className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
            >
              <EditUserIcon/>
              Meu perfil
            </button>
            <button
              onClick={() => {
                router.push('/organizations')
                setUserDropdownOpen(false)
              }}
              className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-gray-700 hover:bg-gray-50 transition-colors"
            >
              <SettingsIcon/>
              Configurar organização
            </button>
            <div className="border-t border-gray-100 mt-1 pt-1">
              <button
                onClick={handleLogout}
                className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-red-600 hover:bg-red-50 transition-colors"
              >
                <LogOutIcon/>
                Sair
              </button>
            </div>
          </div>
        )}
      </div>
    </header>
  )
}
