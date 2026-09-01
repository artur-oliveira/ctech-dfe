import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Primeiros passos',
  description: 'Do cadastro da empresa ao certificado digital: o que precisa estar pronto antes da primeira emissão, e por que homologação vem antes de produção.',
  alternates: {canonical: absoluteUrl('/guide/getting-started')},
}

const LD = guideTopicLd({
  href: '/guide/getting-started',
  title: 'Primeiros passos',
  description: 'Do cadastro da empresa ao certificado digital: o que precisa estar pronto antes da primeira emissão, e por que homologação vem antes de produção.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
