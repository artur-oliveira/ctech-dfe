'use client'

import {useRouter} from 'next/navigation'
import {Menu} from '@base-ui/react/menu'
import {Search} from 'lucide-react'
import {useAuth} from '@/lib/hooks/useAuth'
import {SettingsIcon} from '@/components/ui/icon'
import {ROLE_LABEL, RoleName} from "@/lib/data/roles";

const MENU_POPUP_CLASSNAME = 'rounded-lg border border-gray-200 bg-white shadow-popover py-1 ' +
    'origin-(--transform-origin) duration-100 data-[side=bottom]:slide-in-from-top-2 ' +
    'data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 ' +
    'data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95'

const MENU_ITEM_CLASSNAME = 'w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-gray-700 ' +
    'cursor-default outline-hidden transition-colors data-highlighted:bg-gray-50'

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

interface TopbarProps {
    onSearchClick: () => void
}

export function Topbar({onSearchClick}: TopbarProps) {
    const {user, selectedOrg, setSelectedOrg, logout} = useAuth()
    const router = useRouter()

    const handleLogout = () => {
        void logout('/')
    }

    const initials = user
        ? `${user.first_name[0] ?? ''}${user.last_name[0] ?? ''}`.toUpperCase()
        : ''

    return (
        <header
            className="fixed top-0 right-0 left-0 md:left-(--sidebar-width) flex items-center justify-between bg-white border-b border-gray-200 px-4 md:px-6 z-10"
            style={{height: 'var(--topbar-height)'}}
        >
            {/* Org selector */}
            <div className="relative flex-1 min-w-0">
                {user?.organizations && user.organizations.length > 0 ? (
                    <Menu.Root>
                        <Menu.Trigger
                            className="flex items-center gap-2 px-3 py-1.5 min-h-11 sm:min-h-0 rounded-md border border-gray-200 text-sm text-gray-700 hover:border-gray-300 hover:bg-gray-50 transition-colors max-w-full"
                        >
              <span className="font-medium truncate max-w-35 sm:max-w-50">
                {selectedOrg?.description ?? selectedOrg?.name ?? 'Selecionar organização'}
              </span>
                            <ChevronDownIcon/>
                        </Menu.Trigger>
                        <Menu.Portal>
                            <Menu.Positioner side="bottom" align="start" sideOffset={4} className="isolate z-50">
                                <Menu.Popup className={`w-64 ${MENU_POPUP_CLASSNAME}`}>
                                    {user.organizations.map((org) => (
                                        <Menu.Item
                                            key={org.pk}
                                            onClick={() => setSelectedOrg(org)}
                                            className={`${MENU_ITEM_CLASSNAME} text-left ${
                                                selectedOrg?.pk === org.pk ? 'text-brand-700 font-medium' : ''
                                            }`}
                                        >
                                            <div className="flex-1">
                                                <div className="font-medium">{org.name}</div>
                                                <div
                                                    className="text-xs text-gray-400 mt-0.5">{ROLE_LABEL[org.role as RoleName] || org.role}</div>
                                            </div>
                                        </Menu.Item>
                                    ))}
                                </Menu.Popup>
                            </Menu.Positioner>
                        </Menu.Portal>
                    </Menu.Root>
                ) : (
                    <span className="text-sm text-gray-400">Nenhuma organização</span>
                )}
            </div>

            {/* Busca global. No mobile ela mora na navegação inferior. */}
            <button
                type="button"
                onClick={onSearchClick}
                aria-label="Buscar páginas e cadastros"
                aria-keyshortcuts="Meta+K Control+K"
                className="hidden md:flex items-center gap-2 mx-4 w-56 lg:w-72 shrink-0 rounded-md border border-gray-200 px-3 py-1.5 text-sm text-gray-600 transition-colors hover:border-gray-300 hover:bg-gray-50"
            >
                <Search size={16} aria-hidden="true" className="shrink-0 text-gray-400"/>
                <span className="flex-1 text-left">Buscar…</span>
                <kbd className="shrink-0 rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-xs font-medium text-gray-600">
                    ⌘K
                </kbd>
            </button>
            {/* User menu */}
            <div className="relative shrink-0">
                <Menu.Root>
                    <Menu.Trigger
                        className="flex items-center gap-2 pl-2 pr-3 py-1.5 min-h-11 sm:min-h-0 rounded-md hover:bg-gray-50 transition-colors"
                    >
                        <div
                            className="w-7 h-7 rounded-full flex items-center justify-center bg-brand-600 text-white text-xs font-semibold shrink-0">
                            {initials || <UserIcon/>}
                        </div>
                        <span className="hidden sm:inline text-sm text-gray-700 font-medium">
              {user?.first_name} {user?.last_name}
            </span>
                        <ChevronDownIcon/>
                    </Menu.Trigger>
                    <Menu.Portal>
                        <Menu.Positioner side="bottom" align="end" sideOffset={4} className="isolate z-50">
                            <Menu.Popup className={`w-52 ${MENU_POPUP_CLASSNAME}`}>
                                <div className="px-4 py-2.5 border-b border-gray-100">
                                    <p className="text-sm font-medium text-gray-900">
                                        {user?.first_name} {user?.last_name}
                                    </p>
                                    <p className="text-xs text-gray-500 mt-0.5">{user?.email}</p>
                                </div>
                                <Menu.Item onClick={() => router.push('/profile')} className={MENU_ITEM_CLASSNAME}>
                                    <EditUserIcon/>
                                    Meu perfil
                                </Menu.Item>
                                <Menu.Item onClick={() => router.push('/organizations')}
                                           className={MENU_ITEM_CLASSNAME}>
                                    <SettingsIcon width={14} height={14}/>
                                    Configurar organização
                                </Menu.Item>
                                <div className="border-t border-gray-100 mt-1 pt-1">
                                    <Menu.Item
                                        onClick={handleLogout}
                                        className={`${MENU_ITEM_CLASSNAME} text-red-600 data-highlighted:bg-red-50`}
                                    >
                                        <LogOutIcon/>
                                        Sair
                                    </Menu.Item>
                                </div>
                            </Menu.Popup>
                        </Menu.Positioner>
                    </Menu.Portal>
                </Menu.Root>
            </div>
        </header>
    )
}
