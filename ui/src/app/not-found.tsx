'use client'

import Link from 'next/link'
import {useAuth} from '@/lib/hooks/useAuth'
import {Button} from '@/components/ui/button'

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
        <Button variant="brand" className="mt-6" render={<Link href={user ? '/dashboard' : '/login'}/>}>
          {user ? 'Ir para o painel' : 'Ir para o login'}
        </Button>
      </div>
    </div>
  )
}
