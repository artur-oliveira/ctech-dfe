'use client'

import Link from 'next/link'
import {useAuth} from '@/lib/hooks/useAuth'
import {Button} from '@/components/ui/button'
import {SystemState} from '@/components/SystemState'

export default function NotFound() {
  const {user} = useAuth()

  return (
    <SystemState
      code="404"
      title="Página não encontrada"
      description="O endereço que você acessou não existe ou foi removido."
    >
      <Button variant="brand" render={<Link href={user ? '/dashboard' : '/login'}/>}>
        {user ? 'Ir para o painel' : 'Ir para o login'}
      </Button>
    </SystemState>
  )
}
