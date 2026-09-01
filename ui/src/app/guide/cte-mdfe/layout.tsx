import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Transporte — CT-e e MDF-e',
  description: 'Manifesto de carga com veículos, condutores e notas vinculadas; encerramento da viagem; e o que já dá para fazer com CT-e hoje.',
  alternates: {canonical: absoluteUrl('/guide/cte-mdfe')},
}

const LD = guideTopicLd({
  href: '/guide/cte-mdfe',
  title: 'Transporte — CT-e e MDF-e',
  description: 'Manifesto de carga com veículos, condutores e notas vinculadas; encerramento da viagem; e o que já dá para fazer com CT-e hoje.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
