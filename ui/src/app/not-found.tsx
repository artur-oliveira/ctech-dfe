'use client'

import Link from 'next/link'
import {useAuth} from '@/lib/hooks/useAuth'

export default function NotFound() {
  const {user} = useAuth()

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
      <div className="text-center">
        <p className="text-7xl font-bold text-gray-200 select-none">404</p>
        <h1 className="mt-4 text-xl font-semibold text-gray-900">Página não encontrada</h1>
        <p className="mt-2 text-sm text-gray-500">
          O endereço que você acessou não existe ou foi removido.
        </p>
        <Link
          href={user ? '/dashboard' : '/login'}
          className="mt-6 inline-flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors"
          style={{backgroundColor: 'var(--brand-600)'}}
        >
          {user ? 'Ir para o painel' : 'Ir para o login'}
        </Link>
      </div>
    </div>
  )
}
