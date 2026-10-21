'use client';

import Link from 'next/link';
import {ErrorState} from '@aoctech/ui';
import {useAuth} from '@/lib/hooks/useAuth';
import {Button} from '@/components/ui/button';

export default function NotFound() {
  const {user} = useAuth();

  return (
    <ErrorState
      status={404}
      locale="pt-BR"
      className="min-h-screen justify-center"
      title="Página não encontrada"
      description="O endereço que você acessou não existe ou foi removido."
      action={
        <Button variant="brand" render={<Link href={user ? '/dashboard' : '/login'}/>}>
          {user ? 'Ir para o painel' : 'Ir para o login'}
        </Button>
      }
    />
  );
}
