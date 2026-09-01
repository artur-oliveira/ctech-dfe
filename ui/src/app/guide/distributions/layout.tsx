import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Documentos recebidos',
  description: 'A SEFAZ entrega por NSU tudo que foi emitido contra o seu CNPJ. Como consultar, importar por chave ou XML e manifestar-se.',
  alternates: {canonical: absoluteUrl('/guide/distributions')},
}

const LD = guideTopicLd({
  href: '/guide/distributions',
  title: 'Documentos recebidos',
  description: 'A SEFAZ entrega por NSU tudo que foi emitido contra o seu CNPJ. Como consultar, importar por chave ou XML e manifestar-se.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
