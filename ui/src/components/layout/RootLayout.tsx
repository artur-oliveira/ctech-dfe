'use client'

import React, {useState} from 'react'
import {usePathname} from 'next/navigation'
import {Sidebar} from './Sidebar'
import {Topbar} from './Topbar'
import {BottomNav} from './BottomNav'
import {GlobalSearch} from './GlobalSearch'
import {KeyboardShortcuts} from './KeyboardShortcuts'
import {getDfeThemeFromPath} from '@/lib/theme/dfe-theme'
import {SubscriptionBanner} from '@/components/billing/SubscriptionNotice'

export function RootLayout({children}: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const pathname = usePathname()
  const dfeTheme = getDfeThemeFromPath(pathname)

  return (
    <div className="min-h-screen bg-gray-50" data-dfe-theme={dfeTheme}>
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)}/>

      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/40 z-30 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <Topbar onSearchClick={() => setSearchOpen(true)}/>

      <KeyboardShortcuts onOpenSearch={() => setSearchOpen(true)}/>
      <GlobalSearch open={searchOpen} onClose={() => setSearchOpen(false)}/>

      <main
        className="pt-(--topbar-height) pb-(--bottomnav-height) md:ml-(--sidebar-width)"
      >
        {/* Renders nothing while the account is in good standing. */}
        <SubscriptionBanner/>
        {children}
      </main>

      <BottomNav onOpenMenu={() => setSidebarOpen(true)} onOpenSearch={() => setSearchOpen(true)}/>
    </div>
  )
}
