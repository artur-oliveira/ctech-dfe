'use client'

import Link from 'next/link'
import {useEffect} from 'react'
import {SystemState, SystemStateRetry} from '@/components/SystemState'
import {Button} from '@/components/ui/button'

/**
 * The boundary for an uncaught error inside a route.
 *
 * `reset` re-renders the segment without a full reload, which is the right
 * first attempt: most of these are one bad render over good data.
 *
 * The digest is shown on purpose. It is the only handle a person has when they
 * write in, and asking somebody to describe a blank screen is asking them to do
 * the debugging.
 */
export default function ErrorPage({error, reset}: { error: Error & {digest?: string}; reset: () => void }) {
  useEffect(() => {
    console.error(error)
  }, [error])

  return (
    <SystemState
      code="500"
      title="Algo quebrou nesta tela"
      description="Nada foi enviado à SEFAZ por causa disso — o erro aconteceu aqui, no navegador."
      detail={error.digest ? `Referência do erro: ${error.digest}` : 'Se acontecer de novo, avise o suporte.'}
    >
      <SystemStateRetry onRetry={reset} label="Carregar novamente"/>
      <Button variant="outline" render={<Link href="/dashboard"/>}>Ir para o painel</Button>
    </SystemState>
  )
}
