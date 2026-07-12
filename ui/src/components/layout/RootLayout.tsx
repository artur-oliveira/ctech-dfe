'use client'

import React, {useState} from 'react'
import {usePathname} from 'next/navigation'
import {Sidebar} from './Sidebar'
import {Topbar} from './Topbar'
import {getDfeThemeFromPath} from '@/lib/theme/dfe-theme'

export function RootLayout({children}: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const pathname = usePathname()
  const dfeTheme = getDfeThemeFromPath(pathname)

  return (
    <div className="min-h-screen bg-gray-50" data-dfe-theme={dfeTheme}>
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)}/>

      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/40 z-10 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <Topbar onMenuClick={() => setSidebarOpen(true)}/>

      <main
        className="pt-(--topbar-height) md:ml-(--sidebar-width)"
      >
        {children}
      </main>
    </div>
  )
}
