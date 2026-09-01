import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Emitir NFC-e',
  description: 'A nota do balcão: venda a consumidor final, CSC por ambiente, cancelamento dentro do prazo e substituição de nota.',
  alternates: {canonical: absoluteUrl('/guide/nfce')},
}

const LD = guideTopicLd({
  href: '/guide/nfce',
  title: 'Emitir NFC-e',
  description: 'A nota do balcão: venda a consumidor final, CSC por ambiente, cancelamento dentro do prazo e substituição de nota.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
