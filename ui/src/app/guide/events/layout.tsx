import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Eventos do documento',
  description: 'Cancelamento, carta de correção, encerramento e manifestação: o que cada evento faz, quando cabe e onde ler a resposta da SEFAZ.',
  alternates: {canonical: absoluteUrl('/guide/events')},
}

const LD = guideTopicLd({
  href: '/guide/events',
  title: 'Eventos do documento',
  description: 'Cancelamento, carta de correção, encerramento e manifestação: o que cada evento faz, quando cabe e onde ler a resposta da SEFAZ.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
