import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Como circular pelo sistema',
  description: 'Barra lateral por contexto, busca global em ⌘K, navegação inferior no celular e os atalhos de teclado que cortam cliques.',
  alternates: {canonical: absoluteUrl('/guide/navigation')},
}

const LD = guideTopicLd({
  href: '/guide/navigation',
  title: 'Como circular pelo sistema',
  description: 'Barra lateral por contexto, busca global em ⌘K, navegação inferior no celular e os atalhos de teclado que cortam cliques.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
