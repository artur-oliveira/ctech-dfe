import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Emitir NF-e',
  description: 'Os quatro passos da emissão — destinatário, produtos, pagamento e revisão —, o rascunho automático e o que fazer com a nota depois de autorizada.',
  alternates: {canonical: absoluteUrl('/guide/nfe')},
}

const LD = guideTopicLd({
  href: '/guide/nfe',
  title: 'Emitir NF-e',
  description: 'Os quatro passos da emissão — destinatário, produtos, pagamento e revisão —, o rascunho automático e o que fazer com a nota depois de autorizada.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
