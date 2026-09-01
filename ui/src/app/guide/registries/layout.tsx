import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Cadastros que a emissão usa',
  description: 'Pessoas e produtos no bloco global; serviços, perfis fiscais, naturezas de operação e veículos dentro do documento que os usa — cada um preenche um pedaço da emissão.',
  alternates: {canonical: absoluteUrl('/guide/registries')},
}

const LD = guideTopicLd({
  href: '/guide/registries',
  title: 'Cadastros que a emissão usa',
  description: 'Pessoas e produtos no bloco global; serviços, perfis fiscais, naturezas de operação e veículos dentro do documento que os usa — cada um preenche um pedaço da emissão.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
