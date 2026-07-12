'use client'

import Link from 'next/link'

export function Header() {
  return (
    <header className="bg-primary-600 text-white shadow-lg">
      <div className="max-w-7xl mx-auto px-4 py-4 flex items-center justify-between">
        <Link href="/" className="flex items-center space-x-2">
          <div className="w-8 h-8 bg-white rounded-lg flex items-center justify-center">
            <span className="text-primary-600 font-bold">📄</span>
          </div>
          <span className="text-xl font-bold hidden sm:inline">CTech DF-e</span>
        </Link>

        <nav className="flex items-center space-x-4">
          <Link href="/organizations" className="hover:bg-primary-700 px-3 py-2 rounded">
            Organizações
          </Link>
          <button className="bg-white text-primary-600 px-4 py-2 rounded font-medium hover:bg-primary-50">
            Sair
          </button>
        </nav>
      </div>
    </header>
  )
}
